package videohash

import (
	"errors"
	"fmt"
	"image"
	"log/slog"
	"os"
	"path"

	"github.com/NinaLeven/MemePolice/audiohash"
	"github.com/NinaLeven/MemePolice/ffmpeg"
	"github.com/NinaLeven/MemePolice/fsutils"

	"github.com/corona10/goimagehash"
)

func PerceptualHash(videoPath string) (video uint64, audio uint64, err error) {
	tempDir, err := fsutils.GetTempDir()
	if err != nil {
		return 0, 0, err
	}
	defer fsutils.CleanupTempDir(tempDir)

	vh, err := perceptualVideoHash(tempDir, videoPath)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to calculate video phash: %w", err)
	}

	ah, err := perceptualAudioHash(tempDir, videoPath)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to calculate audio phash: %w", err)
	}

	return vh, ah, err
}

func perceptualAudioHash(tempDir, videoPath string) (uint64, error) {
	audioPath := path.Join(tempDir, path.Base(videoPath)+".mp3")

	err := ffmpeg.ExtractAudio(videoPath, audioPath)
	if err != nil && !errors.Is(err, &ffmpeg.ErrNoAudio{}) {
		return 0, fmt.Errorf("unable to extract audio: %w", err)
	}
	if err != nil && errors.Is(err, &ffmpeg.ErrNoAudio{}) {
		slog.Warn("no audio", slog.String("err", err.Error()))
		return 0, nil
	}

	h, err := audiohash.PerceptualHash(audioPath)
	if err != nil {
		return 0, fmt.Errorf("unable to calculate audio phash: %w", err)
	}

	return h, nil
}

const expectedFramesCount = 12

func perceptualVideoHash(tempDir, videoPath string) (uint64, error) {
	framesDir := path.Join(tempDir, "frames")

	err := os.Mkdir(framesDir, os.ModeDir|os.ModePerm)
	if err != nil {
		return 0, fmt.Errorf("unable to create frames dir: %w", err)
	}

	framesFilenames, err := ffmpeg.ExtractFrames(videoPath, framesDir, expectedFramesCount)
	if err != nil {
		return 0, fmt.Errorf("unable to extract frames: %w", err)
	}

	collagePath := path.Join(tempDir, "collage.png")

	err = createCollage(framesFilenames, collagePath)
	if err != nil {
		return 0, fmt.Errorf("unable to create collage: %w", err)
	}

	h, err := imagePHash(collagePath)
	if err != nil {
		return 0, fmt.Errorf("unable to calc phash: %w", err)
	}

	return h, err
}

func imagePHash(imagePath string) (uint64, error) {
	input, err := os.Open(imagePath)
	if err != nil {
		return 0, fmt.Errorf("unable to open image file: %w", err)
	}
	defer input.Close()

	img, _, err := image.Decode(input)
	if err != nil {
		return 0, fmt.Errorf("unbale to decode image: %w", err)
	}

	phash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return 0, fmt.Errorf("unable to calculate hash: %w", err)
	}

	return phash.GetHash(), nil
}
