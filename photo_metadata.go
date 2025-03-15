package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

// PhotoMetadata structure for storing photo metadata
type PhotoMetadata struct {
	Path         string    `json:"path"`
	TakenDate    time.Time `json:"takenDate"`
	Month        int       `json:"month"`
	Day          int       `json:"day"`
	Year         int       `json:"year"`
	CameraModel  string    `json:"cameraModel"`
	GpsLat       float64   `json:"gpsLat"`
	GpsLon       float64   `json:"gpsLon"`
	IndexedAt    time.Time `json:"indexedAt"`
	ModifiedTime time.Time `json:"modifiedTime"` // File last modification time
	FileSize     int64     `json:"fileSize"`     // File size
	FileHash     string    `json:"fileHash"`     // MD5 file hash (optional)
}

const (
	bucketPhotoMetadata    = "PhotoMetadata"     // Bucket for storing photo metadata
	bucketDateIndex        = "DateIndex"         // Bucket for date index (month-day -> list of paths)
	bucketYearDateIndex    = "YearDateIndex"     // Bucket for year and date index (year-month-day -> list of paths)
	bucketIndexingStats    = "IndexingStats"     // Bucket for indexing statistics
	keyIndexingActive      = "IndexingActive"    // Key for indexing activity flag
	keyIndexedCount        = "IndexedCount"      // Key for indexed photos counter
	keyTotalCount          = "TotalCount"        // Key for total photos count
	keyLastIndexedTime     = "LastIndexedTime"   // Key for last indexing time
	keyAllIndexedFiles     = "AllIndexedFiles"   // Key for list of all indexed files
	keyCalculateFileHashes = "CalculateHashes"   // Key for file hash calculation flag
	keyIndexingStartTime   = "IndexingStartTime" // Key for indexing start time
	keyIndexingDuration    = "IndexingDuration"  // Key for indexing duration (in seconds)
)

var (
	indexingMutex sync.Mutex
)

// InitPhotoMetadata initializes buckets for photo metadata
func InitPhotoMetadata() error {
	return db.Update(func(tx *bolt.Tx) error {
		// Create buckets if they don't exist
		for _, bucketName := range []string{bucketPhotoMetadata, bucketDateIndex, bucketYearDateIndex, bucketIndexingStats} {
			_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
			if err != nil {
				return fmt.Errorf("cannot create bucket %s: %v", bucketName, err)
			}
		}

		// Set default value for hash calculation flag
		b := tx.Bucket([]byte(bucketIndexingStats))
		if b != nil {
			if b.Get([]byte(keyCalculateFileHashes)) == nil {
				err := b.Put([]byte(keyCalculateFileHashes), []byte("false"))
				if err != nil {
					return fmt.Errorf("cannot set default value for calculate hashes flag: %v", err)
				}
			}
		}

		return nil
	})
}

// StartBackgroundIndexing starts background photo indexing process
func StartBackgroundIndexing(photoPath string, workerCount int) {
	startIndexingProcess(photoPath, workerCount, false, false)
}

// ForceReindexing starts forced full photo reindexing
func ForceReindexing(photoPath string, workerCount int) error {
	// Check if indexing is already active
	active, _, _, err := GetIndexingStatus()
	if err != nil {
		return fmt.Errorf("error checking indexing status: %v", err)
	}

	if active {
		return fmt.Errorf("indexing is already active, please wait for it to complete")
	}

	// Clear existing indices
	err = clearAllIndices()
	if err != nil {
		return fmt.Errorf("error clearing indices: %v", err)
	}

	// Start indexing process
	startIndexingProcess(photoPath, workerCount, true, false)

	return nil
}

// StartDifferentialIndexing starts differential indexing (only new and modified files)
func StartDifferentialIndexing(photoPath string, workerCount int) error {
	// Check if indexing is already active
	active, _, _, err := GetIndexingStatus()
	if err != nil {
		return fmt.Errorf("error checking indexing status: %v", err)
	}

	if active {
		return fmt.Errorf("indexing is already active, please wait for it to complete")
	}

	// Start indexing process
	startIndexingProcess(photoPath, workerCount, false, true)

	return nil
}

