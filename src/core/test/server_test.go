package test

import (
	"greactor/src/core"
	"testing"
)

type testServer struct {
	core.EventServer
}

func (es *testServer) React(frame []byte, c core.Conn) (out []byte, action core.Action) {
	out = frame
	return
}

func TestServer(test *testing.T) {
	echo := new(testServer)
	opts := new(core.Options)
	opts.Multicore = true

	s, _ := core.NewServer(echo, "tcp://:8332", opts)
	s.Run()
}
