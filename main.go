package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	// tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	ctx := context.Background()

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, os.Kill)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	assetsDirPath := flag.String("a", "assets", "path to assets directory")
	migrationsDirPath := flag.String("m", "migrations", "path to migrations directory")
	dumpDirPath := flag.String("d", "dump", "path to dump directory")
	postgresURL := flag.String("p", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable", "postgres url")
	flag.Parse()

	// bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	// if err != nil {
	// 	log.Panic(fmt.Errorf("unable to create bot: %w", err))
	// }
	// defer bot.StopReceivingUpdates()

	assets, err := NewFileAssets(*assetsDirPath)
	if err != nil {
		log.Panic(err)
	}

	psqlStorage, err := NewPSQLStorageManager(ctx, *postgresURL, *migrationsDirPath)
	if err != nil {
		log.Panic(err)
	}
	defer psqlStorage.Close()

	// err = NewUpdateHandler(bot, psqlStorage, assets).HandleUpdates(ctx)
	err = NewUpdateHandler(nil, psqlStorage, assets).OneTimeMigration(ctx, *dumpDirPath)
	if err != nil {
		slog.ErrorContext(ctx, "unable to handle updates", slog.String("err", err.Error()))
	}

	cancel()
}
