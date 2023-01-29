package main

import (
	"C"
	"github.com/h2non/bimg"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

func getRandomPhotos(count int) []string {
	if count > 10 {
		log.Println("Max photo count in mediaGroup is 10, setting to 10")
		count = 10
	}

	var randomPhotos []string
	var photosFromAllPaths []string

	photos := find(cfg.photoPath, []string{".JPG", ".PNG", ".JPEG", ".jpg", ".png", ".jpeg", ".webp", ".WEBP", ".gif",
		".GIF"})
	if len(photos) > 0 {
		photosFromAllPaths = append(photosFromAllPaths, photos...)
	}

	photoLibrarySize := len(photosFromAllPaths)
	log.Println("found photos:", photoLibrarySize)

	seed := time.Now().UnixNano()
	source := rand.NewSource(seed)
	rnd := rand.New(source)

	var random []int
	var randomPhotoCount int
	if photoLibrarySize < count {
		randomPhotoCount = photoLibrarySize
	} else {
		randomPhotoCount = count
	}

	for _, i := range rnd.Perm(photoLibrarySize)[:randomPhotoCount] {
		random = append(random, i)
	}

	for _, i := range random {
		compressedPhoto := compressPhoto(photosFromAllPaths[i])

		if compressedPhoto == nil {
			continue
		}

		randomPhotos = append(randomPhotos, *compressedPhoto)
	}

	log.Println("Random photos:", randomPhotos)

	return randomPhotos
}

func compressPhoto(path string) (compressedPath *string) {
	log.Println("Compressing photo: ", path)

	imageName := filepath.Base(path)

	// Open the original image file
	buffer, err := bimg.Read(path)
	if err != nil {
		log.Printf("Error reading image: %s. %s", path, err)
	}

	sourceImage := bimg.NewImage(buffer)
	originalSizeInMb := float64(sourceImage.Length()) / 1024 / 1024
	originalSize, _ := sourceImage.Size()
	originalSizeTotal := originalSize.Width + originalSize.Height

	log.Println("Original image size:", originalSizeInMb, "Mb,", "total width+height:", originalSizeTotal)

	// Compress the image if required. Telegram supports up to 10Mb images and 10k width+height
	if originalSizeInMb <= 6.0 && originalSizeTotal < 9900 {
		log.Println("Compressing skipped")
		return &path
	}

	compressedImage, _ := sourceImage.Resize(1920, 1080)
	newSizeInMb := float64(len(compressedImage)) / 1024 / 1024
	log.Println("Compressed image size: ", newSizeInMb, "Mb")

	compressedImagePath := compressedPhotoPath + "/" + imageName
	log.Println("Save compressed image to:", compressedImagePath)

	err = bimg.Write(compressedImagePath, compressedImage)

	if err != nil {
		log.Printf("Error writing image: %s. %s", compressedImagePath, err)
		return nil
	}

	return &compressedImagePath
}

func clearCompressedPhotos() {
	files, err := os.ReadDir(compressedPhotoPath)
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		filePath := compressedPhotoPath + "/" + file.Name()
		log.Println("Removing file:", filePath)

		err := os.Remove(filePath)
		if err != nil {
			log.Fatal(err)
		}
	}
}
