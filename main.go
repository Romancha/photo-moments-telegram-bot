package main

import (
	"fmt"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
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

	// 1) Ensure the compressed photos folder
	if _, err := os.Stat(tempProcessedPhotoPath); os.IsNotExist(err) {
		err := os.MkdirAll(tempProcessedPhotoPath, os.ModePerm)
		if err != nil {
			log.Panic(err)
		}
	}

	// 2) Initialize bbolt DB
	initDB(cfg.dbPath)
	defer db.Close()

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
		if update.Message != nil {
			log.Printf("New message: [%s]: %d - %s", update.Message.From.UserName, update.Message.From.ID,
				update.Message.Text)

			if update.Message.IsCommand() && update.Message.Command() == "start" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, startMessage)
				if _, err := bot.Send(msg); err != nil {
					log.Println("Failed send msg.", err)
				}
				continue
			}

			// Check user permission
			if !containsInt(cfg.allowedUserIds, update.Message.From.ID) {
				log.Printf("User %s: %d is not allowed", update.Message.From.UserName, update.Message.From.ID)
				continue
			}

			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "photo":
					userPhotoCount, parseUserCountErr := strconv.Atoi(update.Message.CommandArguments())
					if parseUserCountErr != nil {
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
							"Please send a number")
						continue
					}
					if userPhotoCount < 1 {
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
							"Please send a number greater than 0")
						continue
					}
					sendRandomPhoto(userPhotoCount, &update, bot)

				case "info":
					infoArg := update.Message.CommandArguments()

					// Case 1: If user replies to a specific photo message with /info
					//         and provides no numeric argument => show that photo's info.
					if update.Message.ReplyToMessage != nil && (infoArg == "" || infoArg == " ") {
						handleReplyToPhotoInfo(update, bot)
						break
					}

					// Case 2: If user just writes "/info 2" (no reply),
					//         we interpret that as "the 2nd photo from the last sending."
					if infoArg != "" {
						photoIndex, err := strconv.Atoi(infoArg)
						if err != nil {
							sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
								"Please provide a valid number, e.g. /info 2.")
							break
						}
						handleLastSendingInfo(update, photoIndex, bot)
						break
					}

					// If user typed "/info" with no reply, no argument
					sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
						"Please specify a photo number or reply to a specific photo with /info.")

				default:
					continue
				}
			}

			// Also handle the situation if user just types a number (if cfg.sendPhotosByNumber = true)
			if cfg.sendPhotosByNumber {
				userPhotoCount, parseUserCountErr := strconv.Atoi(update.Message.Text)
				if parseUserCountErr != nil {
					continue
				}
				if userPhotoCount < 1 {
					sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
						"Please send a number greater than 0")
					continue
				}

				sendRandomPhoto(userPhotoCount, &update, bot)
			}
		}
	}
}

func sendRandomPhoto(count int, update *tgbotapi.Update, bot *tgbotapi.BotAPI) {
	sendRandomPhotoMessage(count, update, bot)
	clearCompressedPhotos()
}

func handleReplyToPhotoInfo(update tgbotapi.Update, bot *tgbotapi.BotAPI) {
	// The message we are replying to is a single photo in the group
	repliedMsgID := update.Message.ReplyToMessage.MessageID

	meta, err := getPhotoMsgMetaById(repliedMsgID)
	if err != nil {
		sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
			"No info found for this photo. Possibly not from the last group or DB error.")
		return
	}

	// We found it: meta.PhotoPath
	// Show a short header
	text := fmt.Sprintf("Sending #%d, photo #%d:", meta.SendingNumber, meta.PhotoIndex)
	sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot, text)

	// Reuse your existing EXIF logic
	sendPhotoDescriptionMessage(update.Message.Chat.ID, update.Message.MessageID, bot, meta.PhotoPath)
}

func handleLastSendingInfo(update tgbotapi.Update, photoIndex int, bot *tgbotapi.BotAPI) {
	// 1) Get the highest sendingNumber
	lastNumber, err := getLastSendingNumber()
	if err != nil {
		sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
			"No sendings found in DB.")
		return
	}

	// 2) Get the PhotoSending record
	ps, err := getSendingByNumber(lastNumber)
	if err != nil {
		sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
			"Could not load record for last sending.")
		return
	}

	// 3) Validate that photoIndex is in range
	if photoIndex < 1 || photoIndex > len(ps.Photos) {
		sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
			fmt.Sprintf("Invalid photo number. Last sending (#%d) had %d photos.",
				lastNumber, len(ps.Photos)))
		return
	}

	// 4) Show info
	p := ps.Photos[photoIndex-1]
	header := fmt.Sprintf("Photo #%d from sending #%d:", photoIndex, lastNumber)
	sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot, header)

	sendPhotoDescriptionMessage(update.Message.Chat.ID, update.Message.MessageID, bot, p.Path)
}
