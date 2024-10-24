package ffmpeg

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/NinaLeven/MemePolice/fsutils"
)

func runCmd(cmdName string, args ...string) (string, error) {
	cmd := exec.Command(cmdName, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func getTotalFrameCount(videoPath string) (int, error) {
	stdout, err := runCmd("ffprobe", "-v", "quiet", "-print_format", "json", "-show_entries", "format=duration", "-show_entries", "stream=codec_type,r_frame_rate", videoPath)
	if err != nil {
		return 0, fmt.Errorf("unable to get video info: %w: %s", err, stdout)
	}

	type stream struct {
		CodecType string `json:"codec_type"`
		FrameRate string `json:"r_frame_rate"`
	}

	type format struct {
		Duration string `json:"duration"`
	}

	type info struct {
		Streams []stream `json:"streams"`
		Format  format   `json:"format"`
	}

	var inf info
	err = json.Unmarshal([]byte(stdout), &inf)
	if err != nil {
		return 0, fmt.Errorf("unable to unmarshal video info: %w", err)
	}

	var videoStream *stream
	for _, s := range inf.Streams {
		if s.CodecType == "video" {
			videoStream = &s
			break
		}
	}
	if videoStream == nil {
		return 0, fmt.Errorf("unable to find video stream: %w", err)
	}

	matches := frameRateRegex.FindStringSubmatch(videoStream.FrameRate)
	if len(matches) != 3 {
		return 0, fmt.Errorf("frame rate unrecognized: %v", matches)
	}

	fps, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("unable to parse frame count: %w", err)
	}

	frameSecondsCount, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, fmt.Errorf("unable to parse frame seconds count: %w", err)
	}
	if frameSecondsCount == 0 {
		return 0, fmt.Errorf("0 seconds")
	}

	secondsCount, err := strconv.ParseFloat(inf.Format.Duration, 64)
	if err != nil {
		return 0, fmt.Errorf("unable to parse seconds count: %w", err)
	}

	return int(math.Ceil(float64(fps) / float64(frameSecondsCount) * secondsCount)), nil
}

var frameRateRegex = regexp.MustCompile(`^(\d+)/(\d+)$`)

func ExtractFrames(videoPath, framesDir string, expectedFramesCount int) ([]string, error) {
	frameCount, err := getTotalFrameCount(videoPath)
	if err != nil {
		return nil, fmt.Errorf("unable to get frame count: %w", err)
	}

	framesInterval := max(1, frameCount/expectedFramesCount)

	stdout, err := runCmd("ffmpeg", "-i", videoPath, "-vf", "select='not(mod(n,"+strconv.Itoa(framesInterval)+"))'", "-vsync", "vfr", path.Join(framesDir, "%03d.png"))
	if err != nil {
		return nil, fmt.Errorf("unable to run ffmpeg: %w: %s", err, stdout)
	}

	frames, err := fsutils.LS(framesDir)
	if err != nil {
		return nil, fmt.Errorf("unable to list files: %w", err)
	}

	slices.Sort(frames)

	frames = frames[:min(len(frames), expectedFramesCount)]

	if len(frames) == 0 {
		return nil, fmt.Errorf("unable to extract frames: empty")
	}

	return frames, nil
}

func ExtractAudio(videoPath, audioPath string) error {
	stdout, err := runCmd("ffmpeg", "-i", videoPath, "-q:a", "0", "-map", "a?" /*"-vn", "-acodec", "mp3",*/, audioPath)
	if err != nil {
		return fmt.Errorf("unable to run ffmpeg: %w: %s", err, stdout)
	}

	return nil
}

func getAudioLen(audioPath string) (int, error) {
	stdout, err := runCmd("ffprobe", "-i", audioPath, "-show_entries", "format=duration", "-v", "quiet", "-of", "csv=p=0")
	if err != nil {
		return 0, fmt.Errorf("unable to get video info: %w: %s", err, stdout)
	}

	secondsCount, err := strconv.ParseFloat(strings.TrimSpace(stdout), 64)
	if err != nil {
		return 0, fmt.Errorf("unable to parse seconds count: %w", err)
	}

	return int(math.Ceil(secondsCount)), nil
}

const maxPddingSeconds = 3

func PadAudioWithSilence(inputAudioPath, outputAudioPath string) error {
	audioLen, err := getAudioLen(inputAudioPath)
	if err != nil {
		return fmt.Errorf("unable to get audio length: %w", err)
	}

	if audioLen < maxPddingSeconds {
		audioLen = maxPddingSeconds
	}

	stdout, err := runCmd("ffmpeg", "-i", inputAudioPath, "-af", "apad,atrim=end="+strconv.Itoa(audioLen) /*"-t", strconv.Itoa(maxPddingSeconds-audioLen),*/, outputAudioPath)
	if err != nil {
		return fmt.Errorf("unable to run ffmpeg: %w: %s", err, stdout)
	}

	return nil
}
