package run

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var version = "0.0.0-dev"

type Config struct {
	File       string   // 文件路径
	Dir        string   // 目录路径
	Extensions []string // 文件拓展名
}

func ParseFlags() (*Config, error) {
	file := flag.String("f", "", "File path to tail (e.g. /var/log/app.log)")
	dir := flag.String("d", "", "Directory path to tail (e.g. /var/log)")
	ext := flag.String("ext", ".log", "Comma-separated file Extensions, default .log (e.g. .log,.txt)")
	ver := flag.Bool("v", false, "Show version")

	flag.Usage = func() {
		fmt.Println("Welcome to rotail!")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *ver {
		fmt.Println("rotail version:", version)
		os.Exit(0)
	}

	// 参数校验
	if *file == "" && *dir == "" {
		return nil, fmt.Errorf("must specify -f or -d")
	}

	return &Config{
		File:       *file,
		Dir:        *dir,
		Extensions: strings.Split(*ext, ","),
	}, nil
}
