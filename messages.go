package main

import (
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/h2non/bimg"
	"log"
	"strconv"
	"strings"
)

var startMessage = " 🖖 Hey! This is a Photo Moments bot, I can send random photos 📷 from your library. " +
	"\nRejoice you moments! ⭐" +
	"\n" +
	"\nIm a open-source project, you can find me on https://github.com/Romancha/photo-moments-telegram-bot"

func sendPhotoDescriptionMessage(chatId int64, messageId int, bot *tgbotapi.BotAPI, photoPath string) {
	photoExif := getPhotoExif(photoPath)
	if photoExif == nil {
		log.Println("photoExif is nil, path:", photoPath)
		return
	}

	msg := "Photo description\n"
	msg += "📂 " + photoPath + "\n"

	var photoCamera string
	if len(strings.TrimSpace(photoExif.Make)) > 0 {
		photoCamera = photoExif.Make
	}
	if len(strings.TrimSpace(photoExif.Model)) > 0 {
		photoCamera += " " + photoExif.Model
	}
	if len(photoCamera) > 0 {
		msg += "📷 " + photoCamera + "\n"
	}

	if len(strings.TrimSpace(photoExif.DateTimeOriginal)) > 0 {
		msg += "📅 " + photoExif.DateTimeOriginal + "\n"
	}

	sendSafeReplyText(chatId, messageId, bot, msg)

	latitude, err := convertGPSCoordinatesToFloat(photoExif.GPSLatitude)
	if err != nil {
		log.Println(err)
		return
	}

	longitude, err := convertGPSCoordinatesToFloat(photoExif.GPSLongitude)
	if err != nil {
		log.Println(err)
		return
	}

	locationMsg := tgbotapi.NewLocation(chatId, latitude, longitude)

	_, err = bot.Send(locationMsg)
	if err != nil {
		log.Println(err)
	}
}

func getPhotoExif(photoPath string) *bimg.EXIF {
	image, err := bimg.Read(photoPath)
	if err != nil {
		log.Println(err)
		return nil
	}

	imageMetadata, err := bimg.Metadata(image)
	if err != nil {
		log.Println(err)
		return nil
	}

	return &imageMetadata.EXIF
}

func sendRandomPhotoMessage(count int, update *tgbotapi.Update, bot *tgbotapi.BotAPI) {
	var chatId int64
	var replyMessageId *int
	if update != nil {
		chatId = update.Message.Chat.ID
		replyMessageId = &update.Message.MessageID
	} else {
		chatId = cfg.chatId
	}

	var _, _ = bot.Send(tgbotapi.NewMessage(chatId, "📷 Sending random photos..."))

	photoMedia := getRandomPhotoMedia(count)
	mediaMsg := tgbotapi.NewMediaGroup(chatId, photoMedia)
	if replyMessageId != nil {
		mediaMsg.ReplyParameters.MessageID = *replyMessageId
	}

	_, err := bot.Send(mediaMsg)
	if err != nil {
		log.Println(err)
	}
}

func sendLastPhotosPathMessage(chatId int64, messageId int, bot *tgbotapi.BotAPI) {
	var lastPhotosInfo string

	for index, photo := range lastPhotos {
		lastPhotosInfo += strconv.Itoa(index+1) + ". " + photo + "\n"
	}

	sendSafeReplyText(chatId, messageId, bot, lastPhotosInfo)
}

func sendSafeReplyText(chatId int64, replyMessageId int, bot *tgbotapi.BotAPI, text string) {
	msg := tgbotapi.NewMessage(chatId, text)
	msg.ReplyParameters.MessageID = replyMessageId

	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}
