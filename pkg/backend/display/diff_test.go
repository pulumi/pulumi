package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadEvents(path string) (events []engine.Event, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening '%v': %w", path, err)
	}
	defer contract.IgnoreClose(f)

	dec := json.NewDecoder(f)
	for {
		var jsonEvent apitype.EngineEvent
		if err = dec.Decode(&jsonEvent); err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("decoding event %d: %w", len(events), err)
		}

		event, err := ConvertJSONEvent(jsonEvent)
		if err != nil {
			return nil, fmt.Errorf("converting event %d: %w", len(events), err)
		}
		events = append(events, event)
	}

	// If there are no events or if the event stream does not terminate with a cancel event,
	// synthesize one here.
	if len(events) == 0 || events[len(events)-1].Type != engine.CancelEvent {
		events = append(events, engine.NewEvent(engine.CancelEvent, nil))
	}

	return events, nil
}

func testDiffEvents(t *testing.T, path string, accept bool, truncateOutput bool) {
	events, err := loadEvents(path)
	require.NoError(t, err)

	var expectedStdout []byte
	var expectedStderr []byte
	if !accept {
		expectedStdout, err = os.ReadFile(path + ".stdout.txt")
		require.NoError(t, err)

		expectedStderr, err = os.ReadFile(path + ".stderr.txt")
		require.NoError(t, err)
	}

	eventChannel, doneChannel := make(chan engine.Event), make(chan bool)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	go ShowDiffEvents("test", eventChannel, doneChannel, Options{
		Color:                colors.Raw,
		ShowConfig:           true,
		ShowReplacementSteps: true,
		ShowSameResources:    true,
		ShowReads:            true,
		TruncateOutput:       truncateOutput,
		Stdout:               &stdout,
		Stderr:               &stderr,
	})

	for _, e := range events {
		eventChannel <- e
	}
	<-doneChannel

	if !accept {
		assert.Equal(t, string(expectedStdout), string(stdout.Bytes()))
		assert.Equal(t, string(expectedStderr), string(stderr.Bytes()))
	} else {
		err = os.WriteFile(path+".stdout.txt", stdout.Bytes(), 0600)
		require.NoError(t, err)

		err = os.WriteFile(path+".stderr.txt", stderr.Bytes(), 0600)
		require.NoError(t, err)
	}
}

func TestDiffEvents(t *testing.T) {
	t.Parallel()

	accept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))

	entries, err := os.ReadDir("testdata")
	require.NoError(t, err)

	//nolint:paralleltest
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join("testdata", entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()

			testDiffEvents(t, path, accept, false)
		})
	}
}
