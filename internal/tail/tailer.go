package tail

type Tailer interface {
	Producer() error
	Consumer() error
	GetErrorChan() <-chan error
	Close()
}
