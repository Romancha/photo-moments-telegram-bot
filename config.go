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
var keyCronSpec = "FM_SEND_PHOTO_CRON_SPEC"
var keyDebug = "FM_DEBUG"

type Config struct {
	chatId         int64
	allowedUserIds []int64
	botToken       string
	photoCount     int
	photoPath      string
	cronSpec       string
	debug          bool
}

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

	cronSpec := "0 10 * * *"
	overrideCronSpec := os.Getenv(keyCronSpec)
	if overrideCronSpec != "" {
		cronSpec = overrideCronSpec
	}

	debug := false
	debugEnv, err := strconv.ParseBool(os.Getenv(keyDebug))
	if err == nil {
		debug = debugEnv
	}

	return Config{
		chatId:         int64(chatId),
		allowedUserIds: allowedUserIds,
		botToken:       os.Getenv(keyBotToken),
		photoCount:     parsedCount,
		photoPath:      photoLibPath,
		cronSpec:       cronSpec,
		debug:          debug,
	}
}
