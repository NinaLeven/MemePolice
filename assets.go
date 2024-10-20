package main

import (
	"fmt"
	"os"
	"path"
)

type FileAssets struct {
	assetsDirectoryPath string

	messageDeleted []byte
	noReference    []byte
	noRepeat       []byte
}

func NewFileAssets(assetsDirectoryPath string) (*FileAssets, error) {
	assets := &FileAssets{
		assetsDirectoryPath: assetsDirectoryPath,
	}

	err := assets.load()
	if err != nil {
		return nil, fmt.Errorf("unable to load assets: %w", err)
	}

	return assets, nil
}

func (r *FileAssets) load() error {
	var err error

	err = r.loadAudioMessageDeleted()
	if err != nil {
		return fmt.Errorf("unable to load file: %w", err)
	}
	err = r.loadAudioNoRererence()
	if err != nil {
		return fmt.Errorf("unable to load file: %w", err)
	}
	err = r.loadAudioNoRepeat()
	if err != nil {
		return fmt.Errorf("unable to load file: %w", err)
	}

	return nil
}

func (r *FileAssets) loadAudioMessageDeleted() error {
	file, err := os.ReadFile(path.Join(r.assetsDirectoryPath, "message_deleted.mp3"))
	if err != nil {
		return fmt.Errorf("unable to open message_deleted file: %w", err)
	}
	r.messageDeleted = file
	return nil
}

func (r *FileAssets) loadAudioNoRererence() error {
	file, err := os.ReadFile(path.Join(r.assetsDirectoryPath, "no_reference.mp3"))
	if err != nil {
		return fmt.Errorf("unable to open no_reference file: %w", err)
	}
	r.noReference = file
	return nil
}

func (r *FileAssets) loadAudioNoRepeat() error {
	file, err := os.ReadFile(path.Join(r.assetsDirectoryPath, "no_repeat.mp3"))
	if err != nil {
		return fmt.Errorf("unable to open no_repeat file: %w", err)
	}
	r.noRepeat = file
	return nil
}

func (r *FileAssets) GetAudioMessageDeleted() []byte {
	return r.messageDeleted
}

func (r *FileAssets) GetAudioNoRererence() []byte {
	return r.noReference
}

func (r *FileAssets) GetAudioNoRepeat() []byte {
	return r.noRepeat
}
