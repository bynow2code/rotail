package tailer

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

	"github.com/bynow2code/rotail/internal/color"
	"github.com/fsnotify/fsnotify"
)

type fileTailer struct {
	path       string // 文件路径
	file       *os.File
	watcher    *fsnotify.Watcher
	immediate  bool  // 是否立即读取一次
	lastSize   int64 // 文件大小
	lastOffset int64 // 文件偏移量
	seekOffset int64 // 启动时文件偏移量
	seekWhence int   // 启动时文件偏移量
	lines      chan string
	errors     chan error
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// RunFileTailer 运行文件跟踪器
func RunFileTailer(ctx context.Context, path string) error {
	tailer, err := newFileTailer(ctx, path)
	if err != nil {
		return err
	}
	defer tailer.close()

	if err := tailer.producer(); err != nil {
		return err
	}

	if err := tailer.consumer(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return nil
	case err, ok := <-tailer.errors:
		if !ok {
			return nil
		}
		return err
	}
}

// 创建文件跟踪器
func newFileTailer(parentCtx context.Context, path string, opts ...fileTailerOption) (*fileTailer, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	tailer := &fileTailer{
		path:       path,
		seekOffset: 0,
		seekWhence: io.SeekEnd,
		lines:      make(chan string, 10),
		errors:     make(chan error, 1),
		ctx:        ctx,
		cancel:     cancel,
	}

	for _, opt := range opts {
		if err := opt(tailer); err != nil {
			return nil, err
		}
	}

	return tailer, nil
}

type fileTailerOption func(tailer *fileTailer) error

// 设置初始偏移量
func withSeekOffset(offset int64, whence int) fileTailerOption {
	return func(t *fileTailer) error {
		t.seekOffset = offset
		t.seekWhence = whence
		return nil
	}
}

// 设置立即读取一次，不等事件到来前
func withImmediate() fileTailerOption {
	return func(t *fileTailer) error {
		t.immediate = true
		return nil
	}
}

// 初始化文件
func (ft *fileTailer) initFile() error {
	file, err := os.Open(ft.path)
	if err != nil {
		return err
	}
	ft.file = file

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("%s is a directory", ft.path)
	}

	// 设置初始偏移量
	offset, err := ft.file.Seek(ft.seekOffset, ft.seekWhence)
	if err != nil {
		return err
	}
	ft.lastOffset = offset

	return nil
}

// 初始化 watcher
func (ft *fileTailer) initWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	ft.watcher = watcher

	return ft.watcher.Add(ft.path)
}

// 生产文件数据
func (ft *fileTailer) producer() error {
	fmt.Printf("%sStarting file tailer: %s\n%s", color.Green, ft.path, color.Reset)

	if err := ft.initFile(); err != nil {
		return err
	}

	if err := ft.initWatcher(); err != nil {
		return err
	}

	ft.wg.Add(1)
	go ft.runProduce()

	return nil
}

// 生产核心逻辑
func (ft *fileTailer) runProduce() {
	defer ft.wg.Done()
	defer func() {
		if ft.watcher != nil {
			_ = ft.watcher.Close()
			ft.watcher = nil
		}

		if ft.file != nil {
			_ = ft.file.Close()
			ft.file = nil
		}

		close(ft.lines)
		close(ft.errors)
	}()

	// 立即读取一次
	if ft.immediate {
		if err := ft.readLines(); err != nil {
			ft.sendError(err)
			return
		}
	}

	for {
		select {
		case <-ft.ctx.Done():
			return
		case event, ok := <-ft.watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Write) {
				if err := ft.readOnWriteEvent(); err != nil {
					ft.sendError(err)
					return
				}
			}

			if event.Op&(fsnotify.Rename|fsnotify.Remove|fsnotify.Create) != 0 {
				if err := ft.readOnCreateRenameRemoveEvent(event); err != nil {
					ft.sendError(err)
					return
				}
			}
		case err, ok := <-ft.watcher.Errors:
			if !ok {
				return
			}
			ft.sendError(err)
			return
		}
	}
}

