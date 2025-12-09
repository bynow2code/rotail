package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type DirTailer struct {
	path    string
	ext     []string
	ft      *FileTailer
	watcher *fsnotify.Watcher
	LineCh  chan string
	ErrCh   chan error
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

func NewDirTailer(path string, opts ...DTOption) (*DirTailer, error) {
	t := &DirTailer{
		path:   path,
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

type DTOption func(tailer *DirTailer) error

func WithExt(ext []string) DTOption {
	return func(t *DirTailer) error {
		t.ext = ext
		return nil
	}
}

func (t *DirTailer) Start() error {
	var err error
	defer func() {
		if err != nil {
			if t.watcher != nil {
				_ = t.watcher.Close()
				t.watcher = nil
			}
		}
	}()

	if !filepath.IsAbs(t.path) {
		var absPath string
		absPath, err = filepath.Abs(t.path)
		if err != nil {
			return err
		}
		t.path = absPath
	}

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

func (t *DirTailer) run() {
	defer t.wg.Done()

	defer func() {
		if t.watcher != nil {
			_ = t.watcher.Close()
			t.watcher = nil
		}

		close(t.LineCh)
	}()

	go t.runTailFile()

	for {
		select {
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Create) {
				t.handleCreateEvent(event)
			}

			if event.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
				//file, err := t.findFileInDir()
				//if err != nil {
				//	t.ErrCh <- err
				//	return
				//}

			}
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}
func (t *DirTailer) runTailFile() {
	file, err := t.findFileInDir()
	if err != nil {
		t.ErrCh <- err
		return
	}

	ft, err := NewFileTailer(file)
	if err != nil {
		t.ErrCh <- err
		return
	}
	t.ft = ft

	if err := t.ft.Start(); err != nil {
		t.ErrCh <- err
		return
	}

	go func() {
		for line := range t.ft.LineCh {
			t.LineCh <- line
		}
	}()

	select {
	case <-t.stopCh:
		t.ft.Stop()
	case err := <-t.ft.ErrCh:
		t.ErrCh <- err
		return
	}
}

func (t *DirTailer) handleCreateEvent(event fsnotify.Event) {
	path := event.Name
	fi, err := os.Stat(path)
	if err != nil {
		t.ErrCh <- err
		return
	}

	if fi.IsDir() {
		return
	}

	ext := filepath.Ext(path)
	if !slices.Contains(t.ext, ext) {
		return
	}

	t.ft.Stop()

	go func() {
		ft, err := NewFileTailer(path, WithSeek(0, io.SeekStart))
		if err != nil {
			t.ErrCh <- err
			return
		}
		t.ft = ft

		if err := t.ft.Start(); err != nil {
			t.ErrCh <- err
			return
		}

		go func() {
			for line := range t.ft.LineCh {
				t.LineCh <- line
			}
		}()

		select {
		case <-t.stopCh:
			t.ft.Stop()
		case err := <-t.ft.ErrCh:
			t.ErrCh <- err
			return
		}
	}()

	return
}

func (t *DirTailer) findFileInDir() (string, error) {
	entries, err := os.ReadDir(t.path)
	if err != nil {
		return "", err
	}

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if slices.Contains(t.ext, ext) {
			return filepath.Join(t.path, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no file found in %s", t.path)
}

func (t *DirTailer) Stop() {
	select {
	case <-t.stopCh:
	default:
		close(t.stopCh)
		t.wg.Wait()
	}
}
