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

// NewDirTailer 创建目录跟踪器
func NewDirTailer(dirPath string, opts ...DirTailerOption) (*DirTailer, error) {
	return NewDirTailerWithCtx(context.Background(), dirPath, opts...)
}

// NewDirTailerWithCtx 创建带上下文的目录跟踪器
func NewDirTailerWithCtx(parentCtx context.Context, dirPath string, opts ...DirTailerOption) (*DirTailer, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	t := &DirTailer{
		dirPath:   dirPath,
		lineChan:  make(chan string, 10),
		errorChan: make(chan error, 1),
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

// WithFileExts 设置要筛选的文件后缀列表
func WithFileExts(fileExts []string) DirTailerOption {
	return func(t *DirTailer) error {
		t.fileExts = fileExts
		return nil
	}
}

// Consumer 消费者
func (t *DirTailer) Consumer() error {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		for {
			select {

			case <-t.ctx.Done():
				// 优雅退出
				return

			case line, ok := <-t.lineChan:
				// 文件跟踪器的数据
				if !ok {
					return
				}

				fmt.Println(line)
			}
		}
	}()
	return nil
}

// 消费
func (t *DirTailer) consume(lineChan chan<- string) {
	defer t.wg.Done()

	for {
		select {

		case <-t.ctx.Done():
			// 优雅退出
			return

		case line, ok := <-t.lineChan:
			// 文件跟踪器的数据
			if !ok {
				return
			}

			if lineChan != nil {
				lineChan <- line
			} else {
				fmt.Println(line)
			}

		case err, ok := <-t.errorChan:
			// 文件跟踪器报错
			if !ok {
				return
			}
			t.sendError(err)
			return
		}
	}
}

// 初始化目录
func (t *DirTailer) initFile() error {
	// 获取绝对路径
	absPath, err := filepath.Abs(t.dirPath)
	if err != nil {
		return err
	}
	t.dirPath = absPath

	// 检查目录
	fi, err := os.Stat(t.dirPath)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", t.dirPath)
	}

	return nil
}

// 初始化 watcher
func (t *DirTailer) initWatcher() error {
	// 创建 watcher
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	t.fsWatcher = w

	// 添加目录监控
	return t.fsWatcher.Add(t.dirPath)
}

// Producer 生产者
func (t *DirTailer) Producer() error {
	fmt.Printf("%sStarting directory tailer: %s\n%s", colorGreen, t.dirPath, colorReset)

	// 初始化目录
	if err := t.initFile(); err != nil {
		return err
	}

	// 初始化 watcher
	if err := t.initWatcher(); err != nil {
		return err
	}

	// 读模式1
	if err := t.readOnStartProducer(); err != nil {
		return err
	}

	t.wg.Add(1)
	go t.produce()

	return nil
}

// 生产
func (t *DirTailer) produce() {
	defer t.wg.Done()

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

			// 事件类型：创建
			if event.Has(fsnotify.Create) {
				if err := t.readOnCreateEvent(event); err != nil {
					t.sendError(err)
					return
				}
			}

			// 事件类型：重命名/删除
			if event.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
				if err := t.readOnRenameRemoveEvent(event); err != nil {
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
func (t *DirTailer) sendError(err error) {
	select {
	case t.errorChan <- err:
	default:
	}
	return
}

// 读模式1
func (t *DirTailer) readOnStartProducer() error {
	// 寻找最新文件
	filePath, err := t.findLatestFile()
	if err != nil {
		if errors.Is(err, ErrFileNotFoundInDir) {
			fmt.Printf("%sNo suitable files in directory, waiting...\n%s", colorYellow, colorReset)
			return nil
		}
		return err
	}

	// 创建文件跟踪器
	fileTailer, err := NewFileTailerWithCtx(t.ctx, filePath)
	if err != nil {
		return err
	}
	t.fileTailer = fileTailer

	// 启动文件跟踪器生产者
	if err := t.fileTailer.Producer(); err != nil {
		return err
	}

	// 启动文件跟踪器消费者
	if err := t.fileTailer.ChannelConsumer(t.lineChan, t.errorChan); err != nil {
		return err
	}

	return nil
}

func (t *DirTailer) readOnCreateEvent(event fsnotify.Event) error {
	// 检查目录
	fi, err := os.Stat(event.Name)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return nil
	}

	// 检查后缀
	ext := filepath.Ext(event.Name)
	if !slices.Contains(t.fileExts, ext) {
		return nil
	}

	// 寻找最新文件
	newFilePath, err := t.findLatestFile()
	if err != nil {
		return err
	}

	// 有正在运行的文件跟踪器
	if t.fileTailer != nil {
		// 路径相同
		if t.fileTailer.filePath == newFilePath {
			return nil
		}

		// 路径不同
		t.fileTailer.Close()
		t.fileTailer = nil
	}

	// 创建文件跟踪器
	fileTailer, err := NewFileTailerWithCtx(t.ctx, newFilePath, WithOffset(0, io.SeekStart), WithImmediateRead())
	if err != nil {
		return err
	}
	t.fileTailer = fileTailer

	// 启动文件跟踪器生产者
	if err := t.fileTailer.Producer(); err != nil {
		return err
	}

	// 启动文件跟踪器消费者
	if err := t.fileTailer.ChannelConsumer(t.lineChan, t.errorChan); err != nil {
		return err
	}

	return nil
}

func (t *DirTailer) readOnRenameRemoveEvent(event fsnotify.Event) error {
	if event.Name == t.dirPath {
		return fmt.Errorf("directory (%v): %s", event.Op, t.dirPath)
	}
	return nil
}

var ErrFileNotFoundInDir = errors.New("no suitable files found in the directory")

// 寻找最新文件
func (t *DirTailer) findLatestFile() (string, error) {
	// 读取目录
	entries, err := os.ReadDir(t.dirPath)
	if err != nil {
		return "", err
	}

	// 倒序遍历
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]

		// 过滤目录
		if entry.IsDir() {
			continue
		}

		// 过滤非后缀文件
		ext := filepath.Ext(entry.Name())
		if slices.Contains(t.fileExts, ext) {
			return filepath.Join(t.dirPath, entry.Name()), nil
		}
	}

	return "", ErrFileNotFoundInDir
}

func (t *DirTailer) GetErrorChan() <-chan error {
	return t.errorChan
}

func (t *DirTailer) Close() {
	t.closeOnce.Do(func() {
		t.cancel()
		t.wg.Wait()

		if t.fileTailer != nil {
			t.fileTailer.Close()
			t.fileTailer = nil
		}

		if t.fsWatcher != nil {
			_ = t.fsWatcher.Close()
			t.fsWatcher = nil
		}

		close(t.errorChan)
	})
}
