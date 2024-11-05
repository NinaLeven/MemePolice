package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (r *UpdateHandler) isCurrentTopkekMessageSrc(ctx context.Context, storage Storage, message *tgbotapi.Message) bool {
	topkek, err := storage.GetLastTopkek(ctx, message.Chat.ID)
	if err != nil {
		slog.ErrorContext(ctx, "unable to get latest topkek", slog.String("err", err.Error()))
		return false
	}

	return topkek.AuthorID == message.From.ID
}

func (r *UpdateHandler) isCurrentTopkekMessage(ctx context.Context, storage Storage, message *tgbotapi.Message) bool {
	topkek, err := storage.GetLastTopkek(ctx, message.Chat.ID)
	if err != nil {
		slog.ErrorContext(ctx, "unable to get latest topkek", slog.String("err", err.Error()))
		return false
	}

	msgs, err := storage.GetTopkekMessages(ctx, topkek.ID)
	if err != nil {
		slog.ErrorContext(ctx, "unable to get topkek mesages", slog.String("err", err.Error()))
		return false
	}

	for _, msg := range msgs {
		if msg.MessageID == message.MessageID && msg.ChatID == message.Chat.ID {
			return true
		}
	}

	return false
}

func (r *UpdateHandler) handleCreateTopkekSrcMessage(ctx context.Context, storage Storage, message *tgbotapi.Message) error {
	if len(message.Photo) == 0 && message.Video == nil {
		return nil
	}

	topkek, err := storage.GetLastTopkek(ctx, message.Chat.ID)
	if err != nil {
		return fmt.Errorf("unable to get latest topkek: %w", err)
	}

	if topkek.Status != TopkekStatusCreated {
		return nil
	}

	err = storage.CreateTopkekMessage(ctx, TopkekMessage{
		TopkekID:  topkek.ID,
		ChatID:    topkek.ChatID,
		MessageID: message.MessageID,
		Type:      TopkekMessageTypeSrc,
		Raw:       *message,
	})
	if err != nil {
		return fmt.Errorf("unable to create topkek message src: %w", err)
	}

	return nil
}

func getNewTopkekName(message *tgbotapi.Message) string {
	args := message.CommandArguments()
	if len(args) > 0 {
		return args
	}

	return fmt.Sprintf("Топкек %d.%d - %d.%d",
		time.Now().UTC().Add(-time.Hour*24*7).Day(),
		time.Now().UTC().Add(-time.Hour*24*7).Month(),
		time.Now().UTC().Day(),
		time.Now().UTC().Month(),
	)
}

func (r *UpdateHandler) handleCreateTopkek(ctx context.Context, storage Storage, message *tgbotapi.Message) error {
	lastTopkek, err := storage.GetLastTopkek(ctx, message.Chat.ID)
	if err != nil && !errors.Is(err, &ErrNotFound{}) {
		return fmt.Errorf("unable to get latest topkek: %w", err)
	}

	slog.InfoContext(ctx, "topkek", slog.Any("topkek", lastTopkek))

	if lastTopkek != nil && lastTopkek.Status != TopkekStatusDone {
		_, err := r.sendMessageReply(ctx, message.Chat.ID, message.MessageID, "топкек уже идет")
		if err != nil {
			return fmt.Errorf("unable to send message reply: %w", err)
		}
		return nil
	}

	_, err = storage.CreateTopkek(ctx, Topkek{
		Name:      getNewTopkekName(message),
		AuthorID:  message.From.ID,
		CreatedAt: time.Now().UTC(),
		ChatID:    message.Chat.ID,
		Status:    TopkekStatusCreated,
	})
	if err != nil {
		return fmt.Errorf("unable to create topkek: %w", err)
	}

	return nil
}

func (r *UpdateHandler) handleStartTopkek(ctx context.Context, storage Storage, message *tgbotapi.Message) error {
	topkek, err := storage.GetLastTopkek(ctx, message.Chat.ID)
	if err != nil {
		return fmt.Errorf("unable to get latest topkek: %w", err)
	}

	if topkek.Status != TopkekStatusCreated {
		_, err := r.sendMessageReply(ctx, message.Chat.ID, message.MessageID, "создай топкек")
		if err != nil {
			return fmt.Errorf("unable to send message reply: %w", err)
		}
		return nil
	}

	err = r.startTopkek(ctx, storage, topkek.ID)
	if err != nil && !errors.Is(err, errNotEnoughTopkekSrcs) {
		return fmt.Errorf("unable to start topkek: %w", err)
	}
	if err != nil && errors.Is(err, errNotEnoughTopkekSrcs) {
		_, err := r.sendMessageReply(ctx, message.Chat.ID, message.MessageID, "надо хотя бы два мема")
		if err != nil {
			return fmt.Errorf("unable to send message reply: %w", err)
		}
		return errNotEnoughTopkekSrcs
	}

	return nil
}