// EnableFileHashing enables file hash calculation during indexing
func EnableFileHashing(enable bool) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketIndexingStats))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketIndexingStats)
		}

		value := "false"
		if enable {
			value = "true"
		}

		return b.Put([]byte(keyCalculateFileHashes), []byte(value))
	})
}

// IsFileHashingEnabled checks if file hash calculation is enabled
func IsFileHashingEnabled() (bool, error) {
	var enabled bool

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketIndexingStats))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketIndexingStats)
		}

		enabledBytes := b.Get([]byte(keyCalculateFileHashes))
		if enabledBytes != nil {
			enabled = string(enabledBytes) == "true"
		}

		return nil
	})

	if err != nil {
		return false, err
	}

	return enabled, nil
}

// clearAllIndices clears all indices
func clearAllIndices() error {
	return db.Update(func(tx *bolt.Tx) error {
		// Delete and recreate buckets
		for _, bucketName := range []string{bucketPhotoMetadata, bucketDateIndex, bucketYearDateIndex} {
			err := tx.DeleteBucket([]byte(bucketName))
			if err != nil && err != bolt.ErrBucketNotFound {
				return fmt.Errorf("error deleting bucket %s: %v", bucketName, err)
			}

			_, err = tx.CreateBucket([]byte(bucketName))
			if err != nil {
				return fmt.Errorf("error creating bucket %s: %v", bucketName, err)
			}
		}

		// Reset indexing statistics
		b := tx.Bucket([]byte(bucketIndexingStats))
		if b != nil {
			err := b.Put([]byte(keyIndexedCount), []byte("0"))
			if err != nil {
				return fmt.Errorf("error resetting indexing counter: %v", err)
			}

			// Clear list of all indexed files
			err = b.Put([]byte(keyAllIndexedFiles), []byte("[]"))
			if err != nil {
				return fmt.Errorf("error clearing list of indexed files: %v", err)
			}
		}

		return nil
	})
}

