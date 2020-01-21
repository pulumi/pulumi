package pulumi

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSprintfPrompt(t *testing.T) {
	out := Sprintf("%v %v %v", "foo", 42, true)
	v, known, err := await(out)
	assert.True(t, known)
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%v %v %v", "foo", 42, true), v)
}

func TestSprintfInputs(t *testing.T) {
	out := Sprintf("%v %v %v", String("foo"), Int(42), Bool(true))
	v, known, err := await(out)
	assert.True(t, known)
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%v %v %v", "foo", 42, true), v)
}

func TestSprintfOutputs(t *testing.T) {
	out := Sprintf("%v %v %v", ToOutput("foo"), ToOutput(42), ToOutput(true))
	v, known, err := await(out)
	assert.True(t, known)
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%v %v %v", "foo", 42, true), v)
}
