package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/fsnotify/fsnotify"
)

func main() {
	filePath := "test.log"

	fileMonitor := NewFileMonitor(filePath)
	err := fileMonitor.Start()
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer fileMonitor.Close()

	errCh := make(chan error)

	go func() {
		// 确保文件从末尾开始读取
		_, err = fileMonitor.File.Seek(0, io.SeekEnd)
		if err != nil {
			errCh <- fmt.Errorf("文件 Seek 错误：%w", err)
		}

		// 获取文件信息
		fileInfo, err := fileMonitor.File.Stat()
		if err != nil {
			errCh <- fmt.Errorf("获取文件信息错误: %w", err)
		}
		// 获取文件大小
		lastSize := fileInfo.Size()

		for {
			select {
			case event, ok := <-fileMonitor.Watcher.Events:
				if !ok {
					return
				}

				log.Println("event:", event)

				if event.Has(fsnotify.Write) {
					if err := handleWriteEvent(fileMonitor, &lastSize); err != nil {
						return
					}
				}

				if event.Has(fsnotify.Remove) {
					handleRemoveEvent(errCh)
				}
			case err, ok := <-fileMonitor.Watcher.Errors:
				if !ok {
					return
				}
				handleWatcherError(errCh, err)
				return
			}
		}
	}()

	err = <-errCh
	fmt.Println(err)
}

func handleWriteEvent(fileMonitor *FileMonitor, lastSize *int64) error {
	// 获取文件信息
	fileInfo, err := fileMonitor.File.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息错误: %w", err)
	}
	// 获取文件大小
	currSize := fileInfo.Size()

	// 大小不变，无需处理
	if currSize == *lastSize {
		return nil
	}

	// 文件被截断
	if currSize < *lastSize {
		if _, err := fileMonitor.File.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}

	// 读取文件
	reader := bufio.NewReader(fileMonitor.File)
	for {
		// 读取一行
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("读取文件错误: %w", err)
		}

		fmt.Println("行：", line)

		// 读到文件末尾
		if errors.Is(err, io.EOF) {
			break
		}
	}

	// 更新文件大小
	*lastSize = currSize
	return nil
}

func handleRemoveEvent(errCh chan error) {
	errCh <- fmt.Errorf("文件被删除")
}

func handleWatcherError(errCh chan error, err error) {
	errCh <- fmt.Errorf("watcher错误: %w", err)
}
