package events

import (
	"greactor/src/core"
)

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

type (
	EventHandler interface {
		OnOpened(c core.Conn) (out []byte, action Action)
		OnInitComplete(server *core.Server) (action Action)

		OnShutdown(server *core.Server)

		OnClosed(c core.Conn, err error) (action Action)

		// 将数据写入socket之前触发
		PreWrite(c core.Conn)

		AfterWrite(c core.Conn, b []byte)

		React(packet []byte, c core.Conn) (out []byte, action Action)
	}

	// EventServer is a built-in implementation of EventHandler which sets up each method with a default implementation,
	// you can compose it with your own implementation of EventHandler when you don't want to implement all methods
	// in EventHandler.
	EventServer struct{}
)
