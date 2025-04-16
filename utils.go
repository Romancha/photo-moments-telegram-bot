package main

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
)

func find(root string, ext []string) []string {
	log.Print("searching for photos in ", root)

	var a []string

	_ = filepath.WalkDir(root, func(s string, d fs.DirEntry, e error) error {
		//skip synology @eadir folder
		if d.IsDir() && d.Name() == "@eaDir" {
			return filepath.SkipDir
		}

		if e != nil {
			log.Panic(e)
			return nil
		}
		if !d.IsDir() && contains(ext, filepath.Ext(d.Name())) {
			a = append(a, s)
		}
		return nil
	})

	return a
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func containsInt(s []int64, e int64) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func convertGPSCoordinatesToFloat(coord string) (float64, error) {
	parts := strings.Split(coord, " ")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid GPS coordinate format: %s", coord)
	}

	degrees, err := parseFraction(parts[0])
	if err != nil {
		return 0, err
	}

	minutes, err := parseFraction(parts[1])
	if err != nil {
		return 0, err
	}

	seconds, err := parseFraction(parts[2])
	if err != nil {
		return 0, err
	}

	// Calculate the total decimal degrees
	decimalDegrees := degrees + minutes/60.0 + seconds/3600.0

	return decimalDegrees, nil
}

func parseFraction(fractionStr string) (float64, error) {
	parts := strings.Split(fractionStr, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid fraction format: %s", fractionStr)
	}

	numerator, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}

	denominator, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, err
	}

	return numerator / denominator, nil
}

// formatDuration formats duration in seconds to a readable form
func formatDuration(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60

	if hours > 0 {
		return fmt.Sprintf("%d h %d min %d sec", hours, minutes, secs)
	} else if minutes > 0 {
		return fmt.Sprintf("%d min %d sec", minutes, secs)
	}
	return fmt.Sprintf("%d sec", secs)
}

// sendMediaGroupWithRetry attempts to send a media group with retries
// in case of network or API errors
func sendMediaGroupWithRetry(bot *tgbotapi.BotAPI, mediaMsg tgbotapi.MediaGroupConfig) ([]tgbotapi.Message, error) {
	maxRetries := 5
	initialRetryDelay := 2 * time.Second

	var sentMessages []tgbotapi.Message
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		sentMessages, err = bot.SendMediaGroup(mediaMsg)
		if err == nil {
			return sentMessages, nil
		}

		// Log the error
		log.Printf("Error sending media group (attempt %d/%d): %v",
			attempt+1, maxRetries, err)

		// If this was the last attempt, return the error
		if attempt >= maxRetries-1 {
			return nil, fmt.Errorf("failed to send media group after %d attempts: %w",
				maxRetries, err)
		}

		// Calculate exponential backoff delay: 2s, 4s, 8s, 16s...
		retryDelay := initialRetryDelay * time.Duration(1<<attempt)
		log.Printf("Retrying in %s...", retryDelay)
		time.Sleep(retryDelay)
	}

	return nil, err // This should never happen due to the return in the loop
}

// sendMessageWithRetry attempts to send a message with retries
// in case of network or API errors
func sendMessageWithRetry(bot *tgbotapi.BotAPI, msg tgbotapi.Chattable) (tgbotapi.Message, error) {
	maxRetries := 5
	initialRetryDelay := 2 * time.Second

	var sentMessage tgbotapi.Message
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		sentMessage, err = bot.Send(msg)
		if err == nil {
			return sentMessage, nil
		}

		// Log the error
		log.Printf("Error sending message (attempt %d/%d): %v",
			attempt+1, maxRetries, err)

		// If this was the last attempt, return the error
		if attempt >= maxRetries-1 {
			return tgbotapi.Message{}, fmt.Errorf("failed to send message after %d attempts: %w",
				maxRetries, err)
		}

		// Calculate exponential backoff delay: 2s, 4s, 8s, 16s...
		retryDelay := initialRetryDelay * time.Duration(1<<attempt)
		log.Printf("Retrying in %s...", retryDelay)
		time.Sleep(retryDelay)
	}

	return tgbotapi.Message{}, err // This should never happen due to the return in the loop
}
