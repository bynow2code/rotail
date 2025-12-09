package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileTailer struct {
	path     string
	file     *os.File
	watcher  *fsnotify.Watcher
	size     int64
	lastSize int64
	offset   int64
	whence   int
	LineCh   chan string
	ErrCh    chan error
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func NewFileTailer(path string, opts ...FTOption) (*FileTailer, error) {
	t := &FileTailer{
		path:   path,
		offset: 0,
		whence: io.SeekEnd,
		LineCh: make(chan string),
		ErrCh:  make(chan error),
		stopCh: make(chan struct{}),
	}

	for _, opt := range opts {
		if err := opt(t); err != nil {
			return nil, err
		}
	}

	return t, nil
}

type FTOption func(tailer *FileTailer) error

func WithSeek(offset int64, whence int) FTOption {
	return func(t *FileTailer) error {
		t.offset = offset
		t.whence = whence
		return nil
	}
}

func (t *FileTailer) Start() error {
	var err error
	defer func() {
		if err != nil {
			if t.watcher != nil {
				_ = t.watcher.Close()
				t.watcher = nil
			}

			if t.file != nil {
				_ = t.file.Close()
				t.file = nil
			}
		}
	}()

	t.file, err = os.Open(t.path)
	if err != nil {
		return err
	}

	fi, err := t.file.Stat()
	if err != nil {
		return err
	}
	t.size = fi.Size()
	t.lastSize = t.size

	t.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err = t.watcher.Add(t.path); err != nil {
		return err
	}

	t.wg.Add(1)
	go t.run()

	return nil
}

func (t *FileTailer) Stop() {
	select {
	case <-t.stopCh:
	default:
		close(t.stopCh)
		t.wg.Wait()
	}
}

func (t *FileTailer) run() {
	defer t.wg.Done()

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
	}()

	if _, err := t.file.Seek(t.offset, t.whence); err != nil {
		t.ErrCh <- err
		return
	}

	t.readLines()

	for {
		select {
		case <-t.stopCh:
			return
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Write) {
				if err := t.handleFileTruncation(); err != nil {
					t.ErrCh <- err
					return
				}

				t.readLines()
			}

			if event.Op&(fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
				// 短暂休眠等待写入方重建文件
				time.Sleep(100 * time.Millisecond)

				if err := t.handleRotate(); err != nil {
					t.ErrCh <- err
					return
				}

				t.readLines()
			}
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}

			t.ErrCh <- err
			return
		}
	}
}

func (t *FileTailer) handleRotate() error {
	var err error

	_ = t.file.Close()
	t.file, err = os.Open(t.path)
	if err != nil {
		return err
	}

	fi, err := t.file.Stat()
	if err != nil {
		return err
	}
	t.size = fi.Size()
	t.lastSize = t.size

	_ = t.watcher.Remove(t.path)
	if err = t.watcher.Add(t.path); err != nil {
		return err
	}

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
