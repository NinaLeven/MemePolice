package main

import (
	"fmt"
	"os"
	"path"
)

type Assets struct {
	assetsDirectoryPath string

	messageDeleted []byte
	noReference    []byte
	noRepeat       []byte
}

func NewAssets(assetsDirectoryPath string) (*Assets, error) {
	assets := &Assets{
		assetsDirectoryPath: assetsDirectoryPath,
	}

	err := assets.load()
	if err != nil {
		return nil, fmt.Errorf("unable to load assets: %w", err)
	}

	return assets, nil
}

func (r *Assets) load() error {
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

func (r *Assets) loadAudioMessageDeleted() error {
	file, err := os.ReadFile(path.Join(r.assetsDirectoryPath, "message_deleted.mp3"))
	if err != nil {
		return fmt.Errorf("unable to open message_deleted file: %w", err)
	}
	r.messageDeleted = file
	return nil
}

func (r *Assets) loadAudioNoRererence() error {
	file, err := os.ReadFile(path.Join(r.assetsDirectoryPath, "no_reference.mp3"))
	if err != nil {
		return fmt.Errorf("unable to open no_reference file: %w", err)
	}
	r.noReference = file
	return nil
}

func (r *Assets) loadAudioNoRepeat() error {
	file, err := os.ReadFile(path.Join(r.assetsDirectoryPath, "no_repeat.mp3"))
	if err != nil {
		return fmt.Errorf("unable to open no_repeat file: %w", err)
	}
	r.noRepeat = file
	return nil
}

func (r *Assets) GetAudioMessageDeleted() []byte {
	return r.messageDeleted
}

func (r *Assets) GetAudioNoRererence() []byte {
	return r.noReference
}

func (r *Assets) GetAudioNoRepeat() []byte {
	return r.noRepeat
}
