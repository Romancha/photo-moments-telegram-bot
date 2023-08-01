package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
	"log"
	"os"
	"strconv"
)

var tempProcessedPhotoPath = os.TempDir() + "/" + "compressed"

var cfg = getConfig()

var lastPhotos []string

func main() {
	log.Println("Starting bot with config:", cfg)

	// Create the folder if it doesn't exist
	if _, err := os.Stat(tempProcessedPhotoPath); os.IsNotExist(err) {
		err := os.MkdirAll(tempProcessedPhotoPath, os.ModePerm)
		if err != nil {
			log.Panic(err)
		}
	}

	bot, err := tgbotapi.NewBotAPI(cfg.botToken)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = false

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	msg := tgbotapi.NewMessage(cfg.chatId, startMessage)
	if _, err := bot.Send(msg); err != nil {
		log.Println("Failed to send start message.", err)
	}

	c := cron.New()
	_, err = c.AddFunc(cfg.cronSpec, func() {
		sendRandomPhoto(-1, nil, bot)
	})
	if err != nil {
		panic("Failed to add cron job.")
	}
	c.Start()

	for update := range updates {
		if update.Message != nil { // If we got a message
			log.Printf("New message: [%s]: %d - %s", update.Message.From.UserName, update.Message.From.ID,
				update.Message.Text)

			// Reply info about bot to all users
			if update.Message.IsCommand() && update.Message.Command() == "start" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, startMessage)
				if _, err := bot.Send(msg); err != nil {
					log.Println("Failed send msg.", err)
				}

				continue
			}

			// Check if the user is allowed to use this bot
			if containsInt(cfg.allowedUserIds, update.Message.From.ID) == false {
				log.Printf("User %s: %d is not allowed to use this bot", update.Message.From.UserName,
					update.Message.From.ID)
				continue
			}

			// If the user send a command, send the information about the bot
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "photo":
					userPhotoCount, parseUserCountErr := strconv.Atoi(update.Message.CommandArguments())
					if parseUserCountErr != nil {
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
							"Please send a number")
						continue
					}

					sendRandomPhoto(userPhotoCount, &update, bot)
				case "paths":
					if len(lastPhotos) == 0 {
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot, "No photos sent yet")
						continue
					}

					sendLastPhotosPathMessage(update.Message.Chat.ID, update.Message.MessageID, bot)
				case "info":
					photoNumber, parsePhotoNumberErr := strconv.Atoi(update.Message.CommandArguments())
					if parsePhotoNumberErr != nil {
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
							"Please send a number")
						continue
					}

					if len(lastPhotos) == 0 {
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot, "No photos sent yet")
						continue
					}

					if photoNumber < 1 || photoNumber > len(lastPhotos) {
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
							"Please send a number in range of last sent photos")
						continue
					}

					sendPhotoDescriptionMessage(update.Message.Chat.ID, update.Message.MessageID, bot,
						lastPhotos[photoNumber-1])
				default:
					continue
				}
			}

			// If user send a number, send that many random photos
			userPhotoCount, parseUserCountErr := strconv.Atoi(update.Message.Text)
			if parseUserCountErr == nil {
				sendRandomPhoto(userPhotoCount, &update, bot)
			}
		}
	}
}

func sendRandomPhoto(count int, update *tgbotapi.Update, bot *tgbotapi.BotAPI) {
	sendRandomPhotoMessage(count, update, bot)
	clearCompressedPhotos()
}

func getRandomPhotoMedia(count int) []interface{} {
	if count <= 0 {
		count = cfg.photoCount
	}

	randomPhotos := getRandomPhotos(count)
	var randomMedia []interface{}

	for _, photo := range randomPhotos {
		randomMedia = append(randomMedia, tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(photo)))
	}

	return randomMedia
}
