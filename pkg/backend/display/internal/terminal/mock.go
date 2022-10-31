package terminal

import (
	"io"
	"sync"
)

type MockTerminal struct {
	m sync.Mutex

	width, height int
	info          Info

	keys chan string

	dest io.Writer
}

func NewMockTerminal(dest io.Writer, width, height int) *MockTerminal {
	return &MockTerminal{
		width:  width,
		height: height,
		info:   info{noTermInfo(0)},
		keys:   make(chan string),
		dest:   dest,
	}
}

func (t *MockTerminal) Close() error {
	close(t.keys)
	return nil
}

func (t *MockTerminal) Size() (width, height int, err error) {
	t.m.Lock()
	defer t.m.Unlock()

	return t.width, t.height, nil
}

func (t *MockTerminal) Write(b []byte) (int, error) {
	return t.dest.Write(b)
}

func (t *MockTerminal) ClearLine() {
	t.info.ClearLine(t)
}

func (t *MockTerminal) CursorUp(count int) {
	t.info.CursorUp(t, count)
}

func (t *MockTerminal) CursorDown(count int) {
	t.info.CursorDown(t, count)
}

func (t *MockTerminal) ReadKey() (string, error) {
	k, ok := <-t.keys
	if !ok {
		return "", io.EOF
	}
	return k, nil
}

func (t *MockTerminal) SetSize(width, height int) {
	t.m.Lock()
	defer t.m.Unlock()

	t.width, t.height = width, height
}

func (t *MockTerminal) SendKey(key string) {
	t.keys <- key
}