// startIndexingProcess starts the indexing process
func startIndexingProcess(photoPath string, workerCount int, forceAll bool, cleanupDeleted bool) {
	indexingMutex.Lock()

	// Check if indexing is already active
	var indexingActive bool
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketIndexingStats))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketIndexingStats)
		}

		activeBytes := b.Get([]byte(keyIndexingActive))
		if activeBytes != nil {
			indexingActive = string(activeBytes) == "true"
		}

		return nil
	})

	if err != nil {
		log.Printf("Error checking indexing status: %v", err)
		indexingMutex.Unlock()
		return
	}

	if indexingActive {
		log.Println("Indexing is already active")
		indexingMutex.Unlock()
		return
	}

	// Record indexing start time
	startTime := time.Now()

	// Set indexing active flag and save start time
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketIndexingStats))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketIndexingStats)
		}

		// Save indexing start time
		err := b.Put([]byte(keyIndexingStartTime), []byte(startTime.Format(time.RFC3339)))
		if err != nil {
			return err
		}

		return b.Put([]byte(keyIndexingActive), []byte("true"))
	})

	if err != nil {
		log.Printf("Error setting indexing active flag: %v", err)
		indexingMutex.Unlock()
		return
	}

	indexingMutex.Unlock()

	go func() {
		defer func() {
			// Reset indexing active flag when completed
			indexingMutex.Lock()
			err := db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte(bucketIndexingStats))
				if b == nil {
					return fmt.Errorf("bucket %s not found", bucketIndexingStats)
				}

				// Update last indexing time
				err := b.Put([]byte(keyLastIndexedTime), []byte(time.Now().Format(time.RFC3339)))
				if err != nil {
					return err
				}

				// Save indexing duration in seconds
				duration := time.Since(startTime).Seconds()
				err = b.Put([]byte(keyIndexingDuration), []byte(fmt.Sprintf("%.2f", duration)))
				if err != nil {
					return err
				}

				return b.Put([]byte(keyIndexingActive), []byte("false"))
			})

			if err != nil {
				log.Printf("Error resetting indexing active flag: %v", err)
			}

			indexingMutex.Unlock()
		}()

		log.Println("Starting background indexing of photos")

		// Get list of all photos
		photos := find(photoPath, []string{".JPG", ".PNG", ".JPEG", ".jpg", ".png", ".jpeg", ".webp", ".WEBP", ".gif",
			".GIF", ".HEIC", ".heic"})

		log.Printf("Found %d photos to index", len(photos))

		// Get last indexing time
		var lastIndexedTime time.Time
		if !forceAll {
			lastIndexed, err := GetLastIndexedTime()
			if err == nil {
				lastIndexedTime = lastIndexed
			}
		}

		// Get file hash calculation flag
		calculateHashes, err := IsFileHashingEnabled()
		if err != nil {
			log.Printf("Error checking file hashing flag: %v", err)
			calculateHashes = false
		}

		// Save total photo count
		err = db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(bucketIndexingStats))
			if b == nil {
				return fmt.Errorf("bucket %s not found", bucketIndexingStats)
			}

			return b.Put([]byte(keyTotalCount), []byte(strconv.Itoa(len(photos))))
		})

		if err != nil {
			log.Printf("Error saving total photo count: %v", err)
		}

		// If need to clean up deleted files, create map of all current files
		var currentFiles map[string]bool
		if cleanupDeleted {
			currentFiles = make(map[string]bool, len(photos))
			for _, photo := range photos {
				currentFiles[photo] = true
			}
		}

		// Create channel for processing photos
		photoChan := make(chan string, workerCount)
		var wg sync.WaitGroup

		// Start workers for processing photos
		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for photoPath := range photoChan {
					// Check if this photo is already indexed in the database and if it needs to be reindexed
					var needsIndexing bool = true
					var existingMetadata *PhotoMetadata

					if !forceAll {
						err := db.View(func(tx *bolt.Tx) error {
							b := tx.Bucket([]byte(bucketPhotoMetadata))
							if b == nil {
								return fmt.Errorf("bucket %s not found", bucketPhotoMetadata)
							}

							metadataBytes := b.Get([]byte(photoPath))
							if metadataBytes != nil {
								// Photo is already indexed, check modification time
								var metadata PhotoMetadata
								err := json.Unmarshal(metadataBytes, &metadata)
								if err != nil {
									return fmt.Errorf("error unmarshaling metadata: %v", err)
								}

								existingMetadata = &metadata

								// Get file information
								fileInfo, err := os.Stat(photoPath)
								if err != nil {
									return fmt.Errorf("error getting file info: %v", err)
								}

								modTime := fileInfo.ModTime()
								fileSize := fileInfo.Size()

								// Check if file has changed since last indexing
								if modTime.After(lastIndexedTime) ||
									modTime.After(metadata.IndexedAt) ||
									fileSize != metadata.FileSize {
									// File changed, need to reindex
									needsIndexing = true
								} else {
									// File not changed, skip
									needsIndexing = false
								}
							}

							return nil
						})

						if err != nil {
							log.Printf("Error checking if photo needs indexing: %v", err)
							continue
						}
					}

					if !needsIndexing {
						// Photo not changed, skip
						continue
					}

					// Extract metadata
					metadata, err := extractPhotoMetadata(photoPath, calculateHashes)
					if err != nil {
						log.Printf("Error extracting metadata for %s: %v", photoPath, err)
						continue
					}

					// If there are existing metadata, save some fields
					if existingMetadata != nil {
						// Save file hash, if it was calculated before and not calculated now
						if !calculateHashes && existingMetadata.FileHash != "" {
							metadata.FileHash = existingMetadata.FileHash
						}
					}

					// Save metadata to database
					err = savePhotoMetadata(metadata)
					if err != nil {
						log.Printf("Error saving metadata for %s: %v", photoPath, err)
					}

					// Increment indexed photos counter
					err = db.Update(func(tx *bolt.Tx) error {
						b := tx.Bucket([]byte(bucketIndexingStats))
						if b == nil {
							return fmt.Errorf("bucket %s not found", bucketIndexingStats)
						}

						countBytes := b.Get([]byte(keyIndexedCount))
						var count int
						if countBytes != nil {
							count, _ = strconv.Atoi(string(countBytes))
						}

						count++
						return b.Put([]byte(keyIndexedCount), []byte(strconv.Itoa(count)))
					})

					if err != nil {
						log.Printf("Error updating indexed count: %v", err)
					}
				}
			}()
		}

		// Send photos to processing channel
		for _, photo := range photos {
			photoChan <- photo
		}

		// Close channel and wait for all workers to complete
		close(photoChan)
		wg.Wait()

		// If need to clean up deleted files
		if cleanupDeleted {
			log.Println("Cleaning up deleted files from index")
			cleanupDeletedFiles(currentFiles)
		}

		log.Println("Background indexing completed")
	}()
}

