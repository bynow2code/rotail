package tail

type Tailer interface {
	Start() error
	Close()
	GetLineChan() <-chan string
	GetErrorChan() <-chan error
}
