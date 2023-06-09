package main

import (
	"io/fs"
	"log"
	"path/filepath"
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