func (r *UpdateHandler) getTopkekMessages(ctx context.Context, storage Storage, topkekID int64, msgType TopkekMessageType) ([]*tgbotapi.Message, error) {
	topkekMessages, err := storage.GetTopkekMessages(ctx, topkekID)
	if err != nil {
		return nil, fmt.Errorf("unable to start topkek: %w", err)
	}

	srcMessages := []*tgbotapi.Message{}

	for _, topkekMessage := range topkekMessages {
		if topkekMessage.Type != msgType {
			continue
		}

		srcMessages = append(srcMessages, &topkekMessage.Raw)
	}

	return srcMessages, nil
}

func maxChunkSize(n int) int {
	if n <= 10 {
		return n
	}
	c := 10
	for i := 10; i > 5; i-- {
		if i-n%i < c-n%c {
			c = i
		}
		if n%i == 0 {
			return i
		}
	}
	return c
}

func chunkMessages(srcs []*tgbotapi.Message) [][]*tgbotapi.Message {
	chunkSize := maxChunkSize(len(srcs))

	res := [][]*tgbotapi.Message{}
	for i := 0; i < len(srcs); i += chunkSize {
		end := i + chunkSize
		if end > len(srcs) {
			end = len(srcs)
		}
		res = append(res, srcs[i:end])
	}

	return res
}

var errNotEnoughTopkekSrcs = errors.New("not enough topkek srcs")

func (r *UpdateHandler) startTopkek(ctx context.Context, storage Storage, topkekID int64) error {
	topkek, err := storage.GetTopkek(ctx, topkekID)
	if err != nil {
		return fmt.Errorf("unable to get topkek: %w", err)
	}

	srcs, err := r.getTopkekMessages(ctx, storage, topkekID, TopkekMessageTypeSrc)
	if err != nil {
		return fmt.Errorf("unable to get topkek src messages: %w", err)
	}

	if len(srcs) < 2 {
		return errNotEnoughTopkekSrcs
	}

	chunks := chunkMessages(srcs)
	i := 0
	for _, chunk := range chunks {
		err := r.sendTopkekChunk(ctx, storage, topkek, chunk, i)
		if err != nil {
			return fmt.Errorf("unable to send topkek chunk: %w", err)
		}
		i += len(chunk)
	}

	for _, msg := range srcs {
		err := r.deleteMessage(ctx, topkek.ChatID, msg.MessageID)
		if err != nil {
			slog.ErrorContext(ctx, "unable to delete src topkek message", slog.String("err", err.Error()))
		}
	}

	err = storage.UpdateTopkekStatus(ctx, topkekID, TopkekStatusStarted)
	if err != nil {
		return fmt.Errorf("unable to update topkek: %w", err)
	}

	return nil
}

func (r *UpdateHandler) sendTopkekChunk(ctx context.Context, storage Storage, topkek *Topkek, chunk []*tgbotapi.Message, i int) error {
	pollAnswers := []string{}
	files := []any{}

	for _, msg := range chunk {
		switch {
		case len(msg.Photo) > 0:
			photo := msg.Photo[len(msg.Photo)-1]
			files = append(files, tgbotapi.NewInputMediaPhoto(tgbotapi.FileID(photo.FileID)))

		case msg.Video != nil:
			files = append(files, tgbotapi.NewInputMediaVideo(tgbotapi.FileID(msg.Video.FileID)))

		default:
			continue
		}

		pollAnswers = append(pollAnswers, strconv.Itoa(i+1))
		i++
	}

	msgIds, err := r.sendMediaGroup(ctx, topkek.ChatID, files)
	if err != nil {
		return fmt.Errorf("unable to send media group: %w", err)
	}

	for _, msg := range msgIds {
		err := storage.CreateTopkekMessage(ctx, TopkekMessage{
			TopkekID:  topkek.ID,
			ChatID:    topkek.ChatID,
			MessageID: msg.MessageID,
			Type:      TopkekMessageTypeDst,
			Raw:       msg,
		})
		if err != nil {
			return fmt.Errorf("unable to create topkek dst message: %w", err)
		}
	}

	pollRes, err := r.sendSimplePoll(ctx, topkek.ChatID, topkek.Name, pollAnswers)
	if err != nil {
		return fmt.Errorf("unable to send topkek poll: %w", err)
	}

	err = storage.CreateTopkekMessage(ctx, TopkekMessage{
		TopkekID:  topkek.ID,
		ChatID:    topkek.ChatID,
		MessageID: pollRes.MessageID,
		Type:      TopkekMessageTypePoll,
		Raw:       *pollRes,
	})
	if err != nil {
		return fmt.Errorf("unable to create topkek dst message: %w", err)
	}

	return nil
}

