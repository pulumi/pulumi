// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressReportingCloser(t *testing.T) {
	t.Parallel()

	// Arrange.
	events := make(chan Event)
	done := make(chan bool)

	size := 1024
	read := 512

	eventEmitter := eventEmitter{ch: events}
	closer := NewProgressReportingCloser(
		eventEmitter,
		PluginDownload,
		"test-id",
		"Test message",
		int64(size),
		0, /*reportingInterval*/
		&constantReadCloser{read: read},
	)

	buf := make([]byte, size)

	var payload ProgressEventPayload
	f := func() {
		e := <-events
		payload = e.Payload().(ProgressEventPayload)
		done <- true
	}

	go f()

	// Act.
	n, err := closer.Read(buf)

	// Assert.
	<-done

	assert.Equal(t, read, n)
	assert.NoError(t, err)

	assert.Equal(t, PluginDownload, payload.Type)
	assert.Equal(t, "test-id", payload.ID)
	assert.Equal(t, "Test message", payload.Message)
	assert.Equal(t, int64(read), payload.Completed)
	assert.Equal(t, int64(size), payload.Total)
}

type constantReadCloser struct {
	read int
}

func (c *constantReadCloser) Read(p []byte) (n int, err error) {
	return c.read, nil
}

func (c *constantReadCloser) Close() error {
	return nil
}
