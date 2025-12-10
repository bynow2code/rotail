package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/bynow2code/rotail/internal/tail"
)

var version = "0.0.0-dev"

func main() {
	file := flag.String("f", "", "File path to tail (e.g. /var/log/app.log)")
	dir := flag.String("d", "", "Directory path to tail (e.g. /var/log)")
	ext := flag.String("ext", ".log", "Comma-separated file extensions, default .log (e.g. .log,.txt)")
	ver := flag.Bool("v", false, "Show version")
	help := flag.Bool("h", false, "Show help")

	flag.Parse()

	if *help {
		printHelp()
		return
	}

	// 版本号
	if *ver {
		fmt.Println(version)
		return
	}

	// 文件 tail
	if *file != "" {
		t, err := tail.NewFileTailer(*file)
		if err != nil {
			fmt.Println("Failed to start file tailer:", err)
			return
		}
		runTailer(t)
		return
	}

	// 目录 tail
	if *dir != "" {
		exts := strings.Split(*ext, ",")
		t, err := tail.NewDirTailer(*dir, tail.WithFileExts(exts))
		if err != nil {
			fmt.Println("Failed to start directory tailer:", err)
			return
		}
		runTailer(t)
		return
	}

	// 无参数
	printHelp()
}

func printHelp() {
	flag.PrintDefaults()
}

// 统一管理 Tailer 生命周期
func runTailer(t tail.Tailer) {
	var wg sync.WaitGroup

	defer func() {
		t.Close() // 停止生产端，关闭 channel
		wg.Wait() // 等消费端退出
	}()

	// 启动生产端
	if err := t.Start(); err != nil {
		fmt.Printf("Startup error: %v\n", err)
		return
	}

	// 启动日志消费者
	wg.Add(1)
	go func() {
		defer wg.Done()
		for line := range t.GetLineChan() {
			fmt.Println(line)
		}
	}()

	// 监听信号和错误
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Println("Received stop signal")
	case err, ok := <-t.GetErrorChan():
		if !ok {
			return
		}
		fmt.Printf("Tailer error: %v\n", err)
	}
}
