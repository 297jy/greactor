package errors

import "errors"

var (
	// ErrServerShutdown occurs when server is closing.
	ErrServerShutdown = errors.New("server is going to be shutdown")
	// ErrServerInShutdown occurs when attempting to shut the server down more than once.
	ErrServerInShutdown = errors.New("server is already in shutdown")
	// ErrAcceptSocket occurs when acceptor does not accept the new connection properly.
	ErrAcceptSocket = errors.New("accept a new connection error")
	// ErrTooManyEventLoopThreads occurs when attempting to set up more than 10,000 events-loop goroutines under LockOSThread mode.
	ErrTooManyEventLoopThreads = errors.New("too many events-loops under LockOSThread mode")
	// ErrUnsupportedProtocol occurs when trying to use protocol that is not supported.
	ErrUnsupportedProtocol = errors.New("only unix, tcp/tcp4/tcp6, udp/udp4/udp6 are supported")
	// ErrUnsupportedTCPProtocol occurs when trying to use an unsupported TCP protocol.
	ErrUnsupportedTCPProtocol = errors.New("only tcp/tcp4/tcp6 are supported")
	// ErrUnsupportedUDPProtocol occurs when trying to use an unsupported UDP protocol.
	ErrUnsupportedUDPProtocol = errors.New("only udp/udp4/udp6 are supported")
	// ErrUnsupportedUDSProtocol occurs when trying to use an unsupported Unix protocol.
	ErrUnsupportedUDSProtocol = errors.New("only unix is supported")
	// ErrUnsupportedPlatform occurs when running gnet on an unsupported platform.
	ErrUnsupportedPlatform = errors.New("unsupported platform in gnet")
	// ErrConnectionClosed occurs when the events-loop receives a closed connection.
	ErrConnectionClosed = errors.New("connection is closed")

	// ================================================= icodecs errors =================================================.

	// ErrIncompletePacket occurs when there is an incomplete packet under TCP protocol.
	ErrIncompletePacket = errors.New("incomplete packet")
	// ErrInvalidFixedLength occurs when the output data have invalid fixed length.
	ErrInvalidFixedLength = errors.New("invalid fixed length of bytes")
	// ErrUnexpectedEOF occurs when no enough data to read by icodecs.
	ErrUnexpectedEOF = errors.New("there is no enough data")
	// ErrUnsupportedLength occurs when unsupported lengthFieldLength is from input data.
	ErrUnsupportedLength = errors.New("unsupported lengthFieldLength. (expected: 1, 2, 3, 4, or 8)")
	// ErrTooLessLength occurs when adjusted frame length is less than zero.
	ErrTooLessLength = errors.New("adjusted frame length is less than zero")
)