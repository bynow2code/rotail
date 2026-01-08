package tailer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/bynow2code/rotail/internal/color"
	"github.com/fsnotify/fsnotify"
)

type dirTailer struct {
	dir        string   // 目录
	extensions []string // 文件后缀
	fileTailer *fileTailer
	watcher    *fsnotify.Watcher
	lines      chan string
	errors     chan error
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// RunDirTailer 运行目录跟踪器
func RunDirTailer(ctx context.Context, dir string, ext []string) error {
	var opts []dirTailerOption

	if ext != nil {
		opts = append(opts, withExtensions(ext))
	}

	tailer, err := newDirTailer(ctx, dir, opts...)
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

// 创建目录跟踪器
func newDirTailer(parentCtx context.Context, dir string, opts ...dirTailerOption) (*dirTailer, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	tailer := &dirTailer{
		dir:    dir,
		lines:  make(chan string, 10),
		errors: make(chan error, 1),
		ctx:    ctx,
		cancel: cancel,
	}

	for _, opt := range opts {
		if err := opt(tailer); err != nil {
			return nil, err
		}
	}

	return tailer, nil
}

type dirTailerOption func(tailer *dirTailer) error

// 设置要筛选的特定后缀的文件
func withExtensions(ext []string) dirTailerOption {
	return func(t *dirTailer) error {
		t.extensions = ext
		return nil
	}
}

// 初始化目录
func (dt *dirTailer) initFile() error {
	absPath, err := filepath.Abs(dt.dir)
	if err != nil {
		return err
	}
	dt.dir = absPath

	fileInfo, err := os.Stat(dt.dir)
	if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", dt.dir)
	}

	return nil
}

// 初始化 watcher
func (dt *dirTailer) initWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	dt.watcher = watcher

	return dt.watcher.Add(dt.dir)
}

// 生产文件数据
func (dt *dirTailer) producer() error {
	if err := dt.initFile(); err != nil {
		return err
	}

	fmt.Printf("%sStarting directory tailer: %s\n%s", color.Green, dt.dir, color.Reset)

	if err := dt.initWatcher(); err != nil {
		return err
	}

	dt.wg.Add(1)
	go dt.runProduce()

	return nil
}

// 生产者核心逻辑
func (dt *dirTailer) runProduce() {
	defer dt.wg.Done()
	defer func() {
		if dt.fileTailer != nil {
			dt.fileTailer.close()
			dt.fileTailer = nil
		}

		if dt.watcher != nil {
			_ = dt.watcher.Close()
			dt.watcher = nil
		}

		close(dt.lines)
		close(dt.errors)
	}()

	if err := dt.readOnStartProducer(); err != nil {
		dt.sendError(err)
		return
	}

	for {
		select {
		case <-dt.ctx.Done():
			return
		case event, ok := <-dt.watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Create) {
				if err := dt.readOnCreateEvent(event); err != nil {
					dt.sendError(err)
					return
				}
			}

			if event.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
				if err := dt.readOnRenameRemoveEvent(event); err != nil {
					dt.sendError(err)
					return
				}
			}
		case err, ok := <-dt.watcher.Errors:
			if !ok {
				return
			}
			dt.sendError(err)
			return
		}
	}
}

// 消费文件数据
func (dt *dirTailer) consumer() error {
	dt.wg.Add(1)
	go dt.runConsume()

	return nil
}

// 消费者核心逻辑
func (dt *dirTailer) runConsume() {
	defer dt.wg.Done()

	for {
		select {
		case <-dt.ctx.Done():
			return
		case line, ok := <-dt.lines:
			if !ok {
				return
			}
			fmt.Println(line)
		}
	}
}

// 发送错误
func (dt *dirTailer) sendError(err error) {
	select {
	case dt.errors <- err:
	default:
	}
	return
}

// 启动时触发读文件
func (dt *dirTailer) readOnStartProducer() error {
	path, err := dt.findLatestFile()
	if err != nil {
		if errors.Is(err, errFileNotFoundInDir) {
			fmt.Printf("%sNo suitable files found in the directory, waiting…\n%s", color.Yellow, color.Reset)
			return nil
		}
		return err
	}

	fTailer, err := newFileTailer(dt.ctx, path)
	if err != nil {
		return err
	}
	dt.fileTailer = fTailer

	if err := dt.fileTailer.producer(); err != nil {
		return err
	}

	if err := dt.fileTailer.channelConsumer(dt.lines, dt.errors); err != nil {
		return err
	}

	return nil
}

// 新文件触发读行
func (dt *dirTailer) readOnCreateEvent(event fsnotify.Event) error {
	fileInfo, err := os.Stat(event.Name)
	if err != nil {
		return err
	}
	if fileInfo.IsDir() {
		return nil
	}

	ext := filepath.Ext(event.Name)
	if !slices.Contains(dt.extensions, ext) {
		return nil
	}

	newPath, err := dt.findLatestFile()
	if err != nil {
		return err
	}

	if dt.fileTailer != nil {
		// 跟踪器正在运行
		if dt.fileTailer.path == newPath {
			return nil
		}

		// 关闭正在运行的文件跟踪器
		dt.fileTailer.close()
		dt.fileTailer = nil
	}

	fTailer, err := newFileTailer(dt.ctx, newPath, withSeekOffset(0, io.SeekStart), withImmediate())
	if err != nil {
		return err
	}
	dt.fileTailer = fTailer

	if err := dt.fileTailer.producer(); err != nil {
		return err
	}

	if err := dt.fileTailer.channelConsumer(dt.lines, dt.errors); err != nil {
		return err
	}

	return nil
}

// 目录重命名/删除触发错误
func (dt *dirTailer) readOnRenameRemoveEvent(event fsnotify.Event) error {
	if event.Name == dt.dir {
		return fmt.Errorf("directory (%v): %s", event.Op, dt.dir)
	}
	return nil
}

var errFileNotFoundInDir = errors.New("no suitable files found in the directory")

// 寻找目录中的最新文件
func (dt *dirTailer) findLatestFile() (string, error) {
	entries, err := os.ReadDir(dt.dir)
	if err != nil {
		return "", err
	}

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]

		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if slices.Contains(dt.extensions, ext) {
			return filepath.Join(dt.dir, entry.Name()), nil
		}
	}

	return "", errFileNotFoundInDir
}

// 关闭所有资源
func (dt *dirTailer) close() {
	dt.cancel()
	dt.wg.Wait()
}