// cleanupDeletedFiles removes files from index that no longer exist on disk
func cleanupDeletedFiles(currentFiles map[string]bool) {
	// Get list of all indexed files
	var allIndexedFiles []string

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketPhotoMetadata))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketPhotoMetadata)
		}

		return b.ForEach(func(k, v []byte) error {
			filePath := string(k)
			allIndexedFiles = append(allIndexedFiles, filePath)
			return nil
		})
	})

	if err != nil {
		log.Printf("Error getting all indexed files: %v", err)
		return
	}

	// Check each file and remove those that no longer exist in the current list
	var deletedCount int
	for _, filePath := range allIndexedFiles {
		if !currentFiles[filePath] {
			// File was deleted, remove it from index
			err := removePhotoFromIndex(filePath)
			if err != nil {
				log.Printf("Error removing deleted file from index: %v", err)
			} else {
				deletedCount++
			}
		}
	}

	log.Printf("Removed %d deleted files from index", deletedCount)
}

// removePhotoFromIndex removes photo from all indices
func removePhotoFromIndex(photoPath string) error {
	return db.Update(func(tx *bolt.Tx) error {
		// Get photo metadata
		bMetadata := tx.Bucket([]byte(bucketPhotoMetadata))
		if bMetadata == nil {
			return fmt.Errorf("bucket %s not found", bucketPhotoMetadata)
		}

		metadataBytes := bMetadata.Get([]byte(photoPath))
		if metadataBytes == nil {
			// Photo not found in index
			return nil
		}

		var metadata PhotoMetadata
		err := json.Unmarshal(metadataBytes, &metadata)
		if err != nil {
			return fmt.Errorf("error unmarshaling metadata: %v", err)
		}

		// Remove from index by date (month-day)
		bDateIndex := tx.Bucket([]byte(bucketDateIndex))
		if bDateIndex != nil {
			dateKey := fmt.Sprintf("%02d-%02d", metadata.Month, metadata.Day)
			pathsData := bDateIndex.Get([]byte(dateKey))
			if pathsData != nil {
				var paths []string
				err = json.Unmarshal(pathsData, &paths)
				if err == nil {
					// Remove path from list
					var newPaths []string
					for _, path := range paths {
						if path != photoPath {
							newPaths = append(newPaths, path)
						}
					}

					// Save updated list
					if len(newPaths) > 0 {
						pathsData, err = json.Marshal(newPaths)
						if err == nil {
							err = bDateIndex.Put([]byte(dateKey), pathsData)
							if err != nil {
								log.Printf("Error updating date index: %v", err)
							}
						}
					} else {
						// If list is empty, delete key
						err = bDateIndex.Delete([]byte(dateKey))
						if err != nil {
							log.Printf("Error deleting date index key: %v", err)
						}
					}
				}
			}
		}

		// Remove from index by year and date (year-month-day)
		bYearDateIndex := tx.Bucket([]byte(bucketYearDateIndex))
		if bYearDateIndex != nil {
			yearDateKey := fmt.Sprintf("%04d-%02d-%02d", metadata.Year, metadata.Month, metadata.Day)
			pathsData := bYearDateIndex.Get([]byte(yearDateKey))
			if pathsData != nil {
				var paths []string
				err = json.Unmarshal(pathsData, &paths)
				if err == nil {
					// Remove path from list
					var newPaths []string
					for _, path := range paths {
						if path != photoPath {
							newPaths = append(newPaths, path)
						}
					}

					// Save updated list
					if len(newPaths) > 0 {
						pathsData, err = json.Marshal(newPaths)
						if err == nil {
							err = bYearDateIndex.Put([]byte(yearDateKey), pathsData)
							if err != nil {
								log.Printf("Error updating year date index: %v", err)
							}
						}
					} else {
						// If list is empty, delete key
						err = bYearDateIndex.Delete([]byte(yearDateKey))
						if err != nil {
							log.Printf("Error deleting year date index key: %v", err)
						}
					}
				}
			}
		}

		// Remove photo metadata
		err = bMetadata.Delete([]byte(photoPath))
		if err != nil {
			return fmt.Errorf("error deleting metadata: %v", err)
		}

		return nil
	})
}

