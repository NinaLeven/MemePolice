package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	_ "image/jpeg"
	_ "image/png"

	"github.com/corona10/goimagehash"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UpdateHandler struct {
	bot     *tgbotapi.BotAPI
	storage *Storage
	assets  *Assets
}

func NewUpdateHandler(
	bot *tgbotapi.BotAPI,
	storage *Storage,
	assets *Assets,
) *UpdateHandler {
	return &UpdateHandler{
		bot:     bot,
		storage: storage,
		assets:  assets,
	}
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
	switch {
	case update.Message != nil:
		slog.InfoContext(ctx, "update: message", slog.Any("update", update))

		err := r.handleMessage(ctx, update.Message)
		if err != nil {
			return fmt.Errorf("unable to handle message: %w", err)
		}
	default:
		slog.WarnContext(ctx, "unknown update", slog.Any("update", update))
		// more actions coming
	}

	err := r.storage.SetLastUpdateID(ctx, update.UpdateID)
	if err != nil {
		return fmt.Errorf("unable to set last update id: %w", err)
	}

	return nil
}

func (r *UpdateHandler) handleMessage(ctx context.Context, message *tgbotapi.Message) (err error) {
	switch {
	case message.Text == "/why@memalnya_police_bot":
		err := r.handleWhyCommand(ctx, message)
		if err != nil {
			return fmt.Errorf("unable to handle command: %w", err)
		}

	case len(message.Photo) != 0:
		err := r.handleNewPhoto(ctx, message)
		if err != nil {
			return fmt.Errorf("unable to new photo: %w", err)
		}

	default:
		slog.WarnContext(ctx, "unknown message", slog.Any("message", message))
	}

	return nil
}

func (r *UpdateHandler) handleWhyCommand(ctx context.Context, message *tgbotapi.Message) (err error) {
	if message.ReplyToMessage == nil {
		err = r.sendVoiceMessageReply(ctx, message.Chat.ID, message.MessageID, "no_reply", r.assets.GetAudioNoRererence())
		if err != nil {
			return fmt.Errorf("unable to send no_reply voice message %w", err)
		}
		return nil
	}

	repeatedMsg, err := r.storage.GetLastMessageImageHashByID(ctx, message.Chat.ID, message.ReplyToMessage.MessageID)
	if err != nil && !errors.Is(err, &ErrNotFound{}) {
		return fmt.Errorf("unable to get message hash by id: %w", err)
	}
	if err != nil && errors.Is(err, &ErrNotFound{}) {
		err = r.sendVoiceMessageReply(ctx, message.Chat.ID, message.MessageID, "no_repeat", r.assets.GetAudioNoRepeat())
		if err != nil {
			return fmt.Errorf("unable to send no_repeat voice message %w", err)
		}
		return nil
	}

	origMsg, err := r.storage.GetFirstMatchingMessageImageHash(ctx, message.Chat.ID, repeatedMsg.Hash)
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

func (r *UpdateHandler) handleNewPhoto(ctx context.Context, message *tgbotapi.Message) (err error) {
	// photo with max resolution
	photo := message.Photo[len(message.Photo)-1]

	img, err := r.getTelegramImage(ctx, photo.FileID)
	if err != nil {
		return fmt.Errorf("unable to get telegram photo: %w", err)
	}

	imgHash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return fmt.Errorf("unable to calculate image perception hash: %w", err)
	}

	defer func() {
		cerr := r.storage.CreateMessageImageHash(ctx, MessageImageHash{
			MessageID: message.MessageID,
			ChatID:    message.Chat.ID,
			Hash:      imgHash.GetHash(),
		})
		if cerr != nil {
			err = errors.Join(err, fmt.Errorf("unable to save message image hash: %w", cerr))
		}
	}()

	_, err = r.storage.GetLastMatchingMessageImageHash(ctx, message.Chat.ID, imgHash.GetHash())
	if err != nil && !errors.Is(err, &ErrNotFound{}) {
		return fmt.Errorf("unable to get lash matching message image hash: %w", err)
	}
	if err != nil && errors.Is(err, &ErrNotFound{}) {
		slog.InfoContext(ctx, "fresh meme")
		return nil
	}
	slog.InfoContext(ctx, "stale meme")

	err = r.sendStaleMemeReaction(ctx, message.Chat.ID, message.MessageID)
	if err != nil {
		return fmt.Errorf("unable to send stale meme reaction: %w", err)
	}

	return nil
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
