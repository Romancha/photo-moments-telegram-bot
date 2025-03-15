package main

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"strconv"
	"strings"
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
