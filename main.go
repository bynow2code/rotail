package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	path := "/Users/changqianqian/GolandProjects/rotail/logs"
	//dirPath := "test.log"
	//dirPath := "./logs"

	tailDir(path)
}

func tailDir(path string) {
	t, err := NewDirTailer(path, WithExt([]string{".log"}))
	if err != nil {
		log.Fatalln(err)
	}

	if err := t.Start(); err != nil {
		log.Fatalln(err)
	}

	go func() {
		for line := range t.LineCh {
			fmt.Println("行：", line)
		}
	}()

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("收到停止信号")
		t.Stop()
		fmt.Println("信号退出")
	case err := <-t.ErrCh:
		fmt.Println("发生错误退出", err)
	}
}

func tailFile(path string) {
	t, err := NewFileTailer(path)
	if err != nil {
		log.Fatalln(err)
	}

	if err := t.Start(); err != nil {
		log.Fatalln(err)
	}

	go func() {
		for line := range t.LineCh {
			fmt.Println("行：", line)
		}
	}()

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("收到停止信号")
		t.Stop()
		fmt.Println("信号退出")
	case err := <-t.ErrCh:
		fmt.Println("发生错误退出", err)
	}
}
