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

	bot.Send(tgbotapi.NewMessage(chatId, "📷 Sending random photos..."))

	// 1) Get the next sending number
	sendingNumber := getNextSendingNumber()

	// 2) Gather photos
	randomPhotoPaths := getRandomPhotos(count)
	var mediaGroup []interface{}

	// We'll store the "originalPaths" in the DB too
	var photoRecords []PhotoRecord
	for i, path := range randomPhotoPaths {
		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(path))

		// set caption only for the first photo
		if i == 0 {
			caption := "#" + strconv.Itoa(sendingNumber)
			photo.Caption = caption
		}

		mediaGroup = append(mediaGroup, photo)
	}

	mediaMsg := tgbotapi.NewMediaGroup(chatId, mediaGroup)
	if replyMessageId != nil {
		mediaMsg.ReplyParameters.MessageID = *replyMessageId
	}

	sentMessages, err := bot.SendMediaGroup(mediaMsg)
	if err != nil {
		log.Println(err)
		return
	}

	// 3) Store each photo’s message ID individually
	//    and also build the "PhotoSending" record
	for i, msg := range sentMessages {
		originalPath := lastPhotos[i]

		// We'll store a single record:
		// photoMsgID -> (sendingNumber, i+1, path)
		meta := PhotoMessageMeta{
			SendingNumber: sendingNumber,
			PhotoIndex:    i + 1,
			PhotoPath:     originalPath,
		}
		err := storePhotoMsgMeta(msg.MessageID, meta)
		if err != nil {
			log.Println("failed to store photo meta", err)
		}

		// Also populate array for group-level record
		photoRecords = append(photoRecords, PhotoRecord{
			Number: i + 1,
			Path:   originalPath,
		})
	}

	// 4) Finally, store group-level sending record
	ps := PhotoSending{
		NumberOfSending: sendingNumber,
		MessageId:       sentMessages[0].MessageID,
		Photos:          photoRecords,
	}
	storeSending(ps)
}

func sendSafeReplyText(chatId int64, replyMessageId int, bot *tgbotapi.BotAPI, text string) {
	msg := tgbotapi.NewMessage(chatId, text)
	msg.ReplyParameters.MessageID = replyMessageId

	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}
