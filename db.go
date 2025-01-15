package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	bolt "go.etcd.io/bbolt"
)

var db *bolt.DB

// PhotoSending stores the metadata we want to persist.
type PhotoSending struct {
	NumberOfSending int           `json:"numberOfSending"`
	MessageId       int           `json:"messageId"`
	Photos          []PhotoRecord `json:"photos"`
}

type PhotoRecord struct {
	Number int    `json:"number"`
	Path   string `json:"path"`
}

type PhotoMessageMeta struct {
	SendingNumber int    `json:"sendingNumber"`
	PhotoIndex    int    `json:"photoIndex"`
	PhotoPath     string `json:"photoPath"`
}

const (
	bucketSending         = "PhotoSending"
	bucketMessageIdToSend = "MessageIdToSending"
	keySendingCounter     = "SendingCounter" // We'll store an integer as a counter
	bucketPhotoByMsgId    = "PhotoByMsgId"   // messageId -> PhotoMessageMeta
)

func initDB(dbPath string) {
	var err error
	db, err = bolt.Open(dbPath, 0666, nil)
	if err != nil {
		log.Fatalf("cannot open bbolt: %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketSending))
		if err != nil {
			return err
		}

		_, err = tx.CreateBucketIfNotExists([]byte(bucketMessageIdToSend))
		if err != nil {
			return err
		}

		_, err = tx.CreateBucketIfNotExists([]byte(bucketPhotoByMsgId))
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Fatalf("cannot create buckets: %v", err)
	}
}

// getNextSendingNumber increments and returns the next "sending number."
func getNextSendingNumber() int {
	var nextNumber int
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSending))
		if b == nil {
			return fmt.Errorf("bucket %q does not exist", bucketSending)
		}

		// Retrieve current counter
		counterBytes := b.Get([]byte(keySendingCounter))
		var counter int
		if counterBytes != nil {
			counter, _ = strconv.Atoi(string(counterBytes))
		}

		counter++
		nextNumber = counter

		// Store the new counter
		return b.Put([]byte(keySendingCounter), []byte(strconv.Itoa(counter)))
	})
	if err != nil {
		log.Println("Error in getNextSendingNumber:", err)
		return 0
	}
	return nextNumber
}

// storeSending saves a PhotoSending record to the DB
func storeSending(ps PhotoSending) {
	data, err := json.Marshal(ps)
	if err != nil {
		log.Println("Error marshalling PhotoSending:", err)
		return
	}

	err = db.Update(func(tx *bolt.Tx) error {
		bSending := tx.Bucket([]byte(bucketSending))
		if bSending == nil {
			return fmt.Errorf("bucket %q does not exist", bucketSending)
		}

		bMap := tx.Bucket([]byte(bucketMessageIdToSend))
		if bMap == nil {
			return fmt.Errorf("bucket %q does not exist", bucketMessageIdToSend)
		}

		// Key is the "numberOfSending" as string
		sendingKey := strconv.Itoa(ps.NumberOfSending)
		if err := bSending.Put([]byte(sendingKey), data); err != nil {
			return err
		}

		// Also map messageId -> numberOfSending
		msgKey := strconv.Itoa(ps.MessageId)
		return bMap.Put([]byte(msgKey), []byte(sendingKey))
	})
	if err != nil {
		log.Println("Error storing PhotoSending:", err)
	}
}

// storePhotoMsgMeta: store a single photo's meta by messageId
func storePhotoMsgMeta(photoMsgId int, meta PhotoMessageMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketPhotoByMsgId))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketPhotoByMsgId)
		}
		return b.Put([]byte(strconv.Itoa(photoMsgId)), data)
	})
}

// getPhotoMsgMetaById: retrieve that single photo's meta
func getPhotoMsgMetaById(photoMsgId int) (*PhotoMessageMeta, error) {
	var meta PhotoMessageMeta
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketPhotoByMsgId))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketPhotoByMsgId)
		}
		data := b.Get([]byte(strconv.Itoa(photoMsgId)))
		if data == nil {
			return fmt.Errorf("no photo meta found for messageId %d", photoMsgId)
		}
		return json.Unmarshal(data, &meta)
	})
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

// getSendingByNumber returns the PhotoSending by the "sending number."
func getSendingByNumber(number int) (*PhotoSending, error) {
	var ps PhotoSending
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSending))
		if b == nil {
			return fmt.Errorf("bucket %q does not exist", bucketSending)
		}

		data := b.Get([]byte(strconv.Itoa(number)))
		if data == nil {
			return fmt.Errorf("no sending found for number: %d", number)
		}

		return json.Unmarshal(data, &ps)
	})
	if err != nil {
		return nil, err
	}
	return &ps, nil
}

// getLastSendingNumber finds the highest "sending number" in the DB
func getLastSendingNumber() (int, error) {
	var lastSendingNum int
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketSending))
		if b == nil {
			return fmt.Errorf("bucket %q does not exist", bucketSending)
		}

		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			sendingNum, parseErr := strconv.Atoi(string(k))
			if parseErr != nil {
				// Skip non-integer keys
				continue
			}
			if sendingNum > lastSendingNum {
				lastSendingNum = sendingNum
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return lastSendingNum, nil
}
