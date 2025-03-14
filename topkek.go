package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tg "github.com/OvyFlash/telegram-bot-api"
)

const defaultMinimumReactions = 5

func parseCreateTopkekOptions(message *tg.Message) createTopkekOptions {
	opts := createTopkekOptions{
		ChatID:   message.Chat.ID,
		AuthorID: message.From.ID,
		Name: fmt.Sprintf("Топкек %d.%d",
			time.Now().UTC().Day(),
			time.Now().UTC().Month(),
		),
		MinReactions: defaultMinimumReactions,
	}

	if message.ReplyToMessage != nil {
		opts.StartingMessageID = &message.ReplyToMessage.MessageID
	}

	args := strings.Split(message.CommandArguments(), " ")

	fs := flag.NewFlagSet("parser", flag.ContinueOnError)
	mFlag := fs.String("m", "", "An integer value")

	err := fs.Parse(args)
	if err != nil {
		return opts
	}

	if *mFlag != "" {
		mValue, err := strconv.Atoi(*mFlag)
		if err != nil {
			slog.Warn("invalid -m flag", "-m", *mFlag)
		}
		opts.MinReactions = mValue
	}

	positionalArgs := fs.Args()
	if len(positionalArgs) > 0 {
		opts.Name = strings.Join(positionalArgs, " ")
	}

	return opts
}

