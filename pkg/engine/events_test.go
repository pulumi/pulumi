package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrySendEvent(t *testing.T) {
	e := Event{}
	c := make(chan Event, 100)
	assert.Equal(t, true, trySendEvent(c, e))
	close(c)
	assert.Equal(t, false, trySendEvent(c, e))
}

func TestTryCloseEventChan(t *testing.T) {
	c := make(chan Event, 100)
	assert.Equal(t, true, tryCloseEventChan(c))
	assert.Equal(t, false, tryCloseEventChan(c))
}
