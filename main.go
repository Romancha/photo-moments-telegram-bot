package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/robfig/cron/v3"
	bolt "go.etcd.io/bbolt"
)

var tempProcessedPhotoPath = os.TempDir() + "/" + "compressed"
var cfg = getConfig()
var lastPhotos []string

type PhotoRequestType int

const (
	RequestTypeToday    PhotoRequestType = iota // Photos taken on this day in different years
	RequestTypeMemories                         // Photos taken a specific number of years ago
)

func main() {
	notSensitiveData := cfg
	notSensitiveData.botToken = "********"
	if notSensitiveData.telegramProxyPass != "" {
		notSensitiveData.telegramProxyPass = "********"
	}

	log.Println("Starting bot with config:", notSensitiveData)

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

	// 3) Initialize photo metadata buckets
	err := InitPhotoMetadata()
	if err != nil {
		log.Printf("Failed to initialize photo metadata: %v", err)
	} else {
		// Check and reset indexing flag if stuck
		err = ResetIndexingFlagIfStuck()
		if err != nil {
			log.Printf("Error checking indexing flag: %v", err)
		}

		// 4) Start background indexing with 2 workers
		StartBackgroundIndexing(cfg.photoPath, 2)
	}

	var bot *tgbotapi.BotAPI
	if cfg.telegramProxyURL != "" {
		proxyParsed, parseErr := url.Parse(cfg.telegramProxyURL)
		if parseErr != nil {
			log.Panicf("Invalid telegram proxy URL: %v", parseErr)
		}
		if cfg.telegramProxyUser != "" {
			proxyParsed.User = url.UserPassword(cfg.telegramProxyUser, cfg.telegramProxyPass)
		}
		httpClient := &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyParsed)},
		}
		bot, err = tgbotapi.NewBotAPIWithClient(cfg.botToken, tgbotapi.APIEndpoint, httpClient)
	} else {
		bot, err = tgbotapi.NewBotAPI(cfg.botToken)
	}
	if err != nil {
		log.Panic(err)
	}
	if cfg.telegramProxyURL != "" {
		safeURL, _ := url.Parse(cfg.telegramProxyURL)
		log.Printf("Using Telegram proxy: %s://%s", safeURL.Scheme, safeURL.Host)
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
		sendRandomPhoto(cfg.photoCount, nil, bot)
	})
	if err != nil {
		panic("Failed to add cron job.")
	}

	// Add cron job for sending photos from this day in different years
	_, err = c.AddFunc(cfg.memoriesCronSpec, func() {
		sendMemoryPhotos(RequestTypeToday, 0, nil, bot)
	})
	if err != nil {
		panic("Failed to add memories cron job.")
	}

	// Add cron job for automatic reindexing
	_, err = c.AddFunc(cfg.reindexCronSpec, func() {
		log.Println("Starting scheduled differential reindexing")
		err := StartDifferentialIndexing(cfg.photoPath, 2)
		if err != nil {
			log.Printf("Error during scheduled reindexing: %v", err)
		}
	})
	if err != nil {
		panic("Failed to add reindexing cron job.")
	}

	c.Start()

	// Set up commands for Telegram menu
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Start interaction with the bot"},
		{Command: "photo", Description: "Send random photos from your library"},
		{Command: "memories", Description: "Photos from this day 1 year ago (use /memories N for N years ago)"},
		{Command: "today", Description: "View photos taken on this day across different years"},
		{Command: "indexing", Description: "Show photo indexing status"},
		{Command: "reindex", Description: "Start photo reindexing (full/diff)"},
		{Command: "info", Description: "Show photo info (reply to photo or use /info N for Nth photo)"},
	}

	// Set bot commands
	commandConfig := tgbotapi.NewSetMyCommands(commands...)
	_, err = bot.Request(commandConfig)
	if err != nil {
		log.Printf("Error setting bot commands: %v", err)
	}

	for update := range updates {
		if update.Message != nil {
			log.Printf("New message: [%s]: %d - %s", update.Message.From.UserName, update.Message.From.ID,
				update.Message.Text)

			if update.Message.IsCommand() && update.Message.Command() == "start" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, startMessage)
				if _, err := sendMessageWithRetry(bot, msg); err != nil {
					log.Println("Failed send start msg after all retries:", err)
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

				case "memories":
					// Processing command to get photos from the past
					yearsArg := update.Message.CommandArguments()
					var yearsAgo = 1 // Default: 1 year ago
					var err error

					if yearsArg != "" {
						yearsAgo, err = strconv.Atoi(yearsArg)
						if err != nil || yearsAgo < 1 {
							sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
								"Please specify a valid number of years, for example: /memories 2")
							break
						}
					}

					sendMemoryPhotos(RequestTypeMemories, yearsAgo, &update, bot)

				case "today":
					// Processing command to get photos taken on this day in different years
					sendMemoryPhotos(RequestTypeToday, 0, &update, bot)

				case "indexing":
					// Show indexing status
					active, _, _, err := GetIndexingStatus()
					if err != nil {
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
							fmt.Sprintf("Error getting indexing status: %v", err))
						break
					}

					// Send initial status message
					statusMsgID, err := sendIndexingStatusMessage(update.Message.Chat.ID, update.Message.MessageID, bot)
					if err != nil {
						log.Printf("Error sending indexing status: %v", err)
						break
					}

					// If indexing is active, start a goroutine to update the status message
					if active {
						go func(chatID int64, messageID int) {
							ticker := time.NewTicker(3 * time.Second) // Update every 3 seconds
							defer ticker.Stop()

							// Add counter to track changes in indexing status
							lastIndexed := 0
							unchangedCount := 0
							maxUnchangedCount := 10 // Maximum number of updates without changes (30 seconds)

							// Keep updating the status message while indexing is active
							for range ticker.C {
								// Check if indexing is still active
								active, indexed, _, err := GetIndexingStatus()
								if err != nil {
									log.Printf("Error checking indexing status: %v", err)
									return
								}

								// Check if the number of indexed photos has changed
								if indexed == lastIndexed {
									unchangedCount++
								} else {
									unchangedCount = 0
									lastIndexed = indexed
								}

								// If status hasn't changed for too long, consider indexing as "stuck"
								if unchangedCount >= maxUnchangedCount {
									log.Printf("Indexing status hasn't changed for %d seconds, stopping updates",
										3*maxUnchangedCount)

									// Reset indexing flag
									err = db.Update(func(tx *bolt.Tx) error {
										b := tx.Bucket([]byte(bucketIndexingStats))
										if b == nil {
											return fmt.Errorf("bucket %s not found", bucketIndexingStats)
										}
										return b.Put([]byte(keyIndexingActive), []byte("false"))
									})

									if err != nil {
										log.Printf("Error resetting indexing flag: %v", err)
									} else {
										log.Println("Reset stuck indexing flag")
										active = false
									}
								}

								// Update the status message
								err = updateIndexingStatusMessage(chatID, messageID, bot)
								if err != nil {
									log.Printf("Error updating indexing status: %v", err)
									return
								}

								// If indexing is no longer active, update one last time and stop
								if !active {
									// Wait a moment for final stats to be updated
									time.Sleep(1 * time.Second)

									// Final update with completed status
									err = updateIndexingStatusMessage(chatID, messageID, bot)
									if err != nil {
										log.Printf("Error updating final indexing status: %v", err)
									}
									return
								}
							}
						}(update.Message.Chat.ID, statusMsgID)
					}

				case "reindex":
					// Start indexing with parameters
					args := strings.Fields(update.Message.Text)
					if len(args) < 2 {
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
							"Usage: /reindex [full|diff]\n"+
								"full - full reindexing (clear and recreate indexes)\n"+
								"diff - differential indexing (only new and modified files)")
						break
					}

					// Check if indexing is already active
					active, _, _, err := GetIndexingStatus()
					if err != nil {
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
							fmt.Sprintf("Error checking indexing status: %v", err))
						break
					}

					if active {
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
							"Indexing is already active, please wait for it to complete")
						break
					}

					// Determine indexing type
					indexType := strings.ToLower(args[1])
					var responseMsg string

					switch indexType {
					case "full":
						// Start full reindexing
						err = ForceReindexing(cfg.photoPath, 2)
						if err != nil {
							sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
								fmt.Sprintf("Error starting full reindexing: %v", err))
							break
						}
						responseMsg = "Full photo reindexing started"

					case "diff":
						// Start differential indexing
						err = StartDifferentialIndexing(cfg.photoPath, 2)
						if err != nil {
							sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
								fmt.Sprintf("Error starting differential indexing: %v", err))
							break
						}
						responseMsg = "Differential photo indexing started (only new and modified files)"

					default:
						sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot,
							"Unknown indexing type. Use 'full' or 'diff'")
						break
					}

					sendSafeReplyText(update.Message.Chat.ID, update.Message.MessageID, bot, responseMsg)

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