func (r *UpdateHandler) handleCreateTopkek(ctx context.Context, storage Storage, message *tg.Message) error {
	opts := parseCreateTopkekOptions(message)

	topkekID, err := r.createTopkek(ctx, storage, opts)
	if err != nil &&
		!errors.Is(err, errNotEnoughTopkekSrcs) &&
		!errors.Is(err, errTopkekAlreadyInProgress) {
		return fmt.Errorf("unable to create topkek: %w", err)
	}
	if err != nil && errors.Is(err, errNotEnoughTopkekSrcs) {
		_, err := r.sendMessageReply(ctx, message.Chat.ID, message.MessageID, "надо хотя бы два мема")
		if err != nil {
			return fmt.Errorf("unable to send message reply: %w", err)
		}
		return errNotEnoughTopkekSrcs
	}
	if err != nil && errors.Is(err, errTopkekAlreadyInProgress) {
		_, err := r.sendMessageReply(ctx, message.Chat.ID, message.MessageID, "топкек уже идет")
		if err != nil {
			return fmt.Errorf("unable to send message reply: %w", err)
		}
		return errTopkekAlreadyInProgress
	}

	err = r.startTopkek(ctx, storage, topkekID)
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

var errTopkekAlreadyInProgress = errors.New("not enough topkek already in progress")

type createTopkekOptions struct {
	MessageID         int
	ChatID            int64
	Name              string
	AuthorID          int64
	StartingMessageID *int
	MinReactions      int
}

func (r *UpdateHandler) createTopkek(ctx context.Context, storage Storage, opts createTopkekOptions) (int64, error) {
	lastTopkek, err := storage.GetLastTopkek(ctx, opts.ChatID)
	if err != nil && !errors.Is(err, &ErrNotFound{}) {
		return 0, fmt.Errorf("unable to get latest topkek: %w", err)
	}

	if lastTopkek != nil && lastTopkek.Status != TopkekStatusDone {
		return 0, errTopkekAlreadyInProgress
	}

	listOpts := ListMessagesWithReactionCountOptions{
		ChatID:            opts.ChatID,
		MinReactions:      opts.MinReactions,
		StartingMessageID: lastTopkek.MessageID,
		ExcludeReactions: [2]string{
			RepeatedMemeEmoji,
			StaleMemeEmoji,
		},
	}
	if opts.StartingMessageID != nil {
		listOpts.StartingMessageID = *opts.StartingMessageID
	}

	sourceMessages, err := storage.ListMessagesWithReactionCount(ctx, listOpts)
	if err != nil {
		return 0, fmt.Errorf("unable to find topkek source messages: %w", err)
	}

	if len(sourceMessages) < 2 {
		return 0, errNotEnoughTopkekSrcs
	}

	topkekID, err := storage.CreateTopkek(ctx, Topkek{
		Name:      opts.Name,
		AuthorID:  opts.AuthorID,
		ChatID:    opts.ChatID,
		MessageID: opts.MessageID,
		Status:    TopkekStatusCreated,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return 0, fmt.Errorf("unable to create topkek: %w", err)
	}

	for _, msg := range sourceMessages {
		err := storage.CreateTopkekMessage(ctx, TopkekMessage{
			TopkekID:  topkekID,
			ChatID:    opts.ChatID,
			MessageID: msg.MessageID,
			Type:      TopkekMessageTypeSrc,
			Raw:       msg.Raw,
		})
		if err != nil {
			return 0, fmt.Errorf("unable to create topkek src message: %w", err)
		}
	}

	return topkekID, nil
}

func (r *UpdateHandler) getTopkekMessages(ctx context.Context, storage Storage, topkekID int64, msgType TopkekMessageType) ([]*tg.Message, error) {
	topkekMessages, err := storage.GetTopkekMessages(ctx, topkekID)
	if err != nil {
		return nil, fmt.Errorf("unable to start topkek: %w", err)
	}

	srcMessages := []*tg.Message{}

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

func chunkMessages(srcs []*tg.Message) [][]*tg.Message {
	chunkSize := maxChunkSize(len(srcs))

	res := [][]*tg.Message{}
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

func (r *UpdateHandler) sendTopkekChunk(ctx context.Context, storage Storage, topkek *Topkek, chunk []*tg.Message, i int) error {
	pollAnswers := []string{}
	files := []any{}

	for _, msg := range chunk {
		switch {
		case len(msg.Photo) > 0:
			photo := msg.Photo[len(msg.Photo)-1]
			files = append(files, tg.NewInputMediaPhoto(tg.FileID(photo.FileID)))

		case msg.Video != nil:
			files = append(files, tg.NewInputMediaVideo(tg.FileID(msg.Video.FileID)))

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
) ([]tg.Message, error) {
	msgs, err := r.bot.SendMediaGroup(tg.NewMediaGroup(chatID, files))
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
) (*tg.Message, error) {
	opts := make([]tg.InputPollOption, 0, len(answers))
	for _, a := range answers {
		opts = append(opts, tg.NewPollOption(a))
	}

	poll := tg.NewPoll(chatID, question, opts...)
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

func (r *UpdateHandler) handleFinishTopkek(ctx context.Context, storage Storage, message *tg.Message) error {
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

func getPollWinners(opts []tg.PollOption) []int {
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
) (*tg.Message, error) {
	photo := tg.NewPhoto(chatID, tg.FileID(fileID))
	photo.Caption = text
	photo.ReplyParameters = tg.ReplyParameters{
		MessageID: replyMessageID,
	}

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
) (*tg.Message, error) {
	photo := tg.NewVideo(chatID, tg.FileID(fileID))
	photo.Caption = text
	photo.ReplyParameters = tg.ReplyParameters{
		MessageID: replyMessageID,
	}

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

	pollResutls := []tg.PollOption{}

	for _, pollMsg := range pollIds {
		poll, err := r.bot.StopPoll(tg.NewStopPoll(topkek.ChatID, pollMsg.MessageID))
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
		winnerMsgs := []*tg.Message{}
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
	var winnerMsgRes *tg.Message

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

func (r *UpdateHandler) restartTopkek(ctx context.Context, storage Storage, topkek *Topkek, srcs []*tg.Message) error {
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

const helpText = `Топкек инструкция:
* Создай топкек - /topkek
* Загрузи фото\видео
* Начни топкек /start
* Ждем сколько надо голосования
* Завершаем топкек /stop`

func (r *UpdateHandler) handleHelp(ctx context.Context, storage Storage, message *tg.Message) error {
	_, err := r.sendMessage(ctx, message.Chat.ID, helpText)
	if err != nil {
		return fmt.Errorf("unable to send text message: %w", err)
	}

	return nil
}
