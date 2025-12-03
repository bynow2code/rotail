package main

func main() {
	//filePath := "test.log"
	//stopCh := make(chan struct{})
	//tailFile(filePath, stopCh)

	dirPath := "/Users/changqianqian/GolandProjects/rotail/logs"
	tailDir(dirPath)
}
