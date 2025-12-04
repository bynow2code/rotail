package main

import (
	"fmt"
)

func main() {
	filePath := "test.log"

	t := NewFileTailer(filePath)
	if err := t.Start(); err != nil {
		fmt.Println(err)
		return
	}
	defer t.Close()

	for {
		select {
		case line, ok := <-t.LineCh:
			if !ok {
				return
			}
			fmt.Println(line)
		case err, ok := <-t.ErrCh:
			if !ok {
				return
			}
			fmt.Println(err)
			return
		}
	}
}
