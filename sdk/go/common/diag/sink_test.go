// Copyright 2016-2018, Pulumi Corporation.
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

package diag

import (
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

func discardSink() Sink {
	// Create a new default sink with /dev/null writers to avoid spamming the test log.
	return newDefaultSink(FormatOptions{Color: colors.Never}, map[Severity]io.Writer{
		Debug:   ioutil.Discard,
		Info:    ioutil.Discard,
		Infoerr: ioutil.Discard,
		Error:   ioutil.Discard,
		Warning: ioutil.Discard,
	})
}

func TestCounts(t *testing.T) {
	t.Parallel()

	sink := discardSink()

	const numEach = 10

	for i := 0; i < numEach; i++ {
		sink.Warningf(&Diag{Message: "A test of the emergency warning system: %v."}, i)
	}

	for i := 0; i < numEach; i++ {
		sink.Errorf(&Diag{Message: "A test of the emergency error system: %v."}, i)
	}
}

// TestEscape ensures that arguments containing format-like characters aren't interpreted as such.
func TestEscape(t *testing.T) {
	t.Parallel()

	sink := discardSink()

	// Passing % chars in the argument should not yield %!(MISSING)s.
	p, s := sink.Stringify(Error, Message("", "%s"), "lots of %v %s %d chars")
	assert.Equal(t, "error: lots of %v %s %d chars\n", p+s)

	// Passing % chars in the format string, on the other hand, should.
	pmiss, smiss := sink.Stringify(Error, Message("", "lots of %v %s %d chars"))
	assert.Equal(t, "error: lots of %!v(MISSING) %!s(MISSING) %!d(MISSING) chars\n", pmiss+smiss)
}
