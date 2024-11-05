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
	MessageID      int
	ChatID         int64
	Raw            tgbotapi.Message
	ImageHash      *uint64
	VideoVideoHash *uint64
	VideoAudioHash *uint64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Storage interface {
	UpsertMessage(ctx context.Context, msg Message) error
	GetFirstMatchingMessageByImageHash(ctx context.Context, chatID int64, hash uint64) (*Message, error)
	GetLastMatchingMessageByImageHash(ctx context.Context, chatID int64, hash uint64) (*Message, error)
	GetFirstMatchingMessageByVideoHash(ctx context.Context, chatID int64, videoHash, audioHash uint64) (*Message, error)
	GetLastMatchingMessageByVideoHash(ctx context.Context, chatID int64, videoHash, audioHash uint64) (*Message, error)
	GetMessage(ctx context.Context, chatID int64, messageID int) (*Message, error)

	SetLastUpdateID(ctx context.Context, lastUpdateID int) error
	GetLastUpdateID(ctx context.Context) (int, error)

	CreateTopkek(ctx context.Context, tk Topkek) (int64, error)
	UpdateTopkekStatus(ctx context.Context, id int64, status TopkekStatus) error
	GetLastTopkek(ctx context.Context, chatID int64) (*Topkek, error)
	GetTopkek(ctx context.Context, topkekID int64) (*Topkek, error)
	CreateTopkekMessage(ctx context.Context, msg TopkekMessage) error
	GetTopkekMessages(ctx context.Context, topkekID int64) ([]TopkekMessage, error)
	DeleteTopkekMessages(ctx context.Context, topkekID int64) error
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

type TopkekStatus string

const (
	TopkekStatusCreated TopkekStatus = "created"
	TopkekStatusStarted TopkekStatus = "started"
	TopkekStatusDone    TopkekStatus = "done"
)

type Topkek struct {
	ID        int64        `db:"id"`
	Name      string       `db:"name"`
	ChatID    int64        `db:"chat_id"`
	AuthorID  int64        `db:"author_id"`
	Status    TopkekStatus `db:"status"`
	CreatedAt time.Time    `db:"created_at"`
}

type TopkekMessageType string

const (
	TopkekMessageTypeSrc    TopkekMessageType = "src"
	TopkekMessageTypePoll   TopkekMessageType = "poll"
	TopkekMessageTypeDst    TopkekMessageType = "dst"
	TopkekMessageTypeWinner TopkekMessageType = "win"
)

type TopkekMessage struct {
	TopkekID  int64
	ChatID    int64
	MessageID int
	Type      TopkekMessageType
	Raw       tgbotapi.Message
}
