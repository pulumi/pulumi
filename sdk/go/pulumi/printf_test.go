package pulumi

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testPrintf(t *testing.T, ins ...interface{}) {
	const f = "%v %v %v"
	expected := fmt.Sprintf(f, "foo", 42, true)

	// Fprintf
	buf := &bytes.Buffer{}
	out := Output(Fprintf(buf, f, ins...))
	_, known, secret, err := await(out)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, err)
	assert.Equal(t, expected, buf.String())

	// Sprintf
	out = Sprintf(f, ins...)
	v, known, secret, err := await(out)
	assert.False(t, secret)
	assert.True(t, known)
	assert.Nil(t, err)
	assert.Equal(t, expected, v)
}

func TestSprintfPrompt(t *testing.T) {
	testPrintf(t, "foo", 42, true)
}

func TestSprintfInputs(t *testing.T) {
	testPrintf(t, String("foo"), Int(42), Bool(true))
}

func TestSprintfOutputs(t *testing.T) {
	testPrintf(t, ToOutput("foo"), ToOutput(42), ToOutput(true))
}
