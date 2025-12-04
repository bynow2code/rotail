package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type FileTailer struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	size     int64
	lastSize int64
	offset   int64
	whence   int
	watcher  *fsnotify.Watcher
	LineCh   chan string
	ErrCh    chan error
}

func NewFileTailer(path string) *FileTailer {
	return &FileTailer{
		path:   path,
		offset: 0,
		whence: io.SeekEnd,
		LineCh: make(chan string),
		ErrCh:  make(chan error),
	}
}

func (t *FileTailer) Start() error {
	var err error

	t.file, err = os.Open(t.path)
	if err != nil {
		return err
	}

	fileInfo, err := t.file.Stat()
	if err != nil {
		return err
	}
	t.size = fileInfo.Size()

	t.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		if _, err = t.file.Seek(t.offset, t.whence); err != nil {
			t.ErrCh <- err
			return
		}

		t.ReadLine()

		t.lastSize = t.size

		for {
			select {
			case event, ok := <-t.watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {

					fileInfo, err := os.Stat(t.path)
					if err != nil {
						t.ErrCh <- err
						return
					}
					currSize := fileInfo.Size()
					if currSize < t.lastSize {
						if _, err = t.file.Seek(0, io.SeekStart); err != nil {
							t.ErrCh <- err
							return
						}
						t.lastSize = currSize
					}

					t.ReadLine()
				}
			case err, ok := <-t.watcher.Errors:
				if !ok {
					return
				}
				t.ErrCh <- err
				return
			}
		}
	}()

	if err = t.watcher.Add(t.path); err != nil {
		return err
	}

	return nil
}

func (t *FileTailer) ReadLine() {
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

func (t *FileTailer) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.file != nil {
		t.file.Close()
		t.file = nil
	}

	if t.watcher != nil {
		t.watcher.Close()
		t.watcher = nil
	}

	if t.LineCh != nil {
		close(t.LineCh)
	}

	if t.ErrCh != nil {
		close(t.ErrCh)
	}
}
