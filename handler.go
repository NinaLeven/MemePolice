package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	_ "image/jpeg"
	_ "image/png"

	"github.com/corona10/goimagehash"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UpdateHandler struct {
	bot     *tgbotapi.BotAPI
	storage StorageManager
	assets  Assets
}

func NewUpdateHandler(
	bot *tgbotapi.BotAPI,
	storage StorageManager,
	assets Assets,
) *UpdateHandler {
	return &UpdateHandler{
		bot:     bot,
		storage: storage,
		assets:  assets,
	}
}

type textPart string

func (t *textPart) UnmarshalJSON(data []byte) error {
	var str string
	err := json.Unmarshal(data, &str)
	if err == nil {
		*t = textPart(str)
		return nil
	}

	type textItem struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}

	var item textItem
	err = json.Unmarshal(data, &item)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Text: %v: %w", string(data), err)
	}

	*t = textPart(item.Text)

	return nil
}

type text string

func (t *text) UnmarshalJSON(data []byte) error {
	var str string
	err := json.Unmarshal(data, &str)
	if err == nil {
		*t = text(str)
		return nil
	}

	type textItem struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}

	var items []textPart
	err = json.Unmarshal(data, &items)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Text: %v: %w", string(data), err)
	}

	var combinedText string
	for _, item := range items {
		combinedText += string(item) + " "
	}
	*t = text(combinedText)

	return nil
}

func (r *UpdateHandler) OneTimeMigration(ctx context.Context, dataDirectoryPath string) error {
	type message struct {
		ID        int64  `json:"id"`
		Type      string `json:"type"`
		Timestamp string `json:"date_unixtime"`
		FromName  string `json:"from"`
		FromID    string `json:"from_id"`
		PhotoPath string `json:"photo"`
		Text      text   `json:"text"`
	}
	type dump struct {
		Messages []*message `json:"messages"`
	}

	getMessages := func() ([]*message, error) {
		resultFile, err := os.Open(path.Join(dataDirectoryPath, "result.json"))
		if err != nil {
			return nil, fmt.Errorf("unable to open result file: %w", err)
		}
		defer resultFile.Close()

		var data dump

		err = json.NewDecoder(resultFile).Decode(&data)
		if err != nil {
			return nil, fmt.Errorf("unale to unmarshal result file: %w", err)
		}

		return data.Messages, nil
	}

	getPhotoHash := func(pth string) (*uint64, error) {
		if pth == "" {
			return nil, nil
		}

		photo, err := os.Open(path.Join(dataDirectoryPath, pth))
		if err != nil {
			return nil, fmt.Errorf("unable to open photo: %w", err)
		}
		defer photo.Close()

		img, _, err := image.Decode(photo)
		if err != nil {
			return nil, fmt.Errorf("unable to decode image: %w", err)
		}

		imgHash, err := goimagehash.PerceptionHash(img)
		if err != nil {
			return nil, fmt.Errorf("unable to calculate image perception hash: %w", err)
		}

		return ptr(imgHash.GetHash()), nil
	}

	processMessage := func(ctx context.Context, storage Storage, msg *message) error {
		if msg == nil || msg.Type != "message" {
			return nil
		}

		imgHash, err := getPhotoHash(msg.PhotoPath)
		if err != nil {
			return fmt.Errorf("unable to photo hash: %w", err)
		}

		userId, err := strconv.ParseInt(strings.TrimPrefix(msg.FromID, "user"), 10, 64)
		if err != nil {
			slog.ErrorContext(ctx, "unable to pasrse userId", slog.String("err", err.Error()))
		}

		timestampInt64, err := strconv.ParseInt(msg.Timestamp, 10, 64)
		if err != nil {
			return fmt.Errorf("unable to pasrse timestamp string: %w", err)
		}

		timestamp := time.Unix(timestampInt64, 0)

		cerr := storage.UpsertMessage(ctx, Message{
			MessageID: int(msg.ID),
			ChatID:    -1001960713646,
			Raw: tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID: -1001960713646,
				},
				From: &tgbotapi.User{
					ID: userId,
				},
				Text: (string(msg.Text))[0:min(len(msg.Text), 4096)],
			},
			ImageHash: imgHash,
			CreatedAt: timestamp,
			UpdatedAt: timestamp,
		})
		if cerr != nil {
			return fmt.Errorf("unable to upsert message: %w", err)
		}

		return nil
	}

	messages, err := getMessages()
	if err != nil {
		slog.InfoContext(ctx, err.Error())

		return err
	}

	slog.InfoContext(ctx, "got messages", slog.Int("len", len(messages)))

	err = r.storage.ExecWithTx(ctx, func(ctx context.Context, storage Storage) error {
		for i, msg := range messages {
			slog.InfoContext(ctx, "processing message", slog.Int("i", i), slog.Int64("message_id", (msg.ID)))
			err := processMessage(ctx, storage, msg)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, err.Error())
		return fmt.Errorf("unable to exec in tx: %w", err)
	}

	return nil
}