// savePhotoMetadata saves photo metadata to database
func savePhotoMetadata(metadata *PhotoMetadata) error {
	return db.Update(func(tx *bolt.Tx) error {
		// Save photo metadata
		bMetadata := tx.Bucket([]byte(bucketPhotoMetadata))
		if bMetadata == nil {
			return fmt.Errorf("bucket %s not found", bucketPhotoMetadata)
		}

		// Marshal metadata to JSON
		data, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("error marshaling metadata: %v", err)
		}

		// Save metadata
		err = bMetadata.Put([]byte(metadata.Path), data)
		if err != nil {
			return fmt.Errorf("error saving metadata: %v", err)
		}

		// Update date index (month-day)
		bDateIndex := tx.Bucket([]byte(bucketDateIndex))
		if bDateIndex == nil {
			return fmt.Errorf("bucket %s not found", bucketDateIndex)
		}

		// Date index key: "month-day"
		dateKey := fmt.Sprintf("%02d-%02d", metadata.Month, metadata.Day)

		// Get current paths list for this date
		var paths []string
		pathsData := bDateIndex.Get([]byte(dateKey))
		if pathsData != nil {
			err = json.Unmarshal(pathsData, &paths)
			if err != nil {
				return fmt.Errorf("error unmarshaling paths: %v", err)
			}
		}

		// Add new path
		paths = append(paths, metadata.Path)

		// Save updated list
		pathsData, err = json.Marshal(paths)
		if err != nil {
			return fmt.Errorf("error marshaling paths: %v", err)
		}

		err = bDateIndex.Put([]byte(dateKey), pathsData)
		if err != nil {
			return fmt.Errorf("error saving date index: %v", err)
		}

		// Update year and date index (year-month-day)
		bYearDateIndex := tx.Bucket([]byte(bucketYearDateIndex))
		if bYearDateIndex == nil {
			return fmt.Errorf("bucket %s not found", bucketYearDateIndex)
		}

		// Date index key: "year-month-day"
		yearDateKey := fmt.Sprintf("%04d-%02d-%02d", metadata.Year, metadata.Month, metadata.Day)

		// Get current paths list for this date and year
		var yearPaths []string
		yearPathsData := bYearDateIndex.Get([]byte(yearDateKey))
		if yearPathsData != nil {
			err = json.Unmarshal(yearPathsData, &yearPaths)
			if err != nil {
				return fmt.Errorf("error unmarshaling year paths: %v", err)
			}
		}

		// Add new path
		yearPaths = append(yearPaths, metadata.Path)

		// Save updated list
		yearPathsData, err = json.Marshal(yearPaths)
		if err != nil {
			return fmt.Errorf("error marshaling year paths: %v", err)
		}

		err = bYearDateIndex.Put([]byte(yearDateKey), yearPathsData)
		if err != nil {
			return fmt.Errorf("error saving year date index: %v", err)
		}

		return nil
	})
}

