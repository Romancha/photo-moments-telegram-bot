package main

import (
	"log"
	"os"
	"strconv"
	"strings"
)

var keyChatId = "FM_CHAT_ID"
var keyAllowedUsers = "FM_ALLOWED_USERS_ID"
var keyBotToken = "FM_TG_BOT_TOKEN"
var keyPhotoCount = "FM_PHOTO_COUNT"
var keyPhotoPath = "FM_PHOTO_PATH"
var keyDbPath = "FM_DB_PATH"
var keyCronSpec = "FM_SEND_PHOTO_CRON_SPEC"
var keySendPhotosByNumber = "FM_SEND_PHOTOS_BY_NUMBER"
var keyDebug = "FM_DEBUG"

type Config struct {
	chatId             int64
	allowedUserIds     []int64
	botToken           string
	photoCount         int
	photoPath          string
	dbPath             string
	cronSpec           string
	sendPhotosByNumber bool
	debug              bool
}

// TODO: rewrite configs with go-flags
func getConfig() Config {

	chatIdEnv := os.Getenv(keyChatId)
	chatId, err := strconv.Atoi(chatIdEnv)
	if err != nil {
		log.Panic("Failed to parse chat id")
	}

	allowedUsersEnv := os.Getenv(keyAllowedUsers)
	allowedUserIds := make([]int64, 0)
	for _, allowedUser := range strings.Split(allowedUsersEnv, ";") {
		if allowedUser == "" {
			continue
		}
		allowedId, err := strconv.ParseInt(allowedUser, 10, 64)
		if err != nil {
			log.Panic("Failed to parse allowed user ids")
		}
		allowedUserIds = append(allowedUserIds, allowedId)
	}

	var photoCount = os.Getenv(keyPhotoCount)
	parsedCount, convErr := strconv.Atoi(photoCount)
	if convErr != nil {
		log.Println(convErr)
		//if failed to parse, set default value
		parsedCount = 5
	}

	photoLibPath := "/photoLibrary"
	var overridePath = os.Getenv(keyPhotoPath)
	if overridePath != "" {
		photoLibPath = overridePath
	}

	dbPath := "photo_moments.db"
	overrideDbPath := os.Getenv(keyDbPath)
	if overrideDbPath != "" {
		dbPath = overrideDbPath
	}

	cronSpec := "0 10 * * *"
	overrideCronSpec := os.Getenv(keyCronSpec)
	if overrideCronSpec != "" {
		cronSpec = overrideCronSpec
	}

	sendPhotosByNumber := true
	overrideSendPhotosByNumber, err := strconv.ParseBool(os.Getenv(keySendPhotosByNumber))
	if err == nil {
		sendPhotosByNumber = overrideSendPhotosByNumber
	}

	debug := false
	debugEnv, err := strconv.ParseBool(os.Getenv(keyDebug))
	if err == nil {
		debug = debugEnv
	}

	return Config{
		chatId:             int64(chatId),
		allowedUserIds:     allowedUserIds,
		botToken:           os.Getenv(keyBotToken),
		photoCount:         parsedCount,
		photoPath:          photoLibPath,
		dbPath:             dbPath,
		cronSpec:           cronSpec,
		sendPhotosByNumber: sendPhotosByNumber,
		debug:              debug,
	}
}
