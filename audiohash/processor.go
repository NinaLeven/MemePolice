package audiohash

import (
	"fmt"
	"os"
	"path"

	"github.com/NinaLeven/MemePolice/ffmpeg"
	"github.com/NinaLeven/MemePolice/fsutils"
	"github.com/corona10/goimagehash"
	"github.com/go-fingerprint/fingerprint"
	"github.com/google/uuid"
	"github.com/jo-hoe/chromaprint"
)

func PerceptualHash(audioPath string) (uint64, error) {
	tempDir, err := fsutils.GetTempDir()
	if err != nil {
		return 0, err
	}
	defer fsutils.CleanupTempDir(tempDir)

	h, err := perceptualAudioHash(tempDir, audioPath)
	if err != nil {
		return 0, fmt.Errorf("unable to calculate audio phash: %w", err)
	}

	return h, err
}

func perceptualAudioHash(tempDir, audioPath string) (uint64, error) {
	tempAudioPath := path.Join(tempDir, uuid.NewString()+".mp3")

	err := ffmpeg.PadAudioWithSilence(audioPath, tempAudioPath)
	if err != nil {
		return 0, fmt.Errorf("unable to pad audio: %w", err)
	}

	proc, err := chromaprint.NewBuilder().
		WithPathToChromaprint(os.Getenv("FPCALC_PATH")).
		WithOverlap(true).
		WithMaxFingerPrintLength(60).
		Build()
	if err != nil {
		return 0, fmt.Errorf("unable to create new builder: %w", err)
	}

	fp, err := proc.CreateFingerprints(tempAudioPath)
	if err != nil {
		return 0, fmt.Errorf("unable to get fingerprints: %w", err)
	}

	fps := []int32{}
	for _, f := range fp {
		for _, h := range f.Fingerprint {
			fps = append(fps, int32(h))
		}
	}

	img := fingerprint.ToImage(fps)

	phash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return 0, fmt.Errorf("unable to get audio-image phash: %w", err)
	}

	return phash.GetHash(), nil
}
