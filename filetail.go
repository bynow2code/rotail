package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileTailer struct {
	path     string
	file     *os.File
	watcher  *fsnotify.Watcher
	size     int64
	lastSize int64
	LineCh   chan string
	ErrCh    chan error
}

func NewFileTailer(path string) *FileTailer {
	return &FileTailer{
		path:   path,
		LineCh: make(chan string),
		ErrCh:  make(chan error),
	}
}

func (t *FileTailer) Start() error {
	f, err := os.Open(t.path)
	if err != nil {
		return err
	}
	t.file = f

	fi, err := t.file.Stat()
	if err != nil {
		_ = f.Close()
		t.file = nil
		return err
	}
	t.size = fi.Size()
	t.lastSize = t.size

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		_ = f.Close()
		t.file = nil
		return err
	}
	t.watcher = watcher

	if err = watcher.Add(t.path); err != nil {
		_ = t.file.Close()
		t.file = nil
		_ = t.watcher.Close()
		t.watcher = nil
		return err
	}

	go t.run()

	return nil
}

func (t *FileTailer) run() {
	defer func() {
		if t.watcher != nil {
			_ = t.watcher.Close()
			t.watcher = nil
		}
		if t.file != nil {
			_ = t.file.Close()
			t.file = nil
		}

		close(t.LineCh)
		close(t.ErrCh)
	}()

	if _, err := t.file.Seek(0, io.SeekEnd); err != nil {
		t.ErrCh <- err
		return
	}

	t.readLines()

	for {
		select {
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}
			fmt.Println(event)

			if event.Has(fsnotify.Write) {
				if err := t.handleFileTruncation(); err != nil {
					t.ErrCh <- err
					return
				}

				t.readLines()
			}

			if event.Op&(fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
				if err := t.handleRotate(); err != nil {
					t.ErrCh <- err
					return
				}

				// 短暂休眠等待写入方重建文件
				time.Sleep(1000 * time.Millisecond)

				t.readLines()
			}
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

func (t *FileTailer) handleRotate() error {
	f, err := os.Open(t.path)
	if err != nil {
		return err
	}
	_ = t.file.Close()
	t.file = f

	fi, err := t.file.Stat()
	if err != nil {
		return err
	}
	t.size = fi.Size()
	t.lastSize = t.size

	return nil
}
func (t *FileTailer) handleFileTruncation() error {
	fi, err := t.file.Stat()
	if err != nil {
		return err
	}

	curSize := fi.Size()
	if curSize < t.lastSize {
		if _, err := t.file.Seek(0, io.SeekStart); err != nil {
			return err

		}
	}

	t.lastSize = curSize

	return nil
}

func (t *FileTailer) readLines() {
	reader := bufio.NewReader(t.file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			t.ErrCh <- err
			return
		}

		t.LineCh <- line

		if errors.Is(err, io.EOF) {
			break
		}
	}
}