func (r *UpdateHandler) HandleUpdates(ctx context.Context) error {
	lastUpdateID, err := r.storage.GetLastUpdateID(ctx)
	if err != nil {
		return fmt.Errorf("unable to get last update id: %w", err)
	}

	u := tgbotapi.NewUpdate(lastUpdateID)
	u.Timeout = 60

	updates := r.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case update, ok := <-updates:
			if !ok {
				return nil
			}

			slog.InfoContext(ctx, "handling update", slog.Int("update_id", update.UpdateID))

			err := r.handleUpdate(ctx, update)
			if err != nil {
				slog.ErrorContext(ctx, "unable to handle update", slog.String("error", err.Error()))
			}
		}
	}
}

func (r *UpdateHandler) handleUpdate(ctx context.Context, update tgbotapi.Update) error {
	err := r.storage.ExecWithTx(ctx, func(ctx context.Context, storage Storage) error {
		switch {
		case update.Message != nil:
			err := r.handleMessage(ctx, storage, update.Message)
			if err != nil {
				return fmt.Errorf("unable to handle message: %w", err)
			}

		default:
			slog.WarnContext(ctx, "unknown update", slog.Any("update", update))
			// more actions coming
		}

		err := storage.SetLastUpdateID(ctx, update.UpdateID)
		if err != nil {
			return fmt.Errorf("unable to set last update id: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to exec in tx: %w", err)
	}

	return nil
}

func (r *UpdateHandler) handleMessage(ctx context.Context, storage Storage, message *tgbotapi.Message) error {
	err := r.handleWhyCommand(ctx, storage, message)
	if err != nil {
		return fmt.Errorf("unable to handle command: %w", err)
	}

	imageHash, err := r.handleNewPhoto(ctx, storage, message)
	if err != nil {
		slog.ErrorContext(ctx, "unable to handle new photo", slog.String("err", err.Error()))
	}

	err = storage.UpsertMessage(ctx, Message{
		MessageID: message.MessageID,
		ChatID:    message.Chat.ID,
		Raw:       *message,
		ImageHash: imageHash,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("unable to save message image hash: %w", err)
	}

	return nil
}

func (r *UpdateHandler) handleWhyCommand(ctx context.Context, storage Storage, message *tgbotapi.Message) (err error) {
	if message.Text != "/why@memalnya_police_bot" {
		return nil
	}

	if message.ReplyToMessage == nil {
		err = r.sendVoiceMessageReply(ctx, message.Chat.ID, message.MessageID, "no_reply", r.assets.GetAudioNoRererence())
		if err != nil {
			return fmt.Errorf("unable to send no_reply voice message %w", err)
		}
		return nil
	}

	repeatedMsg, err := storage.GetMessage(ctx, message.Chat.ID, message.ReplyToMessage.MessageID)
	if err != nil && !errors.Is(err, &ErrNotFound{}) {
		return fmt.Errorf("unable to get message hash by id: %w", err)
	}
	if err != nil && errors.Is(err, &ErrNotFound{}) || repeatedMsg.ImageHash == nil {
		err = r.sendVoiceMessageReply(ctx, message.Chat.ID, message.MessageID, "no_repeat", r.assets.GetAudioNoRepeat())
		if err != nil {
			return fmt.Errorf("unable to send no_repeat voice message %w", err)
		}
		return nil
	}

	origMsg, err := storage.GetFirstMatchingMessageByImageHash(ctx, message.Chat.ID, *repeatedMsg.ImageHash)
	if err != nil && !errors.Is(err, &ErrNotFound{}) {
		return fmt.Errorf("unable to get first matching message image hash: %w", err)
	}
	if err != nil && errors.Is(err, &ErrNotFound{}) {
		err = r.sendVoiceMessageReply(ctx, message.Chat.ID, message.MessageID, "message_deleted", r.assets.GetAudioMessageDeleted())
		if err != nil {
			return fmt.Errorf("unable to send message_deleted voice message %w", err)
		}
		return nil
	}
	if origMsg.MessageID == repeatedMsg.MessageID {
		err = r.sendVoiceMessageReply(ctx, message.Chat.ID, message.MessageID, "no_repeat", r.assets.GetAudioNoRepeat())
		if err != nil {
			return fmt.Errorf("unable to send no_repeat voice message %w", err)
		}
		return nil
	}

	err = r.sendMessageReply(ctx, message.Chat.ID, origMsg.MessageID, ".")
	if err != nil && !errors.Is(err, &ErrNotFound{}) {
		return fmt.Errorf("unable to reply with text: %w", err)
	}
	if err != nil && errors.Is(err, &ErrNotFound{}) {
		err = r.sendVoiceMessageReply(ctx, message.Chat.ID, message.MessageID, "message_deleted", r.assets.GetAudioMessageDeleted())
		if err != nil {
			return fmt.Errorf("unable to send message_deleted voice message %w", err)
		}
		return nil
	}

	return nil
}

func (r *UpdateHandler) sendVoiceMessageReply(ctx context.Context,
	chatID int64,
	replyToMessageID int,
	name string,
	data []byte,
) error {
	voice := tgbotapi.NewVoice(chatID, tgbotapi.FileBytes{
		Name:  name,
		Bytes: data,
	})
	voice.ReplyToMessageID = replyToMessageID

	_, err := r.bot.Send(voice)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return &ErrNotFound{
				Err: fmt.Errorf("unable to send text message: %w", err),
			}
		}
		return fmt.Errorf("unable to send audio message: %w", err)
	}

	return nil
}

func (r *UpdateHandler) sendMessageReply(ctx context.Context,
	chatID int64,
	replyToMessageID int,
	text string,
) error {
	voice := tgbotapi.NewMessage(chatID, text)
	voice.ReplyToMessageID = replyToMessageID

	_, err := r.bot.Send(voice)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return &ErrNotFound{
				Err: fmt.Errorf("unable to send text message: %w", err),
			}
		}
		return fmt.Errorf("unable to send text message: %w", err)
	}

	return nil
}

