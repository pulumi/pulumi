//go:build go1.18
// +build go1.18

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func FuzzShellStackOutputWriter(f *testing.F) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		f.Skipf("Skipping: no 'bash' found: %v", err)
	}

	dir := f.TempDir()

	f.Add("foo")
	f.Add(`"bar"`)
	f.Add(`'baz'`)
	f.Add(`foo \"bar'\\baz`)
	f.Fuzz(func(t *testing.T, give string) {
		// Only consider valid unicode strings
		// that do ot contain the null character.
		if strings.IndexByte(give, 0) >= 0 || !utf8.Valid([]byte(give)) {
			t.Skip()
		}

		f, err := os.CreateTemp(dir, "script.sh")
		require.NoError(t, err)
		defer os.Remove(f.Name())

		err = (&shellStackOutputWriter{W: f}).WriteOne("output", give)
		require.NoError(t, err)
		fmt.Fprintln(f, `echo "$output"`)
		require.NoError(t, f.Close())

		got, err := exec.Command(bash, f.Name()).Output()
		require.NoError(t, err)

		assert.Equal(t, give+"\n", string(got))
	})
}
