package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
)

type TailFile struct {
	FilePath string
	file     *os.File
}

type Option func(file *TailFile) error

// WithSeek 设置文件下次读取或写入操作的偏移量
func WithSeek(offset int64, whence int) Option {
	return func(tf *TailFile) error {
		if tf.file == nil {
			return fmt.Errorf("文件未打开")
		}
		_, err := tf.file.Seek(offset, whence)
		if err != nil {
			return fmt.Errorf("文件Seek错误: %w", err)
		}
		return nil
	}
}
func NewTailFile(filePath string, options ...Option) (*TailFile, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件错误: %w", err)
	}

	tf := &TailFile{
		FilePath: filePath,
		file:     file,
	}

	for _, option := range options {
		if err := option(tf); err != nil {
			return nil, err
		}
	}

	return tf, nil
}

// Close 关闭文件
func (tf *TailFile) Close() {
	_ = tf.file.Close()
}

// GetFileInfo 获取文件信息
func (tf *TailFile) GetFileInfo() (os.FileInfo, error) {
	fileInfo, err := tf.file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息错误: %w", err)
	}
	return fileInfo, nil
}

// GetSize 获取文件大小
func (tf *TailFile) GetSize() (int64, error) {
	fileInfo, err := tf.GetFileInfo()
	if err != nil {
		return 0, err
	}
	return fileInfo.Size(), nil
}

// Seek 设置文件下次读取或写入操作的偏移量
func (tf *TailFile) Seek(offset int64, whence int) (int64, error) {
	ret, err := tf.file.Seek(offset, whence)
	if err != nil {
		return 0, fmt.Errorf("文件Seek错误: %w", err)
	}
	return ret, nil
}

func (tf *TailFile) ReaderLines() error {
	reader := bufio.NewReader(tf.file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("读取文件错误: %w", err)
		}

		fmt.Println(line)

		// 最后一行
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return nil
}

func main() {
	filePath := "test.log"

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(filePath)
	if err != nil {
		log.Fatal(err)
	}

	errCh := make(chan error)

	go func() {
		tailFile, err := NewTailFile(filePath, WithSeek(0, io.SeekEnd))
		if err != nil {
			log.Fatalln(err)
		}
		defer tailFile.Close()

		lastSize, err := tailFile.GetSize()
		if err != nil {
			log.Fatalln(err)
		}

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				log.Println("event:", event)

				if event.Has(fsnotify.Write) {
					if err := handleWriteEvent(tailFile, &lastSize); err != nil {
						return
					}
				}

				if event.Has(fsnotify.Remove) {
					handleRemoveEvent(errCh, err)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				handleWatcherError(errCh, err)
				return
			}
		}
	}()

	<-make(chan struct{})
}

func handleWriteEvent(tailFile *TailFile, lastSize *int64) error {
	currSize, err := tailFile.GetSize()
	if err != nil {
		log.Fatalln(err)
	}

	if currSize == *lastSize {
		// 大小不变，无需处理
		return nil
	}

	if currSize < *lastSize {
		// 文件被截断
		if _, err := tailFile.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}

	if err := tailFile.ReaderLines(); err != nil {
		return err
	}

	*lastSize = currSize
	return nil
}

func handleRemoveEvent(errCh chan error, err error) {
	errCh <- fmt.Errorf("文件被删除: %w", err)
}

func handleWatcherError(errCh chan error, err error) {
	errCh <- fmt.Errorf("watcher错误: %w", err)
}
