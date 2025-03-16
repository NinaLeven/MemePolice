package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	tg "github.com/OvyFlash/telegram-bot-api"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("running memepolice")

	defer func() {
		slog.Info("exiting")
	}()
	defer func() {
		val := recover()
		if val != nil {
			slog.Error("panic", slog.Any("val", val))
		}
	}()

	ctx := context.Background()

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancel()

	assetsDirPath := flag.String("a", "assets", "path to assets directory")
	migrationsDirPath := flag.String("m", "migrations", "path to migrations directory")
	dumpDirPath := flag.String("d", "", "path to dump directory")
	postgresURL := flag.String("p", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable", "postgres url")
	liveChat := flag.Bool("c", false, "enable live memalnya chat")

	flag.Parse()

	bot, err := tg.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Panic(fmt.Errorf("unable to create bot: %w", err))
	}
	defer bot.StopReceivingUpdates()

	assets, err := NewFileAssets(*assetsDirPath)
	if err != nil {
		log.Panic(err)
	}

	psqlStorage, err := NewPSQLStorageManager(ctx, *postgresURL, *migrationsDirPath)
	if err != nil {
		log.Panic(err)
	}
	defer psqlStorage.Close()

	updateHandler := NewUpdateHandler(bot, psqlStorage, assets)

	if *liveChat {
		go updateHandler.LiveChat(ctx, MemalnyaChatID)
	}

	if *dumpDirPath == "" {
		err := updateHandler.HandleUpdates(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "unable to handle updates", slog.String("err", err.Error()))
		}
	} else {
		err := updateHandler.OneTimeMigration(ctx, *dumpDirPath, MemalnyaChatID)
		if err != nil {
			slog.ErrorContext(ctx, "unable to do one time migratio", slog.String("err", err.Error()))
		}
	}

	cancel()
	time.Sleep(time.Second * 2)
}