// sendMemoryPhotos sends photos from the past
// requestType - request type (today or specific number of years ago)
// yearsAgo - number of years ago (used only for RequestTypeMemories)
func sendMemoryPhotos(requestType PhotoRequestType, yearsAgo int, update *tgbotapi.Update, bot *tgbotapi.BotAPI) {
	var chatId int64
	var replyMessageId *int
	if update != nil {
		chatId = update.Message.Chat.ID
		replyMessageId = &update.Message.MessageID
	} else {
		chatId = cfg.chatId
		// Create a dummy message ID for reply
		fakeId := 0
		replyMessageId = &fakeId
	}

	// Form message depending on request type
	var searchMessage string
	if requestType == RequestTypeToday {
		searchMessage = "🗓 Looking for photos taken on this day in different years..."
	} else {
		searchMessage = fmt.Sprintf("🕰 Looking for photos taken %d years ago on this day...", yearsAgo)
	}
	sendSafeReplyText(chatId, *replyMessageId, bot, searchMessage)

	// Get photos depending on request type
	var photos []string
	var err error
	if requestType == RequestTypeToday {
		photos, err = GetPhotosFromThisDay(100) // Get more photos to have enough for selection
	} else {
		photos, err = GetPhotosFromPast(yearsAgo, 30) // Increase limit as we'll group by years
	}

	if err != nil {
		sendSafeReplyText(chatId, *replyMessageId, bot, fmt.Sprintf("Error searching for photos: %v", err))
		return
	}

	if len(photos) == 0 {
		var notFoundMessage string
		if requestType == RequestTypeToday {
			notFoundMessage = "No photos found taken on this day"
		} else {
			notFoundMessage = fmt.Sprintf("No photos found taken %d years ago on this day", yearsAgo)
		}
		sendSafeReplyText(chatId, *replyMessageId, bot, notFoundMessage)
		return
	}

	// Group photos by year
	photosByYear := make(map[int][]string)

	for _, photoPath := range photos {
		// Get photo metadata to determine the year
		var metadata *PhotoMetadata
		err := db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(bucketPhotoMetadata))
			if b == nil {
				return fmt.Errorf("bucket %s not found", bucketPhotoMetadata)
			}

			metadataBytes := b.Get([]byte(photoPath))
			if metadataBytes == nil {
				return fmt.Errorf("metadata not found for %s", photoPath)
			}

			var m PhotoMetadata
			err := json.Unmarshal(metadataBytes, &m)
			if err != nil {
				return fmt.Errorf("error unmarshaling metadata: %v", err)
			}

			metadata = &m
			return nil
		})

		if err != nil {
			log.Printf("Error getting metadata for %s: %v", photoPath, err)
			continue
		}

		// Add photo to the corresponding year group
		photosByYear[metadata.Year] = append(photosByYear[metadata.Year], photoPath)
	}

	// Sort years for sending from oldest to newest
	var years []int
	for year := range photosByYear {
		years = append(years, year)
	}
	sort.Ints(years)

	// Flag to track the first message (which will have sound)
	isFirstMessage := true

	// Calculate how many photos to take from each year
	// to not exceed the total limit
	photosPerYear := make(map[int]int)
	remainingPhotos := cfg.memoriesPhotoCount

	// First pass: ensure at least one photo per year if possible
	for _, year := range years {
		if len(photosByYear[year]) > 0 && remainingPhotos > 0 {
			photosPerYear[year] = 1
			remainingPhotos--
		}
	}

	// Second pass: distribute remaining photos proportionally
	if remainingPhotos > 0 && len(years) > 0 {
		// Calculate total available photos across all years
		totalAvailable := 0
		for _, year := range years {
			// Count available photos beyond the first one we already allocated
			if len(photosByYear[year]) > photosPerYear[year] {
				totalAvailable += len(photosByYear[year]) - photosPerYear[year]
			}
		}

		// Distribute remaining photos proportionally
		for _, year := range years {
			available := len(photosByYear[year]) - photosPerYear[year]
			if available <= 0 {
				continue
			}

			// Calculate fair share based on proportion of available photos
			var share int
			if totalAvailable > 0 {
				share = int(float64(available) / float64(totalAvailable) * float64(remainingPhotos))
			}

			// Ensure we don't take more than available
			if share > available {
				share = available
			}

			photosPerYear[year] += share
			remainingPhotos -= share

			// If we can't distribute proportionally, just break
			if remainingPhotos <= 0 {
				break
			}
		}
	}

	// Third pass: distribute any remaining photos
	if remainingPhotos > 0 {
		for _, year := range years {
			available := len(photosByYear[year]) - photosPerYear[year]
			if available <= 0 {
				continue
			}

			// Take one more photo from this year
			photosPerYear[year]++
			remainingPhotos--

			if remainingPhotos <= 0 {
				break
			}
		}
	}

	// Send photos by groups for each year
	for _, year := range years {
		yearPhotos := photosByYear[year]

		// Skip years with no allocated photos
		if photosPerYear[year] <= 0 {
			continue
		}

		// Limit the number of photos for this year based on our calculation
		if len(yearPhotos) > photosPerYear[year] {
			// First filter similar photos within each year
			filteredYearPhotos, err := FilterSimilarPhotos(yearPhotos)
			if err != nil {
				log.Printf("Error filtering similar photos for year %d: %v", year, err)
				filteredYearPhotos = yearPhotos // Fallback to original photos
			}

			// If we still have more photos than needed after filtering, shuffle and take the calculated number
			if len(filteredYearPhotos) > photosPerYear[year] {
				shuffleStrings(filteredYearPhotos)
				yearPhotos = filteredYearPhotos[:photosPerYear[year]]
			} else {
				yearPhotos = filteredYearPhotos
			}
		}

		// Process photos for this year
		var processedPhotos []string
		for _, photo := range yearPhotos {
			compressedPhoto := processPhoto(photo)
			if compressedPhoto != nil {
				processedPhotos = append(processedPhotos, *compressedPhoto)
			}
		}

		if len(processedPhotos) == 0 {
			continue // Skip if there are no processed photos for this year
		}

		// Send photos for this year
		var mediaGroup []interface{}
		var originalPhotos []string // Store original photo paths for this year

		// Ensure we don't exceed Telegram's limit of 10 photos per media group
		maxPhotosInGroup := 10
		if len(processedPhotos) > maxPhotosInGroup {
			log.Printf("Limiting photos for year %d from %d to %d due to Telegram API limitations",
				year, len(processedPhotos), maxPhotosInGroup)
			processedPhotos = processedPhotos[:maxPhotosInGroup]
			yearPhotos = yearPhotos[:maxPhotosInGroup]
		}

		for i, path := range processedPhotos {
			photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(path))

			// Set caption only for the first photo in the group
			if i == 0 {
				now := time.Now()
				var caption string
				if requestType == RequestTypeToday {
					caption = fmt.Sprintf("📅 Photos taken on %d.%d.%d", now.Day(), now.Month(), year)
				} else {
					pastDate := now.AddDate(-yearsAgo, 0, 0)
					caption = fmt.Sprintf("📅 %d years ago (%s) - year %d", yearsAgo, pastDate.Format("02.01.2006"), year)
				}
				photo.Caption = caption
			}

			mediaGroup = append(mediaGroup, photo)
			// Store original photo path for each processed photo
			originalPhotos = append(originalPhotos, yearPhotos[i])
		}

		mediaMsg := tgbotapi.NewMediaGroup(chatId, mediaGroup)
		mediaMsg.ReplyParameters.MessageID = *replyMessageId

		// Set DisableNotification flag for all messages except the first one
		if !isFirstMessage {
			mediaMsg.DisableNotification = true
		}
		isFirstMessage = false

		// Send the media group and store metadata for /info command
		sentMessages, err := sendMediaGroupWithRetry(bot, mediaMsg)
		if err != nil {
			log.Println("Failed to send memory photos after all retries:", err)
			sendSafeReplyText(chatId, *replyMessageId, bot, fmt.Sprintf("Error sending photos for year %d: %v", year, err))
			continue
		}

		// Get the next sending number for this group
		sendingNumber := getNextSendingNumber()

		// Store metadata for each photo in the group
		var photoRecords []PhotoRecord
		for i, msg := range sentMessages {
			if i >= len(originalPhotos) {
				log.Printf("Warning: sent message index %d exceeds original photos length %d", i, len(originalPhotos))
				continue
			}

			originalPath := originalPhotos[i]

			// Store individual photo metadata
			meta := PhotoMessageMeta{
				SendingNumber: sendingNumber,
				PhotoIndex:    i + 1,
				PhotoPath:     originalPath,
			}
			err := storePhotoMsgMeta(msg.MessageID, meta)
			if err != nil {
				log.Printf("Failed to store photo meta for year %d: %v", year, err)
			}

			// Add to photo records for group-level record
			photoRecords = append(photoRecords, PhotoRecord{
				Number: i + 1,
				Path:   originalPath,
			})
		}

		// Store group-level sending record
		if len(sentMessages) > 0 {
			ps := PhotoSending{
				NumberOfSending: sendingNumber,
				MessageId:       sentMessages[0].MessageID,
				Photos:          photoRecords,
			}
			storeSending(ps)
		}
	}

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
