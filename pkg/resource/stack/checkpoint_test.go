package stack

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadV0Checkpoint(t *testing.T) {
	bytes, err := ioutil.ReadFile("testdata/checkpoint-v0.json")
	assert.NoError(t, err)

	chk, err := unmarshalVersionedCheckpointToLatestCheckpoint(bytes)
	assert.NoError(t, err)
	assert.NotNil(t, chk.Latest)
	assert.Len(t, chk.Latest.Resources, 30)
}

func TestLoadV1Checkpoint(t *testing.T) {
	bytes, err := ioutil.ReadFile("testdata/checkpoint-v1.json")
	assert.NoError(t, err)

	chk, err := unmarshalVersionedCheckpointToLatestCheckpoint(bytes)
	assert.NoError(t, err)
	assert.NotNil(t, chk.Latest)
	assert.Len(t, chk.Latest.Resources, 30)
}
