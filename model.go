package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ptr[T any](t T) *T {
	return &t
}

func val[T any](t *T) T {
	if t == nil {
		var res T
		return res
	}
	return *t
}

type ErrNotFound struct {
	Err error
}

func (e *ErrNotFound) Error() string {
	if e.Err == nil {
		return "not found"
	}
	return fmt.Sprintf("not found: %s", e.Err.Error())
}

func (e *ErrNotFound) Is(target error) bool {
	_, ok := target.(*ErrNotFound)
	return ok
}

func (e *ErrNotFound) As(target interface{}) bool {
	if _, ok := target.(*ErrNotFound); ok {
		*target.(*ErrNotFound) = *e
		return true
	}
	return errors.As(e.Err, target)
}

type Message struct {
	MessageID int
	ChatID    int64
	Raw       tgbotapi.Message
	ImageHash *uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Storage interface {
	UpsertMessage(ctx context.Context, msg Message) error
	GetFirstMatchingMessageByImageHash(ctx context.Context, chatID int64, hash uint64) (*Message, error)
	GetLastMatchingMessageByImageHash(ctx context.Context, chatID int64, hash uint64) (*Message, error)
	GetMessage(ctx context.Context, chatID int64, messageID int) (*Message, error)

	SetLastUpdateID(ctx context.Context, lastUpdateID int) error
	GetLastUpdateID(ctx context.Context) (int, error)
}

type StorageManager interface {
	Storage

	ExecWithTx(ctx context.Context, handler func(ctx context.Context, storage Storage) error) error
}

type Assets interface {
	GetAudioMessageDeleted() []byte
	GetAudioNoRererence() []byte
	GetAudioNoRepeat() []byte
}
