package tail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type DirTailer struct {
	dirPath    string
	fileExts   []string
	fileTailer *FileTailer
	fsWatcher  *fsnotify.Watcher
	lineChan   chan string
	errorChan  chan error
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	closeOnce  sync.Once
}

func NewDirTailer(dirPath string, opts ...DirTailerOption) (*DirTailer, error) {
	return NewDirTailerWithCtx(context.Background(), dirPath, opts...)
}

func NewDirTailerWithCtx(parentCtx context.Context, dirPath string, opts ...DirTailerOption) (*DirTailer, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	t := &DirTailer{
		dirPath:   dirPath,
		lineChan:  make(chan string),
		errorChan: make(chan error),
		ctx:       ctx,
		cancel:    cancel,
	}

	for _, opt := range opts {
		if err := opt(t); err != nil {
			return nil, err
		}
	}

	return t, nil
}

type DirTailerOption func(tailer *DirTailer) error

// WithFileExts 设置文件后缀
func WithFileExts(fileExts []string) DirTailerOption {
	return func(t *DirTailer) error {
		t.fileExts = fileExts
		return nil
	}
}

// 设置文件
func (t *DirTailer) initFile() error {
	fi, err := os.Stat(t.dirPath)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", t.dirPath)
	}

	absPath, err := filepath.Abs(t.dirPath)
	if err != nil {
		return err
	}
	t.dirPath = absPath

	return nil
}

// 设置 watcher
func (t *DirTailer) initWatcher() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	t.fsWatcher = w

	if err = t.fsWatcher.Add(t.dirPath); err != nil {
		return err
	}

	return nil
}

// Start 启动目录跟踪器
func (t *DirTailer) Start() error {
	fmt.Printf("%sStarting dir tailer: %s\n%s", colorGreen, t.dirPath, colorReset)

	if err := t.initFile(); err != nil {
		return err
	}

	if err := t.initWatcher(); err != nil {
		return err
	}

	t.wg.Add(1)
	go t.run()

	return nil
}

// 运行
func (t *DirTailer) run() {
	defer t.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			if err := t.handleTimerTrigger(); err != nil {
				t.errorChan <- err
				return
			}
		case event, ok := <-t.fsWatcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Create) {
				if err := t.handleDirCreate(event); err != nil {
					t.errorChan <- err
					return
				}
			}

			if event.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
				if err := t.handleDirChange(event); err != nil {
					t.errorChan <- err
					return
				}
			}
		case err, ok := <-t.fsWatcher.Errors:
			if !ok {
				return
			}
			t.errorChan <- err
			return
		}
	}
}

var ErrFileNotFound = errors.New("file not found")

// 处理定时器触发
func (t *DirTailer) handleTimerTrigger() error {
	return t.handleFileRotate()
}

// 处理文件创建
func (t *DirTailer) handleDirCreate(event fsnotify.Event) error {
	ext := filepath.Ext(event.Name)
	if !slices.Contains(t.fileExts, ext) {
		return nil
	}

	return t.handleFileRotate()
}

// 处理文件旋转
func (t *DirTailer) handleFileRotate() error {
	// 寻找最新文件
	var fileNotFound bool
	filePath, err := t.findLatestFile()
	if err != nil {
		if errors.Is(err, ErrFileNotFound) {
			fileNotFound = true
		} else {
			return err
		}
	}
	if fileNotFound {
		return nil
	}

	var opts []FileTailerOption
	if t.fileTailer != nil {
		// 正在 tail 这个文件
		if t.fileTailer.filePath == filePath {
			return nil
		}

		// 关闭 tail 旧文件
		t.fileTailer.Close()
		t.fileTailer = nil

		// 设置初始偏移量
		opts = append(opts, WithOffset(0, io.SeekStart))
	}

	fileTailer, err := NewFileTailerWithCtx(t.ctx, filePath, opts...)
	if err != nil {
		return err
	}
	t.fileTailer = fileTailer

	go t.runTailFile()

	return nil
}

// 处理目录变更
func (t *DirTailer) handleDirChange(event fsnotify.Event) error {
	if event.Name == t.dirPath {
		return fmt.Errorf("directory changed: (%v)", event.Op)
	}
	return nil
}

// 寻找最新文件
func (t *DirTailer) findLatestFile() (string, error) {
	entries, err := os.ReadDir(t.dirPath)
	if err != nil {
		return "", err
	}

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]

		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if slices.Contains(t.fileExts, ext) {
			return filepath.Join(t.dirPath, entry.Name()), nil
		}
	}

	return "", ErrFileNotFound
}

// 运行文件跟踪器
func (t *DirTailer) runTailFile() {
	t.wg.Add(1)
	defer t.wg.Done()

	if err := t.fileTailer.Start(); err != nil {
		t.errorChan <- err
		return
	}

	for {
		select {
		case <-t.ctx.Done():
			return
		case line, ok := <-t.fileTailer.GetLineChan():
			if !ok {
				return
			}
			t.lineChan <- line
		case err, ok := <-t.fileTailer.GetErrorChan():
			if !ok {
				return
			}
			t.errorChan <- err
			return
		}
	}
}

func (t *DirTailer) GetLineChan() <-chan string {
	return t.lineChan
}

func (t *DirTailer) GetErrorChan() <-chan error {
	return t.errorChan
}

func (t *DirTailer) Close() {
	t.closeOnce.Do(func() {
		t.cancel()
		t.wg.Wait()

		close(t.lineChan)
		close(t.errorChan)

		if t.fileTailer != nil {
			t.fileTailer.Close()
			t.fileTailer = nil
		}

		if t.fsWatcher != nil {
			_ = t.fsWatcher.Close()
			t.fsWatcher = nil
		}
	})
}
