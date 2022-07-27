package core

type IOEvent = uint32

// 当事件处理完成后，需要进行的动作
type Action int

const (
	// None indicates that no action should occur following an events.
	None Action = iota

	// Close closes the connection.
	Close

	// Shutdown shutdowns the server.
	Shutdown
)

type EventHandler interface {
	OnOpened(c Conn) (out []byte, action Action)

	OnInitComplete(server *Server) (action Action)

	OnShutdown(server *Server)

	OnClosed(c Conn, err error) (action Action)

	// 将数据写入socket之前触发
	PreWrite(c Conn)

	AfterWrite(c Conn, b []byte)

	React(packet []byte, c Conn) (out []byte, action Action)
}