func (r *UpdateHandler) sendMediaGroup(ctx context.Context,
	chatID int64,
	files []any,
) ([]tgbotapi.Message, error) {
	msgs, err := r.bot.SendMediaGroup(tgbotapi.NewMediaGroup(chatID, files))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, &ErrNotFound{
				Err: fmt.Errorf("unable to send media group: %w", err),
			}
		}
		return nil, fmt.Errorf("unable to send media group: %w", err)
	}

	return msgs, nil
}

func (r *UpdateHandler) sendSimplePoll(ctx context.Context,
	chatID int64,
	question string,
	answers []string,
) (*tgbotapi.Message, error) {
	poll := tgbotapi.NewPoll(chatID, question, answers...)
	poll.AllowsMultipleAnswers = true
	poll.IsAnonymous = false

	msg, err := r.bot.Send(poll)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, &ErrNotFound{
				Err: fmt.Errorf("unable to send poll: %w", err),
			}
		}
		return nil, fmt.Errorf("unable to send poll: %w", err)
	}

	return &msg, nil
}

func (r *UpdateHandler) handleFinishTopkek(ctx context.Context, storage Storage, message *tgbotapi.Message) error {
	topkek, err := storage.GetLastTopkek(ctx, message.Chat.ID)
	if err != nil {
		return fmt.Errorf("unable to get latest topkek: %w", err)
	}

	if message.CommandArguments() == "-f" {
		err := r.finishTopkekForce(ctx, storage, topkek.ID)
		if err != nil {
			return fmt.Errorf("unbale to force finish topkek: %w", err)
		}
		return nil
	}

	if topkek.Status == TopkekStatusCreated {
		err = r.finishTopkekForce(ctx, storage, topkek.ID)
		if err != nil {
			return fmt.Errorf("unable to finish topkek: %w", err)
		}
		return nil
	}
	if topkek.Status != TopkekStatusStarted {
		_, err := r.sendMessageReply(ctx, message.Chat.ID, message.MessageID, "сначала начни топкек, шкура")
		if err != nil {
			return fmt.Errorf("unable to send message reply: %w", err)
		}
		return nil
	}

	err = r.finishTopkek(ctx, storage, topkek.ID)
	if err != nil {
		return fmt.Errorf("unable to finish topkek: %w", err)
	}

	return nil
}

func getPollWinners(opts []tgbotapi.PollOption) []int {
	m := 0
	for _, opt := range opts {
		if opt.VoterCount > m {
			m = opt.VoterCount
		}
	}

	res := []int{}
	for _, opt := range opts {
		num, err := strconv.Atoi(opt.Text)
		if err != nil {
			slog.Error("unable to parse poll result text", slog.String("err", err.Error()))
		}
		if opt.VoterCount != m {
			continue
		}
		res = append(res, num-1)
	}

	return res
}

func (r *UpdateHandler) sendPhotoRepy(ctx context.Context,
	chatID int64,
	replyMessageID int,
	text string,
	fileID string,
) (*tgbotapi.Message, error) {
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(fileID))
	photo.Caption = text
	photo.ReplyToMessageID = replyMessageID

	msg, err := r.bot.Send(photo)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, &ErrNotFound{
				Err: fmt.Errorf("unable to send photo reply: %w", err),
			}
		}
		return nil, fmt.Errorf("unable to send photo reply: %w", err)
	}

	return &msg, nil
}

func (r *UpdateHandler) sendVideoRepy(ctx context.Context,
	chatID int64,
	replyMessageID int,
	text string,
	fileID string,
) (*tgbotapi.Message, error) {
	photo := tgbotapi.NewVideo(chatID, tgbotapi.FileID(fileID))
	photo.Caption = text
	photo.ReplyToMessageID = replyMessageID

	msg, err := r.bot.Send(photo)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, &ErrNotFound{
				Err: fmt.Errorf("unable to send video reply: %w", err),
			}
		}
		return nil, fmt.Errorf("unable to send video reply: %w", err)
	}

	return &msg, nil
}

