package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	ctx := context.Background()

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, os.Kill)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	storageFilePath := flag.String("s", "storage.json", "path to storage file")
	assetsDirPath := flag.String("a", "assets", "path to assets directory")
	flag.Parse()

	bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Panic(fmt.Errorf("unable to create bot: %w", err))
	}
	defer bot.StopReceivingUpdates()

	assets, err := NewAssets(*assetsDirPath)
	if err != nil {
		log.Panic(err)
	}

	storage := NewStorage(ctx, *storageFilePath)
	defer storage.Close()

	updateHandler := NewUpdateHandler(bot, storage, assets)

	err = updateHandler.HandleUpdates(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "unable to handle updates", slog.String("err", err.Error()))
	}
	cancel()

	select {
	case <-storage.Done():
		return
	case <-time.After(time.Minute):
		return
	}
}
