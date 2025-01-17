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
	if count < 1 {
		log.Println("Photo count is less than 1, setting to 1")
		count = 1
	}

	if count > 10 {
		log.Println("Max photo count in mediaGroup is 10, setting to 10")
		count = 10
	}

	var processedPhotos []string
	var originalPhotos []string
	var photosFromAllPaths []string

	photos := find(cfg.photoPath, []string{".JPG", ".PNG", ".JPEG", ".jpg", ".png", ".jpeg", ".webp", ".WEBP", ".gif",
		".GIF", ".HEIC", ".heic"})
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
		var randomPhoto = photosFromAllPaths[i]
		compressedPhoto := processPhoto(randomPhoto)

		if compressedPhoto == nil {
			continue
		}

		processedPhotos = append(processedPhotos, *compressedPhoto)
		originalPhotos = append(originalPhotos, randomPhoto)
	}

	lastPhotos = originalPhotos

	log.Println("Random photos:", processedPhotos)
	log.Println("Original photos:", originalPhotos)

	return processedPhotos
}

func processPhoto(path string) (compressedPath *string) {
	log.Println("Checking for compression photo: ", path)

	imageName := filepath.Base(path)
	imageExt := filepath.Ext(path)

	// convert HEIC to JPG
	if imageExt == ".heic" || imageExt == ".HEIC" {
		log.Println("Converting HEIC to JPG")
		heicImage, err := bimg.Read(path)
		if err != nil {
			log.Printf("Error reading image: %s. %s", path, err)
		}

		jpgImage, err := bimg.NewImage(heicImage).Convert(bimg.JPEG)
		if err != nil {
			log.Printf("Error converting image: %s. %s", path, err)
		}

		convertedImagePath := tempProcessedPhotoPath + "/" + imageName + ".jpg"
		log.Println("Save converted image to:", convertedImagePath)
		err = bimg.Write(convertedImagePath, jpgImage)
		if err != nil {
			log.Printf("Error writing image: %s. %s", convertedImagePath, err)
			return nil
		}

		path = convertedImagePath
	}

	// Open the original image file
	originalImage, err := bimg.Read(path)
	if err != nil {
		log.Printf("Error reading image: %s. %s", path, err)
	}

	bImageOriginal := bimg.NewImage(originalImage)
	originalSizeInMb := float64(bImageOriginal.Length()) / 1024 / 1024
	originalSize, _ := bImageOriginal.Size()
	originalSizeTotal := originalSize.Width + originalSize.Height

	log.Println("Original image size:", originalSizeInMb, "Mb,", "total width+height:", originalSizeTotal)

	// Compress the image if required. Telegram supports up to 10Mb images and 10k width+height
	if originalSizeInMb <= 6.0 && originalSizeTotal < 9900 {
		log.Println("Compressing skipped")
		return &path
	}

	compressOptions := bimg.Options{
		Quality:      90,
		Compression:  6,
		NoAutoRotate: true,
	}
	compressedImage, err := bimg.Resize(originalImage, compressOptions)
	if err != nil {
		log.Printf("Error compressing image: %s. %s", path, err)
		return nil
	}

	newSizeInMb := float64(len(compressedImage)) / 1024 / 1024
	log.Println("Compressed image size: ", newSizeInMb, "Mb")

	compressedImagePath := tempProcessedPhotoPath + "/" + imageName
	log.Println("Save compressed image to:", compressedImagePath)

	err = bimg.Write(compressedImagePath, compressedImage)

	if err != nil {
		log.Printf("Error writing image: %s. %s", compressedImagePath, err)
		return nil
	}

	return &compressedImagePath
}

func clearCompressedPhotos() {
	files, err := os.ReadDir(tempProcessedPhotoPath)
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		filePath := tempProcessedPhotoPath + "/" + file.Name()
		log.Println("Removing file:", filePath)

		err := os.Remove(filePath)
		if err != nil {
			log.Fatal(err)
		}
	}
}