func (r *UpdateHandler) finishTopkekForce(ctx context.Context, storage Storage, topkekID int64) error {
	err := storage.UpdateTopkekStatus(ctx, topkekID, TopkekStatusDone)
	if err != nil {
		return fmt.Errorf("unable to update topkek: %w", err)
	}

	return nil
}

func (r *UpdateHandler) finishTopkek(ctx context.Context, storage Storage, topkekID int64) error {
	topkek, err := storage.GetTopkek(ctx, topkekID)
	if err != nil {
		return fmt.Errorf("unable to get topkek: %w", err)
	}

	pollIds, err := r.getTopkekMessages(ctx, storage, topkekID, TopkekMessageTypePoll)
	if err != nil {
		return fmt.Errorf("unable to get topkek polls: %w", err)
	}

	destMsgs, err := r.getTopkekMessages(ctx, storage, topkekID, TopkekMessageTypeDst)
	if err != nil {
		return fmt.Errorf("unable to get topkek descs: %w", err)
	}

	pollResutls := []tgbotapi.PollOption{}

	for _, pollMsg := range pollIds {
		poll, err := r.bot.StopPoll(tgbotapi.NewStopPoll(topkek.ChatID, pollMsg.MessageID))
		if err != nil {
			return fmt.Errorf("unable to stop poll: %w", err)
		}
		pollResutls = append(pollResutls, poll.Options...)
	}

	winners := getPollWinners(pollResutls)
	if len(winners) == 0 {
		return fmt.Errorf("0 topkek winners: %w", err)
	}

	if len(winners) > 1 {
		winnerMsgs := []*tgbotapi.Message{}
		for _, winner := range winners {
			winnerMsgs = append(winnerMsgs, destMsgs[winner])
		}

		err := r.restartTopkek(ctx, storage, topkek, winnerMsgs)
		if err != nil {
			return fmt.Errorf("unable to restart topkek: %w", err)
		}

		return nil
	}

	winner := winners[0]

	winnerMsg := destMsgs[winner]
	var winnerMsgRes *tgbotapi.Message

	switch {
	case len(winnerMsg.Photo) > 0:
		photo := winnerMsg.Photo[len(winnerMsg.Photo)-1]
		winnerMsgRes, err = r.sendPhotoRepy(ctx,
			topkek.ChatID,
			winnerMsg.MessageID,
			fmt.Sprintf("Победитель %s", topkek.Name),
			photo.FileID,
		)
		if err != nil {
			return fmt.Errorf("unable to send winner photo: %w", err)
		}

	case winnerMsg.Video != nil:
		winnerMsgRes, err = r.sendVideoRepy(ctx,
			topkek.ChatID,
			winnerMsg.MessageID,
			fmt.Sprintf("Победитель %s", topkek.Name),
			winnerMsg.Video.FileID,
		)
		if err != nil {
			return fmt.Errorf("unable to send winner video: %w", err)
		}
	}

	err = storage.CreateTopkekMessage(ctx, TopkekMessage{
		TopkekID:  topkekID,
		ChatID:    topkek.ChatID,
		MessageID: winnerMsgRes.MessageID,
		Type:      TopkekMessageTypeWinner,
		Raw:       *winnerMsgRes,
	})
	if err != nil {
		return fmt.Errorf("unable to cerate topkek winner msg: %w", err)
	}

	err = storage.UpdateTopkekStatus(ctx, topkekID, TopkekStatusDone)
	if err != nil {
		return fmt.Errorf("unable to update topkek: %w", err)
	}

	return nil
}

func (r *UpdateHandler) restartTopkek(ctx context.Context, storage Storage, topkek *Topkek, srcs []*tgbotapi.Message) error {
	err := storage.DeleteTopkekMessages(ctx, topkek.ID)
	if err != nil {
		return fmt.Errorf("unable to delete topkek old topkek msgs: %w", err)
	}

	chunks := chunkMessages(srcs)
	i := 0
	for _, chunk := range chunks {
		err := r.sendTopkekChunk(ctx, storage, topkek, chunk, i)
		if err != nil {
			return fmt.Errorf("unable to send topkek chunk: %w", err)
		}
		i += len(chunk)
	}

	for _, msg := range srcs {
		err := r.deleteMessage(ctx, topkek.ChatID, msg.MessageID)
		if err != nil {
			slog.ErrorContext(ctx, "unable to delete src topkek message", slog.String("err", err.Error()))
		}
	}

	err = storage.UpdateTopkekStatus(ctx, topkek.ID, TopkekStatusStarted)
	if err != nil {
		return fmt.Errorf("unable to update topkek: %w", err)
	}

	return nil
}
