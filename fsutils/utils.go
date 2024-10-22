package fsutils

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/google/uuid"
)

func GetTempDir() (string, error) {
	tempDir := path.Join(os.TempDir(), uuid.NewString())
	err := os.Mkdir(tempDir, os.ModeDir|os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("unable to get temp dir: %w", err)
	}
	return tempDir, nil
}

func CleanupTempDir(tempDir string) error {
	err := os.RemoveAll(tempDir)
	if err != nil {
		return fmt.Errorf("unable to remove temp dir(%s): %w", tempDir, err)
	}
	return nil
}

func CP(inputPath string, outputPath string) error {
	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("unable to open output file: %w", err)
	}
	defer input.Close()

	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("unable to open output file: %w", err)
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	if err != nil {
		return fmt.Errorf("unable to copy file: %w", err)
	}

	return nil
}

func LS(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("unable to list files: %w", err)
	}
	res := []string{}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		res = append(res, path.Join(dir, f.Name()))
	}
	return res, nil
}
