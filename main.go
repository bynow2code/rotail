package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bynow2code/rotail/internal/tail"
)

var version = "0.0.0-dev"

func main() {
	fileTailer, err := tail.NewFileTailer("./test.log")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer fileTailer.Close()

	if err := fileTailer.Start(); err != nil {
		fmt.Println(err)
		return
	}

	go func() {
		for {
			select {
			case line, ok := <-fileTailer.GetLineChan():
				if !ok {
					return
				}
				fmt.Println(line)
			case err, ok := <-fileTailer.GetErrorChan():
				if !ok {
					return
				}
				fmt.Println(err)
				return
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("Received stop signal")
	}

	//file := flag.String("f", "", "File path to tail (e.g. /var/log/app.log)")
	//dir := flag.String("d", "", "Directory path to tail (e.g. /var/log)")
	//ext := flag.String("ext", ".log", "Comma-separated file extensions, default .log (e.g. .log,.txt)")
	//ver := flag.Bool("v", false, "Show version")
	//help := flag.Bool("h", false, "Show help")
	//
	//flag.Parse()
	//if *help {
	//	flag.PrintDefaults()
	//	os.Exit(0)
	//}
	//
	//switch {
	//case *file != "":
	//	t, err := tail.NewFileTailer(*file)
	//	if err != nil {
	//		fmt.Printf("Exited due to error: %ver\n", err)
	//		return
	//	}
	//	runTailer(t)
	//case *dir != "":
	//	exts := strings.Split(*ext, ",")
	//	t, err := tail.NewDir(*dir, tail.WithExt(exts))
	//	if err != nil {
	//		fmt.Printf("Exited due to error: %ver\n", err)
	//		return
	//	}
	//	runTailer(t)
	//case *ver:
	//	fmt.Println(version)
	//case *help:
	//	flag.PrintDefaults()
	//default:
	//	flag.PrintDefaults()
	//}
}

func runTailer(t tail.Tailer) {
	if err := t.Start(); err != nil {
		fmt.Printf("Exited due to error: %v\n", err)
		return
	}

	go func() {
		for line := range t.GetLineCh() {
			fmt.Println(line)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("Received stop signal")
		t.Stop()
		fmt.Println("Exited via signal")
	case err, ok := <-t.GetErrCh():
		if !ok {
			return
		}
		fmt.Printf("Exited due to error: %v\n", err)
	}
}
