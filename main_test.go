package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDir(t *testing.T) {
	dirPath := "./logs"
	ext := ".log"
	dirEntries, err := os.ReadDir(dirPath)
	_ = err
	fmt.Println(dirEntries)
	for _, entry := range dirEntries {
		fmt.Println(entry.Name())
	}

	for i := len(dirEntries) - 1; i >= 0; i-- {
		if !dirEntries[i].IsDir() {
			if ext == filepath.Ext(dirEntries[i].Name()) {
				fmt.Println(dirEntries[i])
				break
			}
		}
	}
}
