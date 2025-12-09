package tail

import (
	"bufio"
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
	path     string
	file     *os.File
	watcher  *fsnotify.Watcher
	size     int64
	lastSize int64
	offset   int64
	whence   int
	lineCh   chan string
	errCh    chan error
	stopCh   chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// NewFile 创建一个文件跟踪器
func NewFile(path string, opts ...FTOption) (*FileTailer, error) {
	t := &FileTailer{
		path:   path,
		offset: 0,
		whence: io.SeekEnd,
		lineCh: make(chan string),
		errCh:  make(chan error),
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

// WithSeek 设置偏移量
func WithSeek(offset int64, whence int) FTOption {
	return func(t *FileTailer) error {
		t.offset = offset
		t.whence = whence
		return nil
	}
}

// Start 启动文件跟踪器
func (t *FileTailer) Start() error {
	fmt.Printf("%sStarting f tailer:%s\n%s", colorGreen, t.path, colorReset)

	f, err := os.Open(t.path)
	if err != nil {
		return err
	}
	t.file = f

	// 设置初始偏移量
	if _, err := t.file.Seek(t.offset, t.whence); err != nil {
		t.cleanup()
		return err
	}

	// 设置初始文件大小
	fi, err := t.file.Stat()
	if err != nil {
		t.cleanup()
		return err
	}
	t.size = fi.Size()
	t.lastSize = t.size

	// 创建 watcher
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.cleanup()
		return err
	}
	t.watcher = w

	if err = t.watcher.Add(t.path); err != nil {
		t.cleanup()
		return err
	}

	t.wg.Add(1)
	go t.run()

	return nil
}

// run 运行文件跟踪器
func (t *FileTailer) run() {
	defer t.wg.Done()
	defer t.cleanup()

	if err := t.readLines(); err != nil {
		t.errCh <- err
		return
	}

	for {
		select {
		case <-t.stopCh:
			return
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Write) {
				if err := t.handleWriteEvent(); err != nil {
					t.errCh <- err
					return
				}

			}

			if event.Op&(fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
				// 等待写入方重建文件
				time.Sleep(100 * time.Millisecond)
				if err := t.handleRotate(); err != nil {
					t.errCh <- err
					return
				}
			}
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			t.errCh <- err
			return
		}
	}
}

// handleWriteEvent 处理文件写入事件
func (t *FileTailer) handleWriteEvent() error {
	if err := t.handleTruncate(); err != nil {
		return err
	}

	return t.readLines()
}

// handleTruncate 处理文件截断
func (t *FileTailer) handleTruncate() error {
	fi, err := t.file.Stat()
	if err != nil {
		return err
	}

	// 文件被截断
	curSize := fi.Size()
	if curSize < t.lastSize {
		fmt.Printf("%sFile truncated:%s\n%s", colorYellow, t.path, colorReset)
		if _, err := t.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}

	t.lastSize = curSize

	return nil
}

// handleRotate 处理文件轮转
func (t *FileTailer) handleRotate() error {
	if t.file != nil {
		_ = t.file.Close()
		t.file = nil
	}

	f, err := os.Open(t.path)
	if err != nil {
		return err
	}
	t.file = f

	// 重新设置初始文件大小
	fi, err := t.file.Stat()
	if err != nil {
		return err
	}
	t.size = fi.Size()
	t.lastSize = t.size

	// 重新添加 watcher
	_ = t.watcher.Remove(t.path)
	if err = t.watcher.Add(t.path); err != nil {
		return err
	}

	return t.readLines()
}

// readLines 读取文件行
func (t *FileTailer) readLines() error {
	reader := bufio.NewReader(t.file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}

		line = strings.TrimSpace(line)
		if line != "" {
			t.lineCh <- line
		}

		if errors.Is(err, io.EOF) {
			break
		}
	}

	return nil
}

// GetLineCh 获取行通道
func (t *FileTailer) GetLineCh() <-chan string {
	return t.lineCh
}

// GetErrCh 获取错误通道
func (t *FileTailer) GetErrCh() <-chan error {
	return t.errCh
}

// Stop 停止文件跟踪器
func (t *FileTailer) Stop() {
	t.stopOnce.Do(func() {
		close(t.stopCh)
		t.wg.Wait()
	})
}

// cleanup 清理资源
func (t *FileTailer) cleanup() {
	if t.watcher != nil {
		_ = t.watcher.Close()
		t.watcher = nil
	}

	if t.file != nil {
		_ = t.file.Close()
		t.file = nil
	}

	close(t.lineCh)
	close(t.errCh)
}
