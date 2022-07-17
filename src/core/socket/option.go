package socket

// Option is used for setting an option on socket.
type Option struct {
	SetSockOpt func(int, int) error
	Opt        int
}

