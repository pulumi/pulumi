// Copyright 2016 Marapongo, Inc. All rights reserved.

package diag

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounts(t *testing.T) {
	// Create a new default sink with /dev/null writers to avoid spamming the test log.
	sink := newDefaultSink(FormatOptions{}, ioutil.Discard, ioutil.Discard)

	const numEach = 10

	for i := 0; i < numEach; i++ {
		assert.Equal(t, sink.Errors(), 0, "expected errors pre to stay at zero")
		assert.Equal(t, sink.Warnings(), i, "expected warnings pre to be at iteration count")
		sink.Warningf(&Diag{Message: "A test of the emergency warning system: %v."}, i)
		assert.Equal(t, sink.Errors(), 0, "expected errors post to stay at zero")
		assert.Equal(t, sink.Warnings(), i+1, "expected warnings post to be at iteration count+1")
	}

	for i := 0; i < numEach; i++ {
		assert.Equal(t, sink.Errors(), i, "expected errors pre to be at iteration count")
		assert.Equal(t, sink.Warnings(), numEach, "expected warnings pre to stay at numEach")
		sink.Errorf(&Diag{Message: "A test of the emergency error system: %v."}, i)
		assert.Equal(t, sink.Errors(), i+1, "expected errors post to be at iteration count+1")
		assert.Equal(t, sink.Warnings(), numEach, "expected warnings post to stay at numEach")
	}
}
