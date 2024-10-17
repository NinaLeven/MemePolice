package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"math/bits"
	"os"
	"path"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

type storageData struct {
	LastUpdateID int
	Chats        map[int64][]MessageImageHash
}

type Storage struct {
	done            chan error
	closed          int32
	storageFilepath string
	m               sync.RWMutex

	storageData
}

func NewStorage(ctx context.Context,
	storageFilepath string,
) *Storage {
	storage := &Storage{
		done:            make(chan error, 1),
		storageFilepath: storageFilepath,
		storageData: storageData{
			Chats: map[int64][]MessageImageHash{},
		},
	}

	err := storage.load()
	if err != nil {
		slog.ErrorContext(ctx, "unable to load storage file", slog.String("error", err.Error()))
	}

	go func() {
		<-ctx.Done()
		err := storage.Close()
		if err != nil {
			slog.ErrorContext(ctx, "unable to close storage", slog.String("error", err.Error()))
		}
	}()

	return storage
}

func (r *Storage) load() error {
	r.m.Lock()
	defer r.m.Unlock()

	file, err := os.Open(r.storageFilepath)
	if err != nil {
		return fmt.Errorf("unable to open storage file: %w", err)
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&r.storageData)
	if err != nil {
		r.storageData = storageData{
			LastUpdateID: 0,
			Chats:        map[int64][]MessageImageHash{},
		}
		return fmt.Errorf("unable to decode stored data: %w", err)
	}

	if r.storageData.Chats == nil {
		r.storageData.Chats = map[int64][]MessageImageHash{}
	}

	return nil
}

func (r *Storage) store() error {
	r.m.Lock()
	defer r.m.Unlock()

	tempFilePath := path.Join(os.TempDir(), fmt.Sprintf("memepolice_%s.json", uuid.NewString()))

	file, err := os.Create(tempFilePath)
	if err != nil {
		return fmt.Errorf("unable to open storage file %w", err)
	}
	defer file.Close()

	err = json.NewEncoder(file).Encode(r.storageData)
	if err != nil {
		return fmt.Errorf("unable to store")
	}

	err = os.Rename(tempFilePath, r.storageFilepath)
	if err != nil {
		return fmt.Errorf("unable to replace old storage file with a new one: %w", err)
	}

	return nil
}

func (r *Storage) Done() chan error {
	return r.done
}

func (r *Storage) Close() (err error) {
	if !atomic.CompareAndSwapInt32(&r.closed, 0, 1) {
		return fmt.Errorf("already closed")
	}
	defer func() {
		r.done <- err
		close(r.done)
	}()

	err = r.store()
	if err != nil {
		return fmt.Errorf("unable to store data: %w", err)
	}

	return nil
}

type MessageImageHash struct {
	MessageID int
	ChatID    int64
	Hash      uint64
}

func (r *Storage) CreateMessageImageHash(ctx context.Context, opts MessageImageHash) error {
	r.m.Lock()
	defer r.m.Unlock()

	r.Chats[opts.ChatID] = append(r.Chats[opts.ChatID], opts)
	return nil
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

func hammingDistance(lhash, rhash uint64) int {
	return bits.OnesCount64(lhash ^ rhash)
}

const cutoffDistance = 7

func getMatchingMessageImageHash(
	chat iter.Seq2[int, MessageImageHash],
	hash uint64,
) (*MessageImageHash, error) {
	for _, item := range chat {
		if hammingDistance(hash, item.Hash) < cutoffDistance {
			return &item, nil
		}
	}

	return nil, &ErrNotFound{
		Err: fmt.Errorf("message image hash not found"),
	}
}

func (r *Storage) GetFirstMatchingMessageImageHash(ctx context.Context, chatID int64, hash uint64) (*MessageImageHash, error) {
	r.m.RLock()
	defer r.m.RUnlock()

	chat, ok := r.Chats[chatID]
	if !ok {
		return nil, &ErrNotFound{
			Err: fmt.Errorf("chat not found: %d", chatID),
		}
	}

	return getMatchingMessageImageHash(slices.All(chat), hash)
}

func (r *Storage) GetLastMatchingMessageImageHash(ctx context.Context, chatID int64, hash uint64) (*MessageImageHash, error) {
	r.m.RLock()
	defer r.m.RUnlock()

	chat, ok := r.Chats[chatID]
	if !ok {
		return nil, &ErrNotFound{
			Err: fmt.Errorf("chat not found: %d", chatID),
		}
	}

	return getMatchingMessageImageHash(slices.Backward(chat), hash)
}

func (r *Storage) GetLastMessageImageHashByID(ctx context.Context, chatID int64, messageID int) (*MessageImageHash, error) {
	r.m.RLock()
	defer r.m.RUnlock()

	chat, ok := r.Chats[chatID]
	if !ok {
		return nil, &ErrNotFound{
			Err: fmt.Errorf("chat not found: %d", chatID),
		}
	}

	for _, item := range slices.Backward(chat) {
		if item.MessageID == messageID {
			return &item, nil
		}
	}

	return nil, &ErrNotFound{
		Err: fmt.Errorf("message image hash not found"),
	}
}

func (r *Storage) SetLastUpdateID(ctx context.Context, lastUpdateID int) error {
	r.m.Lock()
	defer r.m.Unlock()

	r.LastUpdateID = lastUpdateID

	return nil
}

func (r *Storage) GetLastUpdateID(ctx context.Context) (int, error) {
	r.m.RLock()
	defer r.m.RUnlock()

	return r.LastUpdateID, nil
}