// 消费文件数据
func (ft *fileTailer) consumer() error {
	ft.wg.Add(1)
	go ft.runConsume()

	return nil
}

// 消费核心逻辑
func (ft *fileTailer) runConsume() {
	defer ft.wg.Done()

	for {
		select {
		case <-ft.ctx.Done():
			return
		case line, ok := <-ft.lines:
			if !ok {
				return
			}
			fmt.Println(line)
		}
	}
}

// 消费文件数据到指定通道
func (ft *fileTailer) channelConsumer(lines chan<- string, errors chan<- error) error {
	ft.wg.Add(1)
	go ft.runChannelConsume(lines, errors)

	return nil
}

// 消费文件数据到指定通道核心逻辑
func (ft *fileTailer) runChannelConsume(lines chan<- string, errors chan<- error) {
	defer ft.wg.Done()

	for {
		select {
		case <-ft.ctx.Done():
			return
		case line, ok := <-ft.lines:
			if !ok {
				return
			}
			lines <- line
		case err, ok := <-ft.errors:
			if !ok {
				return
			}
			errors <- err
			return
		}
	}
}

// 发送错误
func (ft *fileTailer) sendError(err error) {
	select {
	case ft.errors <- err:
	default:
	}
	return
}

// 写入事件触发读行
func (ft *fileTailer) readOnWriteEvent() error {
	fileInfo, err := ft.file.Stat()
	if err != nil {
		return err
	}
	ft.lastSize = fileInfo.Size()

	if ft.lastOffset == ft.lastSize {
		return nil
	} else if ft.lastOffset < ft.lastSize {
		return ft.readLines()
	} else if ft.lastOffset > ft.lastSize {
		// 文件截断
		fmt.Printf("%sFile truncated, read from start\n%s", color.Yellow, color.Reset)

		offset, err := ft.file.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
		ft.lastOffset = offset

		return ft.readLines()
	}

	return nil
}

// 文件创建/重命名/删除触发读行
func (ft *fileTailer) readOnCreateRenameRemoveEvent(event fsnotify.Event) error {
	fmt.Printf("%sFile (%v): preparing to reopen: %s \n%s", color.Yellow, event.Op, ft.path, color.Reset)

	// 等待文件轮转
	time.Sleep(1 * time.Second)

	if err := ft.reInitFile(); err != nil {
		return err
	}

	if err := ft.reInitWatcher(); err != nil {
		return err
	}

	fmt.Printf("%sFile reopened, read from start. \n%s", color.Yellow, color.Reset)

	return ft.readLines()
}

// 重新打开文件
func (ft *fileTailer) reInitFile() error {
	_ = ft.file.Close()
	ft.file = nil

	file, err := os.Open(ft.path)
	if err != nil {
		return err
	}
	ft.file = file

	fileInfo, err := ft.file.Stat()
	if err != nil {
		return err
	}
	ft.lastSize = fileInfo.Size()

	if fileInfo.IsDir() {
		return fmt.Errorf("%s is a directory", ft.path)
	}

	// 重新设置偏移量
	offset, err := ft.file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	ft.lastOffset = offset

	return nil
}

// 重新监听文件
func (ft *fileTailer) reInitWatcher() error {
	_ = ft.watcher.Remove(ft.path)
	if err := ft.watcher.Add(ft.path); err != nil {
		return err
	}
	return nil
}

// 读取所有行
func (ft *fileTailer) readLines() error {
	// 是否读到末尾
	var isEOF bool

	reader := bufio.NewReader(ft.file)

	for {
		// 读一行
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				// 到末尾了
				isEOF = true
			} else {
				return err
			}
		}

		// 获取当前偏移量
		offset, err := ft.file.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		ft.lastOffset = offset

		// 发送行数据
		line = strings.TrimSpace(line)
		if line != "" {
			ft.lines <- line
		}

		// 读到末尾了
		if isEOF {
			break
		}
	}

	return nil
}

// 关闭所有资源
func (ft *fileTailer) close() {
	ft.cancel()
	ft.wg.Wait()
}
