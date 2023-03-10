package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
	"log"
	"os"
	"strconv"
)

var compressedPhotoPath = os.TempDir() + "/" + "compressed"

var cfg = getConfig()

func main() {
	log.Println("Starting bot with config:", cfg)

	// Create the folder if it doesn't exist
	if _, err := os.Stat(compressedPhotoPath); os.IsNotExist(err) {
		err := os.MkdirAll(compressedPhotoPath, os.ModePerm)
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
	c.AddFunc(cfg.cronSpec, func() {
		sendRandomPhoto(-1, nil, bot)
	})
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
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please send a number")
						msg.ReplyToMessageID = update.Message.MessageID
						_, err := bot.Send(msg)
						if err != nil {
							log.Println(err)
						}
					} else {
						sendRandomPhoto(userPhotoCount, &update, bot)
					}
					continue
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
	var chatId int64
	var replyMessageId *int
	if update != nil {
		chatId = update.Message.Chat.ID
		replyMessageId = &update.Message.MessageID
	} else {
		chatId = cfg.chatId
	}

	var _, _ = bot.Send(tgbotapi.NewMessage(chatId, "???? Sending random photos..."))

	photoMedia := getRandomPhotoMedia(count)
	mediaMsg := tgbotapi.NewMediaGroup(chatId, photoMedia)
	if replyMessageId != nil {
		mediaMsg.ReplyToMessageID = *replyMessageId
	}

	_, err := bot.Send(mediaMsg)
	if err != nil {
		log.Println(err)
	}

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
