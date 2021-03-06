package gsmtcp

type AlreadyConnectedErr struct {
}

func (e AlreadyConnectedErr) Error() string {
	return "already connected"
}

type TimedOutErr struct {
}

func (e TimedOutErr) Error() string {
	return "timed out"
}

type NotReadyErr struct {
}

func (e NotReadyErr) Error() string {
	return "not ready"
}

type MaxBytesErr struct {
}

func (e MaxBytesErr) Error() string {
	return "maximum packet size reached"
}
