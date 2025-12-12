package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
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

	// 文件跟踪器
	if *file != "" {
		runFileTailer(*file)
		return
	}

	// 目录跟踪器
	if *dir != "" {
		runDirTailer(*dir, *ext)
		return
	}

	// 无参数
	printHelp()
}

func printHelp() {
	flag.PrintDefaults()
}

func runFileTailer(file string) {
	tailer, err := tail.NewFileTailer(file)
	if err != nil {
		fmt.Println("Failed to start file tailer:", err)
		return
	}
	runTailer(tailer)
}

func runDirTailer(dir string, exts string) {
	fileExts := strings.Split(exts, ",")
	tailer, err := tail.NewDirTailer(dir, tail.WithFileExts(fileExts))
	if err != nil {
		fmt.Println("Failed to start directory tailer:", err)
		return
	}
	runTailer(tailer)
}
func runTailer(tailer tail.Tailer) {
	defer tailer.Close()

	// 启动跟踪器生产者
	if err := tailer.Producer(); err != nil {
		fmt.Println("Failed to start tailer producer: ", err)
		return
	}

	// 启动跟踪器消费者
	if err := tailer.Consumer(); err != nil {
		fmt.Println("Failed to start tailer consumer: ", err)
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Println("Received stop signal")
	case err := <-tailer.GetErrorChan():
		fmt.Println("Tailer exits on error: ", err)
	}
}
