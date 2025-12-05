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