func (r *UpdateHandler) handleNewPhoto(ctx context.Context, storage Storage, message *tgbotapi.Message) (*uint64, error) {
	if len(message.Photo) == 0 {
		return nil, nil
	}

	// photo with max resolution
	photo := message.Photo[len(message.Photo)-1]

	img, err := r.getTelegramImage(ctx, photo.FileID)
	if err != nil {
		return nil, fmt.Errorf("unable to get telegram photo: %w", err)
	}

	imgHash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return nil, fmt.Errorf("unable to calculate image perception hash: %w", err)
	}

	_, err = storage.GetLastMatchingMessageByImageHash(ctx, message.Chat.ID, imgHash.GetHash())
	if err != nil && !errors.Is(err, &ErrNotFound{}) {
		return nil, fmt.Errorf("unable to get lash matching message image hash: %w", err)
	}
	if err != nil && errors.Is(err, &ErrNotFound{}) {
		return ptr(imgHash.GetHash()), nil
	}

	err = r.sendStaleMemeReaction(ctx, message.Chat.ID, message.MessageID)
	if err != nil {
		return nil, fmt.Errorf("unable to send stale meme reaction: %w", err)
	}

	return ptr(imgHash.GetHash()), nil
}

const staleMemeEmoji = "✍️"

func (r *UpdateHandler) sendStaleMemeReaction(_ context.Context, chatID int64, messageID int) error {
	params := tgbotapi.Params{
		"chat_id":    strconv.FormatInt(chatID, 10),
		"message_id": strconv.Itoa(messageID),
	}

	err := params.AddInterface("reaction", []any{
		map[string]any{
			"type":  "emoji",
			"emoji": staleMemeEmoji,
		},
	})
	if err != nil {
		return fmt.Errorf("unable to add reaction param: %w", err)
	}

	resp, err := r.bot.MakeRequest("setMessageReaction", params)
	if err != nil {
		return fmt.Errorf("unable to make setMessageReaction request: %w", err)
	}

	if !resp.Ok {
		return fmt.Errorf("error making setMessageReaction reqeust: error_code: %d", resp.ErrorCode)
	}

	return nil
}

func (r *UpdateHandler) getTelegramImage(ctx context.Context, fileID string) (image.Image, error) {
	fileReader, err := r.getTelegramFile(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("unable to get telegram file: %w", err)
	}
	defer fileReader.Close()

	img, _, err := image.Decode(fileReader)
	if err != nil {
		return nil, fmt.Errorf("unable to decode image: %w", err)
	}

	return img, nil
}

func (r *UpdateHandler) getTelegramFile(_ context.Context, fileID string) (io.ReadCloser, error) {
	file, err := r.bot.GetFile(tgbotapi.FileConfig{
		FileID: fileID,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get file link: %w", err)
	}

	resp, err := http.Get(file.Link(r.bot.Token))
	if err != nil {
		return nil, fmt.Errorf("unable to download file: %w", err)
	}

	return resp.Body, nil
}