// extractPhotoMetadata extracts metadata from photo
func extractPhotoMetadata(photoPath string, calculateHash bool) (*PhotoMetadata, error) {
	// Read EXIF data
	exif := getPhotoExif(photoPath)

	// Get file information
	fileInfo, err := os.Stat(photoPath)
	if err != nil {
		return nil, fmt.Errorf("error getting file info: %v", err)
	}

	metadata := &PhotoMetadata{
		Path:         photoPath,
		IndexedAt:    time.Now(),
		ModifiedTime: fileInfo.ModTime(),
		FileSize:     fileInfo.Size(),
	}

	// Calculate file hash if needed
	if calculateHash {
		hash, err := calculateMD5(photoPath)
		if err == nil {
			metadata.FileHash = hash
		} else {
			log.Printf("Error calculating hash for %s: %v", photoPath, err)
		}
	}

	// Set camera model
	if exif != nil {
		var cameraModel string
		if len(strings.TrimSpace(exif.Make)) > 0 {
			cameraModel = exif.Make
		}
		if len(strings.TrimSpace(exif.Model)) > 0 {
			cameraModel += " " + exif.Model
		}
		metadata.CameraModel = cameraModel

		// Set GPS coordinates
		if len(exif.GPSLatitude) > 0 && len(exif.GPSLongitude) > 0 {
			lat, err := convertGPSCoordinatesToFloat(exif.GPSLatitude)
			if err == nil {
				metadata.GpsLat = lat
			}

			lon, err := convertGPSCoordinatesToFloat(exif.GPSLongitude)
			if err == nil {
				metadata.GpsLon = lon
			}
		}

		// Set shooting date
		if len(strings.TrimSpace(exif.DateTimeOriginal)) > 0 {
			// Date format in EXIF: "2006:01:02 15:04:05"
			t, err := time.Parse("2006:01:02 15:04:05", exif.DateTimeOriginal)
			if err == nil {
				metadata.TakenDate = t
				metadata.Year = t.Year()
				metadata.Month = int(t.Month())
				metadata.Day = t.Day()
			} else {
				// If unable to parse date from EXIF, use file creation date
				metadata.TakenDate = fileInfo.ModTime()
				metadata.Year = fileInfo.ModTime().Year()
				metadata.Month = int(fileInfo.ModTime().Month())
				metadata.Day = fileInfo.ModTime().Day()
			}
		} else {
			// If no date in EXIF, use file creation date
			metadata.TakenDate = fileInfo.ModTime()
			metadata.Year = fileInfo.ModTime().Year()
			metadata.Month = int(fileInfo.ModTime().Month())
			metadata.Day = fileInfo.ModTime().Day()
		}
	} else {
		// If no EXIF data, use file creation date
		metadata.TakenDate = fileInfo.ModTime()
		metadata.Year = fileInfo.ModTime().Year()
		metadata.Month = int(fileInfo.ModTime().Month())
		metadata.Day = fileInfo.ModTime().Day()
	}

	return metadata, nil
}

