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
	filePath      string
	fileHandle    *os.File
	fsWatcher     *fsnotify.Watcher
	immediateRead bool
	lastFileSize  int64
	lastOffset    int64
	seekOffset    int64
	seekWhence    int
	lineChan      chan string
	errorChan     chan error
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	closeOnce     sync.Once
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
		lineChan:   make(chan string, 10),
		errorChan:  make(chan error, 1),
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

// WithImmediateRead 设置立即读取一次，不等事件到来前
func WithImmediateRead() FileTailerOption {
	return func(t *FileTailer) error {
		t.immediateRead = true
		return nil
	}
}

// Consumer 消费者
func (t *FileTailer) Consumer() error {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		for {
			select {
			case <-t.ctx.Done():
				// 优雅退出
				return

			case line, ok := <-t.lineChan:
				// 接收数据
				if !ok {
					return
				}
				fmt.Println(line)
			}
		}
	}()

	return nil
}

// ChannelConsumer 消费数据并发送到指定通道
func (t *FileTailer) ChannelConsumer(lineChan chan<- string, errorChan chan<- error) error {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		for {
			select {
			case <-t.ctx.Done():
				// 优雅退出
				return

			case line, ok := <-t.lineChan:
				// 数据
				if !ok {
					return
				}
				lineChan <- line

			case err, ok := <-t.errorChan:
				// 错误
				if !ok {
					return
				}
				errorChan <- err
				return
			}
		}
	}()

	return nil
}

// 初始化文件
func (t *FileTailer) initFile() error {
	// 打开文件
	f, err := os.Open(t.filePath)
	if err != nil {
		return err
	}
	t.fileHandle = f

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	// 检查是否为文件
	if fi.IsDir() {
		return fmt.Errorf("%s is a directory", t.filePath)
	}

	// 设置初始偏移量
	offset, err := t.fileHandle.Seek(t.seekOffset, t.seekWhence)
	if err != nil {
		return err
	}
	t.lastOffset = offset

	return nil
}

// 初始化 watcher
func (t *FileTailer) initWatcher() error {
	// 创建 watcher
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	t.fsWatcher = w

	// 添加文件监控
	return t.fsWatcher.Add(t.filePath)
}

// Producer 生产者
func (t *FileTailer) Producer() error {
	fmt.Printf("%sStarting file tailer: %s\n%s", colorGreen, t.filePath, colorReset)

	// 初始化文件
	if err := t.initFile(); err != nil {
		return err
	}

	// 初始化 watcher
	if err := t.initWatcher(); err != nil {
		return err
	}

	t.wg.Add(1)
	go t.produce()

	return nil
}

// 生产
func (t *FileTailer) produce() {
	defer t.wg.Done()

	// 判断是否立即读取一次
	if t.immediateRead {
		if err := t.readLines(); err != nil {
			t.sendError(err)
			return
		}
	}

	for {

		select {
		case <-t.ctx.Done():
			// 优雅退出
			return

		case event, ok := <-t.fsWatcher.Events:
			// watcher 事件
			if !ok {
				return
			}

			// 事件类型：写入
			if event.Has(fsnotify.Write) {
				if err := t.readOnWriteEvent(); err != nil {
					t.sendError(err)
					return
				}
			}

			// 事件类型：重命名/删除/新建
			if event.Op&(fsnotify.Rename|fsnotify.Remove|fsnotify.Create) != 0 {
				if err := t.readOnCreateRenameRemoveEvent(event); err != nil {
					t.sendError(err)
					return
				}
			}

		case err, ok := <-t.fsWatcher.Errors:
			// watcher 错误
			if !ok {
				return
			}
			t.sendError(err)
			return
		}
	}
}

// 发送错误
func (t *FileTailer) sendError(err error) {
	select {
	case t.errorChan <- err:
	default:
	}
	return
}

func (t *FileTailer) readOnWriteEvent() error {
	// 重新获取文件大小
	fi, err := t.fileHandle.Stat()
	if err != nil {
		return err
	}
	t.lastFileSize = fi.Size()

	if t.lastOffset == t.lastFileSize {
		return nil
	} else if t.lastOffset < t.lastFileSize {
		return t.readLines()
	} else if t.lastOffset > t.lastFileSize {
		// 文件截断
		fmt.Printf("%sFile truncated, read from start\n%s", colorYellow, colorReset)
		offset, err := t.fileHandle.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
		t.lastOffset = offset
		return t.readLines()
	}

	return nil
}

func (t *FileTailer) readOnCreateRenameRemoveEvent(event fsnotify.Event) error {
	fmt.Printf("%sFile (%v): preparing to reopen: %s \n%s", colorYellow, event.Op, t.filePath, colorReset)

	// 等待文件轮转
	time.Sleep(1 * time.Second)

	// 重新初始化文件
	if err := t.reOpenFile(); err != nil {
		return err
	}

	// 重新监听文件
	if err := t.reAddWatcher(); err != nil {
		return err
	}

	return t.readLines()
}

// 重新初始化文件
func (t *FileTailer) reOpenFile() error {
	// 关闭上个文件
	_ = t.fileHandle.Close()
	t.fileHandle = nil

	// 重新打开文件
	f, err := os.Open(t.filePath)
	if err != nil {
		return err
	}
	t.fileHandle = f

	// 重新获取文件大小
	fi, err := t.fileHandle.Stat()
	if err != nil {
		return err
	}
	t.lastFileSize = fi.Size()

	// 检查是否为文件
	if fi.IsDir() {
		return fmt.Errorf("%s is a directory", t.filePath)
	}

	// 重新设置偏移量
	offset, err := t.fileHandle.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	t.lastOffset = offset

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

// 读取所有行
func (t *FileTailer) readLines() error {
	// 是否读到末尾
	var isEOF bool

	reader := bufio.NewReader(t.fileHandle)

	for {
		// 读一行
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				// 读到末尾了
				isEOF = true
			} else {
				return err
			}
		}

		// 获取当前偏移量
		offset, err := t.fileHandle.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		t.lastOffset = offset

		// 发送行数据
		line = strings.TrimSpace(line)
		if line != "" {
			t.lineChan <- line
		}

		// 读到末尾了
		if isEOF {
			break
		}
	}

	return nil
}

func (t *FileTailer) GetErrorChan() <-chan error {
	return t.errorChan
}

func (t *FileTailer) Close() {
	t.closeOnce.Do(func() {
		t.cancel()
		t.wg.Wait()

		if t.fsWatcher != nil {
			_ = t.fsWatcher.Close()
			t.fsWatcher = nil
		}

		if t.fileHandle != nil {
			_ = t.fileHandle.Close()
		}

		close(t.lineChan)
		close(t.errorChan)
	})
}
