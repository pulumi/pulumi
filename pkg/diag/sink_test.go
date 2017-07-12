// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package diag

import (
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func discardSink() Sink {
	// Create a new default sink with /dev/null writers to avoid spamming the test log.
	return newDefaultSink(FormatOptions{}, map[Severity]io.Writer{
		Info:    ioutil.Discard,
		Error:   ioutil.Discard,
		Warning: ioutil.Discard,
	})
}

func TestCounts(t *testing.T) {
	t.Parallel()

	sink := discardSink()

	const numEach = 10

	for i := 0; i < numEach; i++ {
		assert.Equal(t, sink.Debugs(), 0, "expected debugs pre to stay at zero")
		assert.Equal(t, sink.Infos(), 0, "expected infos pre to stay at zero")
		assert.Equal(t, sink.Errors(), 0, "expected errors pre to stay at zero")
		assert.Equal(t, sink.Warnings(), i, "expected warnings pre to be at iteration count")
		sink.Warningf(&Diag{Message: "A test of the emergency warning system: %v."}, i)
		assert.Equal(t, sink.Infos(), 0, "expected infos post to stay at zero")
		assert.Equal(t, sink.Errors(), 0, "expected errors post to stay at zero")
		assert.Equal(t, sink.Warnings(), i+1, "expected warnings post to be at iteration count+1")
	}

	for i := 0; i < numEach; i++ {
		assert.Equal(t, sink.Debugs(), 0, "expected debugs pre to stay at zero")
		assert.Equal(t, sink.Infos(), 0, "expected infos pre to stay at zero")
		assert.Equal(t, sink.Errors(), i, "expected errors pre to be at iteration count")
		assert.Equal(t, sink.Warnings(), numEach, "expected warnings pre to stay at numEach")
		sink.Errorf(&Diag{Message: "A test of the emergency error system: %v."}, i)
		assert.Equal(t, sink.Debugs(), 0, "expected deugs post to stay at zero")
		assert.Equal(t, sink.Infos(), 0, "expected infos post to stay at zero")
		assert.Equal(t, sink.Errors(), i+1, "expected errors post to be at iteration count+1")
		assert.Equal(t, sink.Warnings(), numEach, "expected warnings post to stay at numEach")
	}
}

// TestEscape ensures that arguments containing format-like characters aren't interpreted as such.
func TestEscape(t *testing.T) {
	t.Parallel()

	sink := discardSink()

	// Passing % chars in the argument should not yield %!(MISSING)s.
	s := sink.Stringify(Error, Message("%s"), "lots of %v %s %d chars")
	assert.Equal(t, "error: lots of %v %s %d chars\n", s)

	// Passing % chars in the format string, on the other hand, should.
	smiss := sink.Stringify(Error, Message("lots of %v %s %d chars"))
	assert.Equal(t, "error: lots of %!v(MISSING) %!s(MISSING) %!d(MISSING) chars\n", smiss)
}
