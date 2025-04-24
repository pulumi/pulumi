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

package display

// Note: to regenerate the baselines for these tests, run `go test` with `PULUMI_ACCEPT=true`.

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
		events = append(events, engine.NewCancelEvent())
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
		assert.Equal(t, string(expectedStdout), stdout.String())
		assert.Equal(t, string(expectedStderr), stderr.String())
	} else {
		err = os.WriteFile(path+".stdout.txt", stdout.Bytes(), 0o600)
		require.NoError(t, err)

		err = os.WriteFile(path+".stderr.txt", stderr.Bytes(), 0o600)
		require.NoError(t, err)
	}
}

func TestDiffEvents(t *testing.T) {
	t.Parallel()

	accept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))

	entries, err := os.ReadDir("testdata/not-truncated")
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join("testdata/not-truncated", entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			testDiffEvents(t, path, accept, false)
		})
	}

	entries, err = os.ReadDir("testdata/truncated")
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join("testdata/truncated", entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			testDiffEvents(t, path, accept, true)
		})
	}
}

func TestJsonYamlDiff(t *testing.T) {
	t.Parallel()

	accept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))

	entries, err := os.ReadDir("testdata/json-yaml")
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join("testdata/json-yaml", entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			testDiffEvents(t, path, accept, false)
		})
	}
}

func assertExpectedCreateDiff(t *testing.T, path string, accept bool) {
	events, err := loadEvents(path)
	require.NoError(t, err)

	expectedPath := path + ".create-diff.txt"

	var expectedDiff []byte
	if !accept {
		expectedDiff, err = os.ReadFile(expectedPath)
		require.NoError(t, err)
	}

	diff, err := CreateDiff(events, Options{
		ShowConfig:           true,
		ShowReplacementSteps: true,
		ShowSameResources:    true,
		ShowReads:            true,
		Color:                colors.Never,
	})
	require.NoError(t, err)

	if !accept {
		assert.Equal(t, string(expectedDiff), diff)
	} else {
		err = os.WriteFile(expectedPath, []byte(diff), 0o600)
		require.NoError(t, err)
	}
}

func TestDiffEventsCreateDiff(t *testing.T) {
	t.Parallel()

	accept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))

	entries, err := os.ReadDir("testdata/not-truncated")
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join("testdata/not-truncated", entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			assertExpectedCreateDiff(t, path, accept)
		})
	}

	entries, err = os.ReadDir("testdata/truncated")
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join("testdata/truncated", entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			assertExpectedCreateDiff(t, path, accept)
		})
	}
}

func TestJsonYamlCreateDiff(t *testing.T) {
	t.Parallel()

	accept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))

	entries, err := os.ReadDir("testdata/json-yaml")
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join("testdata/json-yaml", entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			assertExpectedCreateDiff(t, path, accept)
		})
	}
}

func TestCreateDiffRequiresColor(t *testing.T) {
	t.Parallel()

	_, err := CreateDiff([]engine.Event{}, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "color must be specified")
}
