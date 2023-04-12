//go:build go1.18
// +build go1.18

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func FuzzBashStackOutputWriter(f *testing.F) {
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

		file, err := os.CreateTemp(dir, "script.sh")
		require.NoError(t, err)
		defer os.Remove(file.Name())

		var buff bytes.Buffer
		w := io.MultiWriter(&buff, file)

		err = (&bashStackOutputWriter{W: w}).WriteOne("output", give)
		require.NoError(t, err)
		fmt.Fprintln(w, `echo "$output"`)
		require.NoError(t, file.Close())

		got, err := exec.Command(bash, file.Name()).Output()
		if !assert.NoError(t, err, "Failed script:\n%s", buff.String()) {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
				t.Logf("stderr:\n%s", exitErr.Stderr)
			}
		}
		assert.Equal(t, give+"\n", string(got))
	})
}

func FuzzPowershellStackOutputWriter(f *testing.F) {
	pwsh, err := exec.LookPath("powershell")
	if err != nil {
		f.Skipf("Skipping: no 'powershell' found: %v", err)
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

		// It's important that this file's name end with .ps1
		// or Powershell will refuse to run it.
		file, err := os.CreateTemp(dir, "script.*.ps1")
		require.NoError(t, err)
		defer os.Remove(file.Name())

		var buff bytes.Buffer
		w := io.MultiWriter(&buff, file)

		err = (&powershellStackOutputWriter{W: w}).WriteOne("output", give)
		require.NoError(t, err)
		fmt.Fprintln(w, `echo "$output"`)
		require.NoError(t, file.Close())

		got, err := exec.Command(pwsh, "-File", file.Name()).Output()
		if !assert.NoError(t, err, "Failed script:\n%s", buff.String()) {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
				t.Logf("stderr:\n%s", exitErr.Stderr)
			}
		}

		// Strip CRLF separately so that we handle both cases:
		// ends with CRLF and ends with LF.
		got = bytes.TrimSuffix(got, []byte{'\n'})
		got = bytes.TrimSuffix(got, []byte{'\r'})
		assert.Equal(t, give, string(got))
	})
}
