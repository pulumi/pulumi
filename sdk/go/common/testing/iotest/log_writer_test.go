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
		desc string

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
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			fakeT := fakeT{TB: t}
			w := LogWriter(&fakeT)

			for _, input := range tt.writes {
				n, err := w.Write([]byte(input))
				assert.NoError(t, err)
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
