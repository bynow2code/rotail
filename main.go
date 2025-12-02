package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	err := tailFile("test.log")
	if err != nil {
		log.Fatalln(err)
	}
}

func tailFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件错误: %w", err)
	}
	defer file.Close()

	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("文件Seek错误: %w", err)
	}

	reader := bufio.NewReader(file)

	ticker := time.NewTicker(500 * time.Millisecond)
	for range ticker.C {
		for {
			line, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				if !errors.Is(err, io.EOF) {
					return fmt.Errorf("读取文件错误: %w", err)
				}
			}

			line = strings.TrimSpace(line)
			if line != "" {
				fmt.Println(line)
			}

			if errors.Is(err, io.EOF) {
				continue
			}
		}
	}
	return nil
}
