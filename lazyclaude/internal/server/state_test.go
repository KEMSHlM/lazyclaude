package server_test

import (
	"testing"
	"time"

	"github.com/KEMSHlM/lazyclaude/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestState_SetAndGetConn(t *testing.T) {
	t.Parallel()
	s := server.NewState()

	s.SetConn("conn-1", &server.ConnState{PID: 1001, Window: "@1"})

	cs := s.GetConn("conn-1")
	require.NotNil(t, cs)
	assert.Equal(t, 1001, cs.PID)
	assert.Equal(t, "@1", cs.Window)
}

func TestState_GetConn_NotFound(t *testing.T) {
	t.Parallel()
	s := server.NewState()
	assert.Nil(t, s.GetConn("nonexistent"))
}

func TestState_RemoveConn(t *testing.T) {
	t.Parallel()
	s := server.NewState()

	s.SetConn("conn-1", &server.ConnState{PID: 1001, Window: "@1"})
	assert.Equal(t, 1, s.ConnCount())

	s.RemoveConn("conn-1")
	assert.Equal(t, 0, s.ConnCount())
	assert.Nil(t, s.GetConn("conn-1"))
}

func TestState_RemoveConn_ClearsPidMapping(t *testing.T) {
	t.Parallel()
	s := server.NewState()

	s.SetConn("conn-1", &server.ConnState{PID: 1001, Window: "@1"})
	assert.Equal(t, "@1", s.WindowForPID(1001))

	s.RemoveConn("conn-1")
	assert.Equal(t, "", s.WindowForPID(1001))
}

func TestState_WindowForPID(t *testing.T) {
	t.Parallel()
	s := server.NewState()

	s.SetConn("conn-1", &server.ConnState{PID: 1001, Window: "@1"})
	s.SetConn("conn-2", &server.ConnState{PID: 1002, Window: "@2"})

	assert.Equal(t, "@1", s.WindowForPID(1001))
	assert.Equal(t, "@2", s.WindowForPID(1002))
	assert.Equal(t, "", s.WindowForPID(9999))
}

func TestState_Pending_SetAndGet(t *testing.T) {
	t.Parallel()
	s := server.NewState()

	s.SetPending("@1", server.PendingTool{
		ToolName: "Bash",
		Input:    `{"command":"ls"}`,
		CWD:      "/home/user",
	})

	tool, ok := s.GetPending("@1")
	assert.True(t, ok)
	assert.Equal(t, "Bash", tool.ToolName)
	assert.Equal(t, `{"command":"ls"}`, tool.Input)
	assert.Equal(t, "/home/user", tool.CWD)
}

func TestState_Pending_ConsumedOnGet(t *testing.T) {
	t.Parallel()
	s := server.NewState()

	s.SetPending("@1", server.PendingTool{ToolName: "Bash"})

	_, ok := s.GetPending("@1")
	assert.True(t, ok)

	// Second get should return false (consumed)
	_, ok = s.GetPending("@1")
	assert.False(t, ok)
}

func TestState_Pending_Expired(t *testing.T) {
	t.Parallel()
	s := server.NewState()

	// Use SetPendingWithExpiry to set an already-expired entry
	s.SetPendingWithExpiry("@1", server.PendingTool{ToolName: "Bash"}, time.Now().Add(-1*time.Second))

	_, ok := s.GetPending("@1")
	assert.False(t, ok) // expired, should not be returned
}

func TestState_Pending_NotFound(t *testing.T) {
	t.Parallel()
	s := server.NewState()

	_, ok := s.GetPending("nonexistent")
	assert.False(t, ok)
}

func TestState_ConnCount(t *testing.T) {
	t.Parallel()
	s := server.NewState()
	assert.Equal(t, 0, s.ConnCount())

	s.SetConn("c1", &server.ConnState{PID: 1})
	s.SetConn("c2", &server.ConnState{PID: 2})
	assert.Equal(t, 2, s.ConnCount())

	s.RemoveConn("c1")
	assert.Equal(t, 1, s.ConnCount())
}

func TestState_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	s := server.NewState()
	done := make(chan struct{})

	// Writer goroutine
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			s.SetConn("conn", &server.ConnState{PID: i, Window: "@1"})
			s.SetPending("@1", server.PendingTool{ToolName: "test"})
		}
	}()

	// Reader goroutine (concurrent)
	for i := 0; i < 100; i++ {
		s.GetConn("conn")
		s.WindowForPID(i)
		s.GetPending("@1")
		s.ConnCount()
	}

	<-done
}
