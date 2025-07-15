// Copyright 2016-2023, Pulumi Corporation.
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

package iotest

import (
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogWriter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc   string
		prefix string // prefix string, if any

		writes []string // individual write calls
		want   []string // expected log output
	}{
		{
			desc:   "empty strings",
			writes: []string{"", "", ""},
		},
		{
			desc:   "no newline",
			writes: []string{"foo", "bar", "baz"},
			want:   []string{"foobarbaz"},
		},
		{
			desc: "newline separated",
			writes: []string{
				"foo\n",
				"bar\n",
				"baz\n\n",
				"qux",
			},
			want: []string{
				"foo",
				"bar",
				"baz",
				"",
				"qux",
			},
		},
		{
			desc:   "partial line",
			writes: []string{"foo", "bar\nbazqux"},
			want: []string{
				"foobar",
				"bazqux",
			},
		},
		{
			desc:   "prefixed/empty strings",
			prefix: "out: ",
			writes: []string{"", "", ""},
		},
		{
			desc:   "prefixed/no newline",
			prefix: "out: ",
			writes: []string{"foo", "bar", "baz"},
			want:   []string{"out: foobarbaz"},
		},
		{
			desc:   "prefixed/newline separated",
			prefix: "out: ",
			writes: []string{
				"foo\n",
				"bar\n",
				"baz\n\n",
				"qux",
			},
			want: []string{
				"out: foo",
				"out: bar",
				"out: baz",
				"out: ",
				"out: qux",
			},
		},
		{
			desc:   "prefixed/partial line",
			prefix: "out: ",
			writes: []string{"foo", "bar\nbazqux"},
			want: []string{
				"out: foobar",
				"out: bazqux",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			fakeT := fakeT{TB: t}
			w := LogWriterPrefixed(&fakeT, tt.prefix)

			for _, input := range tt.writes {
				n, err := w.Write([]byte(input))
				require.NoError(t, err)
				assert.Equal(t, len(input), n)
			}

			fakeT.runCleanup()

			assert.Equal(t, tt.want, fakeT.msgs)
		})
	}
}

// Ensures that there are no data races in LogWriter
// by writing to it from multiple concurrent goroutines.
// 'go test -race' will explode if there's a data race.
func TestLogWriterRace(t *testing.T) {
	t.Parallel()

	const N = 100 // number of concurrent writers

	fakeT := fakeT{TB: t}
	w := LogWriter(&fakeT)

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()

			_, err := io.WriteString(w, "foo\n")
			require.NoError(t, err)
			_, err = io.WriteString(w, "bar\n")
			require.NoError(t, err)
			_, err = io.WriteString(w, "baz\n")
			require.NoError(t, err)
		}()
	}
	wg.Wait()
}

// Wraps a testing.TB and intercepts log messages.
type fakeT struct {
	testing.TB

	msgs     []string
	cleanups []func()
}

func (t *fakeT) Logf(msg string, args ...interface{}) {
	t.msgs = append(t.msgs, fmt.Sprintf(msg, args...))
}

func (t *fakeT) Cleanup(f func()) {
	t.cleanups = append(t.cleanups, f)
}

func (t *fakeT) runCleanup() {
	// cleanup functions are called in reverse order.
	for i := len(t.cleanups) - 1; i >= 0; i-- {
		t.cleanups[i]()
	}
}
