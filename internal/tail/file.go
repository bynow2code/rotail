package tail

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileTailer struct {
	filePath     string
	fileHandle   *os.File
	fsWatcher    *fsnotify.Watcher
	lastFileSize int64
	seekOffset   int64
	seekWhence   int
	lineChan     chan string
	errorChan    chan error
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewFileTailer 创建文件跟踪器
func NewFileTailer(filePath string, opts ...FileTailerOption) (*FileTailer, error) {
	return NewFileTailerWithCtx(context.Background(), filePath, opts...)
}

// NewFileTailerWithCtx 创建带上下文的文件跟踪器
func NewFileTailerWithCtx(parentCtx context.Context, filePath string, opts ...FileTailerOption) (*FileTailer, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	t := &FileTailer{
		filePath:   filePath,
		seekOffset: 0,
		seekWhence: io.SeekEnd,
		lineChan:   make(chan string),
		errorChan:  make(chan error),
		ctx:        ctx,
		cancel:     cancel,
	}

	for _, opt := range opts {
		if err := opt(t); err != nil {
			return nil, err
		}
	}

	return t, nil
}

type FileTailerOption func(tailer *FileTailer) error

// WithOffset 设置初始偏移量
func WithOffset(offset int64, whence int) FileTailerOption {
	return func(t *FileTailer) error {
		t.seekOffset = offset
		t.seekWhence = whence
		return nil
	}
}

func (t *FileTailer) initFile() error {
	f, err := os.Open(t.filePath)
	if err != nil {
		return fmt.Errorf("open file error:%w", err)
	}
	t.fileHandle = f

	// 设置初始偏移量
	if _, err := t.fileHandle.Seek(t.seekOffset, t.seekWhence); err != nil {
		return fmt.Errorf("seek file error:%w", err)
	}

	// 设置初始文件大小
	fi, err := t.fileHandle.Stat()
	if err != nil {
		return fmt.Errorf("stat file error:%w", err)
	}
	t.lastFileSize = fi.Size()

	return nil
}

func (t *FileTailer) initWatcher() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("new fsnotify watcher error:%w", err)
	}
	t.fsWatcher = w

	if err = t.fsWatcher.Add(t.filePath); err != nil {
		return fmt.Errorf("add fsnotify watcher error:%w", err)
	}

	return nil
}

// Start 启动文件跟踪器
func (t *FileTailer) Start() error {
	fmt.Printf("%sStarting file tailer:%s\n%s", colorGreen, t.filePath, colorReset)

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

func (t *FileTailer) run() {
	defer t.wg.Done()

	ticker := time.NewTicker(200 * time.Millisecond)
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

			if event.Has(fsnotify.Write) {
				if err := t.handleFileWrite(); err != nil {
					t.errorChan <- err
					return
				}
			}

			if event.Op&(fsnotify.Rename|fsnotify.Remove|fsnotify.Create) != 0 {
				if err := t.handleFileResourceChange(); err != nil {
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

// 处理定时器触发
func (t *FileTailer) handleTimerTrigger() error {
	sizeState, err := t.handleFileTruncation()
	if err != nil {
		return err
	}
	if sizeState == fileSizeUnchanged {
		return nil
	}

	if err := t.readLines(); err != nil {
		return err
	}

	return nil
}

// 处理文件写入
func (t *FileTailer) handleFileWrite() error {
	if _, err := t.handleFileTruncation(); err != nil {
		return err
	}

	if err := t.readLines(); err != nil {
		return err
	}

	return nil
}

// 处理文件资源变化
func (t *FileTailer) handleFileResourceChange() error {
	if err := t.handleFileRotation(); err != nil {
		return err
	}

	if err := t.readLines(); err != nil {
		return err
	}

	return nil
}

type fileSizeState int

const (
	fileSizeIncreased fileSizeState = iota + 1 // 文件变大
	fileSizeUnchanged                          // 文件不变
	fileSizeDecreased                          // 文件变小
)

// 处理文件截断
func (t *FileTailer) handleFileTruncation() (fileSizeState, error) {
	fi, err := t.fileHandle.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat file error:%w", err)
	}

	currentFileSize := fi.Size()
	if currentFileSize == t.lastFileSize {
		return fileSizeUnchanged, nil
	} else if currentFileSize > t.lastFileSize {
		t.lastFileSize = currentFileSize
		return fileSizeIncreased, nil
	} else if currentFileSize < t.lastFileSize {
		fmt.Printf("%sFile truncated:%s\n%s", colorYellow, t.filePath, colorReset)

		if _, err := t.fileHandle.Seek(0, io.SeekStart); err != nil {
			return 0, fmt.Errorf("seek file error:%w", err)
		}

		t.lastFileSize = currentFileSize
		return fileSizeDecreased, err
	}

	return 0, nil
}

// 读取所有行
func (t *FileTailer) readLines() error {
	reader := bufio.NewReader(t.fileHandle)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("read file error:%w", err)
		}

		line = strings.TrimSpace(line)
		if line != "" {
			t.lineChan <- line
		}

		// 最后一行
		if errors.Is(err, io.EOF) {
			break
		}
	}

	return nil
}

// 处理文件轮转
func (t *FileTailer) handleFileRotation() error {
	// 等待文件轮转
	time.Sleep(10 * time.Second)

	if err := t.reOpenFile(); err != nil {
		return err
	}

	if err := t.reAddWatcher(); err != nil {
		return err
	}

	return nil
}

// 重新打开文件
func (t *FileTailer) reOpenFile() error {
	fmt.Printf("%sFile rotated:%s\n%s", colorYellow, t.filePath, colorReset)

	_ = t.fileHandle.Close()
	t.fileHandle = nil

	f, err := os.Open(t.filePath)
	if err != nil {
		return err
	}
	t.fileHandle = f

	// 重新设置初始偏移量
	if _, err := t.fileHandle.Seek(t.seekOffset, t.seekWhence); err != nil {
		return err
	}

	// 重新初始文件大小
	fi, err := t.fileHandle.Stat()
	if err != nil {
		return err
	}
	t.lastFileSize = fi.Size()

	return nil
}

// 重新监听文件
func (t *FileTailer) reAddWatcher() error {
	_ = t.fsWatcher.Remove(t.filePath)
	if err := t.fsWatcher.Add(t.filePath); err != nil {
		return err
	}
	return nil
}

func (t *FileTailer) GetLineChan() <-chan string {
	return t.lineChan
}

func (t *FileTailer) GetErrorChan() <-chan error {
	return t.errorChan
}

func (t *FileTailer) Close() {
	t.cancel()
	t.wg.Wait()

	close(t.lineChan)
	close(t.errorChan)

	if t.fsWatcher != nil {
		_ = t.fsWatcher.Close()
		t.fsWatcher = nil
	}

	if t.fileHandle != nil {
		_ = t.fileHandle.Close()
	}
}
