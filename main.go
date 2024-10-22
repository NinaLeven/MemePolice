package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"github.com/NinaLeven/MemePolice/videohash"

	"github.com/corona10/goimagehash"

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

	assetsDirPath := flag.String("a", "assets", "path to assets directory")
	migrationsDirPath := flag.String("m", "migrations", "path to migrations directory")
	dumpDirPath := flag.String("d", "", "path to dump directory")
	postgresURL := flag.String("p", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable", "postgres url")
	flag.Parse()

	bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
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

	if *dumpDirPath == "" {
		err := updateHandler.HandleUpdates(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "unable to handle updates", slog.String("err", err.Error()))
		}
	} else {
		err := updateHandler.OneTimeMigration(ctx, *dumpDirPath)
		if err != nil {
			slog.ErrorContext(ctx, "unable to do one time migratio", slog.String("err", err.Error()))
		}
	}

	cancel()
}

func main1() {
	if len(os.Args) < 2 {
		log.Fatalln("provide video path")
	}

	vh1, ah1, err := videohash.PerceptualHash(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}

	vh2, ah2, err := videohash.PerceptualHash(os.Args[2])
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("video: ", hashDistance(vh1, vh2))
	fmt.Println("audio: ", hashDistance(ah1, ah2))
	fmt.Println("combined: ", hashDistance(vh1^ah1, vh2^ah2))
}

func hashDistance(a, b uint64) int {
	res, _ := goimagehash.NewImageHash(a, goimagehash.PHash).Distance(goimagehash.NewImageHash(b, goimagehash.PHash))
	return res
}
