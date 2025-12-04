package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	filePath := "test.log"

	t := NewFileTailer(filePath)
	if err := t.Start(); err != nil {
		log.Fatalln(err)
	}

	go func() {
		for line := range t.LineCh {
			fmt.Println("行：", line)
		}
	}()

	go func() {
		for err := range t.ErrCh {
			fmt.Println("错误", err)
		}
	}()

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("收到停止信号")
	}
}