// calculateMD5 calculates MD5 file hash
func calculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GetPhotosFromPast returns photos taken on this day in previous years
func GetPhotosFromPast(yearsAgo int, limit int) ([]string, error) {
	now := time.Now()
	month := int(now.Month())
	day := now.Day()
	year := now.Year() - yearsAgo

	// Date index key: "year-month-day"
	yearDateKey := fmt.Sprintf("%04d-%02d-%02d", year, month, day)

	var photos []string
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketYearDateIndex))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketYearDateIndex)
		}

		// Get paths list for this date and year
		pathsData := b.Get([]byte(yearDateKey))
		if pathsData == nil {
			// No photos for this date and year
			return nil
		}

		// Unmarshal paths list
		err := json.Unmarshal(pathsData, &photos)
		if err != nil {
			return fmt.Errorf("error unmarshaling paths: %v", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Limit photo count
	if len(photos) > limit {
		// Shuffle photos and take first limit
		shuffleStrings(photos)
		photos = photos[:limit]
	}

	return photos, nil
}

// GetPhotosFromThisDay returns photos taken on this day in different years
func GetPhotosFromThisDay(limit int) ([]string, error) {
	now := time.Now()
	month := int(now.Month())
	day := now.Day()

	// Date index key: "month-day"
	dateKey := fmt.Sprintf("%02d-%02d", month, day)

	var photos []string
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketDateIndex))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketDateIndex)
		}

		// Get paths list for this date
		pathsData := b.Get([]byte(dateKey))
		if pathsData == nil {
			// No photos for this date
			return nil
		}

		// Unmarshal paths list
		err := json.Unmarshal(pathsData, &photos)
		if err != nil {
			return fmt.Errorf("error unmarshaling paths: %v", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Limit photo count
	if len(photos) > limit {
		// Shuffle photos and take first limit
		shuffleStrings(photos)
		photos = photos[:limit]
	}

	return photos, nil
}

// GetIndexingStatus returns indexing status
func GetIndexingStatus() (bool, int, int, error) {
	var active bool
	var indexed, total int

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketIndexingStats))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketIndexingStats)
		}

		// Get indexing activity flag
		activeBytes := b.Get([]byte(keyIndexingActive))
		if activeBytes != nil {
			active = string(activeBytes) == "true"
		}

		// Get indexed photos count
		indexedBytes := b.Get([]byte(keyIndexedCount))
		if indexedBytes != nil {
			indexed, _ = strconv.Atoi(string(indexedBytes))
		}

		// Get total photos count
		totalBytes := b.Get([]byte(keyTotalCount))
		if totalBytes != nil {
			total, _ = strconv.Atoi(string(totalBytes))
		}

		return nil
	})

	if err != nil {
		return false, 0, 0, err
	}

	// If total photos count was not saved, get it
	if total == 0 {
		photoPath := cfg.photoPath
		photos := find(photoPath, []string{".JPG", ".PNG", ".JPEG", ".jpg", ".png", ".jpeg", ".webp", ".WEBP", ".gif",
			".GIF", ".HEIC", ".heic"})
		total = len(photos)
	}

	return active, indexed, total, nil
}

// GetLastIndexedTime returns last indexing time
func GetLastIndexedTime() (time.Time, error) {
	var lastIndexed time.Time

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketIndexingStats))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketIndexingStats)
		}

		// Get last indexing time
		lastIndexedBytes := b.Get([]byte(keyLastIndexedTime))
		if lastIndexedBytes != nil {
			t, err := time.Parse(time.RFC3339, string(lastIndexedBytes))
			if err != nil {
				return fmt.Errorf("error parsing last indexed time: %v", err)
			}
			lastIndexed = t
		}

		return nil
	})

	if err != nil {
		return time.Time{}, err
	}

	return lastIndexed, nil
}

// shuffleStrings shuffles strings in slice
func shuffleStrings(strings []string) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(strings), func(i, j int) {
		strings[i], strings[j] = strings[j], strings[i]
	})
}

// GetIndexingDuration returns last indexing duration in seconds
func GetIndexingDuration() (float64, error) {
	var duration float64

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketIndexingStats))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketIndexingStats)
		}

		// Get indexing duration
		durationBytes := b.Get([]byte(keyIndexingDuration))
		if durationBytes != nil {
			var err error
			duration, err = strconv.ParseFloat(string(durationBytes), 64)
			if err != nil {
				return fmt.Errorf("error parsing indexing duration: %v", err)
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return duration, nil
}

// ResetIndexingFlagIfStuck checks if indexing flag is stuck and resets it
func ResetIndexingFlagIfStuck() error {
	// Get indexing status
	active, _, _, err := GetIndexingStatus()
	if err != nil {
		return fmt.Errorf("error getting indexing status: %v", err)
	}

	// If indexing is active, it's likely that the previous process was interrupted
	// (e.g., container was stopped), so we should reset the flag
	if active {
		log.Println("Detected active indexing flag at startup. Resetting it as the previous process was likely interrupted.")

		// Reset indexing flag
		err = db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(bucketIndexingStats))
			if b == nil {
				return fmt.Errorf("bucket %s not found", bucketIndexingStats)
			}

			return b.Put([]byte(keyIndexingActive), []byte("false"))
		})

		if err != nil {
			return fmt.Errorf("error resetting indexing flag: %v", err)
		}

		log.Println("Indexing flag reset successfully")
	}

	return nil
}
