package display

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testProgressEvents(t *testing.T, path string, accept, interactive bool, width, height int, raw bool) {
	events, err := loadEvents(path)
	require.NoError(t, err)

	suffix := ".non-interactive"
	if interactive {
		suffix = fmt.Sprintf(".interactive-%vx%v", width, height)
		if !raw {
			suffix += "-cooked"
		}
	}

	var expectedStdout []byte
	var expectedStderr []byte
	if !accept {
		expectedStdout, err = os.ReadFile(path + suffix + ".stdout.txt")
		require.NoError(t, err)

		expectedStderr, err = os.ReadFile(path + suffix + ".stderr.txt")
		require.NoError(t, err)
	}

	eventChannel, doneChannel := make(chan engine.Event), make(chan bool)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	go ShowProgressEvents("test", "update", "stack", "project", eventChannel, doneChannel, Options{
		IsInteractive:        interactive,
		Color:                colors.Raw,
		ShowConfig:           true,
		ShowReplacementSteps: true,
		ShowSameResources:    true,
		ShowReads:            true,
		Stdout:               &stdout,
		Stderr:               &stderr,
		term:                 terminal.NewMockTerminal(&stdout, width, height, true),
		deterministicOutput:  true,
	}, false)

	for _, e := range events {
		eventChannel <- e
	}
	<-doneChannel

	if !accept {
		assert.Equal(t, string(expectedStdout), string(stdout.Bytes()))
		assert.Equal(t, string(expectedStderr), string(stderr.Bytes()))
	} else {
		err = os.WriteFile(path+suffix+".stdout.txt", stdout.Bytes(), 0600)
		require.NoError(t, err)

		err = os.WriteFile(path+suffix+".stderr.txt", stderr.Bytes(), 0600)
		require.NoError(t, err)
	}
}

func TestProgressEvents(t *testing.T) {
	t.Parallel()

	accept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))

	entries, err := os.ReadDir("testdata/not-truncated")
	require.NoError(t, err)

	dimensions := []struct{ width, height int }{
		{width: 80, height: 24},
		{width: 100, height: 80},
		{width: 200, height: 80},
	}

	//nolint:paralleltest
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join("testdata/not-truncated", entry.Name())

		t.Run(entry.Name()+"interactive", func(t *testing.T) {
			t.Parallel()

			for _, dim := range dimensions {
				width, height := dim.width, dim.height
				t.Run(fmt.Sprintf("%vx%v", width, height), func(t *testing.T) {
					t.Parallel()

					t.Run("raw", func(t *testing.T) {
						testProgressEvents(t, path, accept, true, width, height, true)
					})

					t.Run("cooked", func(t *testing.T) {
						testProgressEvents(t, path, accept, true, width, height, false)
					})
				})
			}
		})

		t.Run(entry.Name()+"non-interactive", func(t *testing.T) {
			t.Parallel()

			testProgressEvents(t, path, accept, false, 80, 24, false)
		})
	}
}
