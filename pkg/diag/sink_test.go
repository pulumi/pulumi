// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package diag

import (
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounts(t *testing.T) {
	t.Parallel()

	// Create a new default sink with /dev/null writers to avoid spamming the test log.
	sink := newDefaultSink(FormatOptions{}, map[Category]io.Writer{
		Info:    ioutil.Discard,
		Error:   ioutil.Discard,
		Warning: ioutil.Discard,
	})

	const numEach = 10

	for i := 0; i < numEach; i++ {
		assert.Equal(t, sink.Infos(), 0, "expected infos pre to stay at zero")
		assert.Equal(t, sink.Errors(), 0, "expected errors pre to stay at zero")
		assert.Equal(t, sink.Warnings(), i, "expected warnings pre to be at iteration count")
		sink.Warningf(&Diag{Message: "A test of the emergency warning system: %v."}, i)
		assert.Equal(t, sink.Infos(), 0, "expected infos post to stay at zero")
		assert.Equal(t, sink.Errors(), 0, "expected errors post to stay at zero")
		assert.Equal(t, sink.Warnings(), i+1, "expected warnings post to be at iteration count+1")
	}

	for i := 0; i < numEach; i++ {
		assert.Equal(t, sink.Infos(), 0, "expected infos pre to stay at zero")
		assert.Equal(t, sink.Errors(), i, "expected errors pre to be at iteration count")
		assert.Equal(t, sink.Warnings(), numEach, "expected warnings pre to stay at numEach")
		sink.Errorf(&Diag{Message: "A test of the emergency error system: %v."}, i)
		assert.Equal(t, sink.Infos(), 0, "expected infos post to stay at zero")
		assert.Equal(t, sink.Errors(), i+1, "expected errors post to be at iteration count+1")
		assert.Equal(t, sink.Warnings(), numEach, "expected warnings post to stay at numEach")
	}
}
