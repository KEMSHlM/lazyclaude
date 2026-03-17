package context_test

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/gui/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testContext tracks OnFocus/OnBlur calls.
type testContext struct {
	context.BaseContext
	focusCount int
	blurCount  int
}

func newTestContext(name string) *testContext {
	return &testContext{
		BaseContext: context.NewBaseContext(name, context.KindSide),
	}
}

func (c *testContext) OnFocus() { c.focusCount++ }
func (c *testContext) OnBlur()  { c.blurCount++ }

func TestManager_PushAndCurrent(t *testing.T) {
	t.Parallel()
	m := context.NewManager()

	assert.Nil(t, m.Current())
	assert.Equal(t, 0, m.Depth())

	ctx1 := newTestContext("sessions")
	m.Push(ctx1)

	assert.Equal(t, ctx1, m.Current())
	assert.Equal(t, 1, m.Depth())
	assert.Equal(t, 1, ctx1.focusCount)
}

func TestManager_Push_BlursPrevious(t *testing.T) {
	t.Parallel()
	m := context.NewManager()

	ctx1 := newTestContext("sessions")
	ctx2 := newTestContext("diff")

	m.Push(ctx1)
	assert.Equal(t, 1, ctx1.focusCount)
	assert.Equal(t, 0, ctx1.blurCount)

	m.Push(ctx2)
	assert.Equal(t, 1, ctx1.blurCount) // ctx1 was blurred
	assert.Equal(t, 1, ctx2.focusCount)
	assert.Equal(t, ctx2, m.Current())
	assert.Equal(t, 2, m.Depth())
}

func TestManager_Pop(t *testing.T) {
	t.Parallel()
	m := context.NewManager()

	ctx1 := newTestContext("sessions")
	ctx2 := newTestContext("diff")

	m.Push(ctx1)
	m.Push(ctx2)

	popped, err := m.Pop()
	require.NoError(t, err)
	assert.Equal(t, ctx2, popped)
	assert.Equal(t, 1, ctx2.blurCount) // popped context was blurred
	assert.Equal(t, 2, ctx1.focusCount) // bottom context got re-focused
	assert.Equal(t, ctx1, m.Current())
}

func TestManager_Pop_Empty(t *testing.T) {
	t.Parallel()
	m := context.NewManager()

	_, err := m.Pop()
	assert.Error(t, err)
}

func TestManager_Pop_LastItem(t *testing.T) {
	t.Parallel()
	m := context.NewManager()

	ctx := newTestContext("only")
	m.Push(ctx)

	popped, err := m.Pop()
	require.NoError(t, err)
	assert.Equal(t, ctx, popped)
	assert.Nil(t, m.Current())
	assert.Equal(t, 0, m.Depth())
}

func TestManager_Replace(t *testing.T) {
	t.Parallel()
	m := context.NewManager()

	ctx1 := newTestContext("sessions")
	ctx2 := newTestContext("diff")

	m.Push(ctx1)
	err := m.Replace(ctx2)
	require.NoError(t, err)

	assert.Equal(t, ctx2, m.Current())
	assert.Equal(t, 1, m.Depth())
	assert.Equal(t, 1, ctx1.blurCount) // old was blurred
	assert.Equal(t, 1, ctx2.focusCount)
}

func TestManager_Replace_Empty(t *testing.T) {
	t.Parallel()
	m := context.NewManager()

	err := m.Replace(newTestContext("x"))
	assert.Error(t, err)
}

func TestManager_DeepStack(t *testing.T) {
	t.Parallel()
	m := context.NewManager()

	ctx1 := newTestContext("main")
	ctx2 := newTestContext("popup1")
	ctx3 := newTestContext("popup2")

	m.Push(ctx1)
	m.Push(ctx2)
	m.Push(ctx3)
	assert.Equal(t, 3, m.Depth())
	assert.Equal(t, ctx3, m.Current())

	m.Pop()
	assert.Equal(t, ctx2, m.Current())

	m.Pop()
	assert.Equal(t, ctx1, m.Current())

	m.Pop()
	assert.Nil(t, m.Current())
}