package tail

type Tailer interface {
	Start() error
	Stop()
	GetLineCh() <-chan string
	GetErrCh() <-chan error
}
