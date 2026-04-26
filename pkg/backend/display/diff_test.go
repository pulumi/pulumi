// Copyright 2016, Pulumi Corporation.
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
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
			testDiffEvents(t, path, accept, false /* truncateOutput */)
		})
		t.Run(entry.Name()+"/urns", func(t *testing.T) {
			t.Parallel()
			testDiffEventsURNs(t, path, accept)
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
			testDiffEvents(t, path, accept, true /* truncateOutput */)
		})
	}
}

func testDiffEventsURNs(t *testing.T, path string, accept bool) {
	events, err := loadEvents(path)
	require.NoError(t, err)

	var expectedStdout []byte
	if !accept {
		expectedStdout, err = os.ReadFile(path + ".urns.stdout.txt")
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
		ShowURNs:             true,
		Stdout:               &stdout,
		Stderr:               &stderr,
	})

	for _, e := range events {
		eventChannel <- e
	}
	<-doneChannel

	if !accept {
		assert.Equal(t, string(expectedStdout), stdout.String())
	} else {
		err = os.WriteFile(path+".urns.stdout.txt", stdout.Bytes(), 0o600)
		require.NoError(t, err)
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
			testDiffEvents(t, path, accept, false /* truncateOutput */)
		})
	}
}

func assertExpectedCreateDiff(t *testing.T, path string, accept, truncateOutput bool) {
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
		TruncateOutput:       truncateOutput,
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
			assertExpectedCreateDiff(t, path, accept, false /* truncateOutput */)
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
			assertExpectedCreateDiff(t, path, accept, true /* truncateOutput */)
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
			assertExpectedCreateDiff(t, path, accept, false /* truncateOutput */)
		})
	}
}

func TestCreateDiffRequiresColor(t *testing.T) {
	t.Parallel()

	_, err := CreateDiff([]engine.Event{}, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "color must be specified")
}

func TestCreateDiffDoesNotIndentBeneathHiddenParent(t *testing.T) {
	t.Parallel()

	stackURN := resource.URN("urn:pulumi:dev::project::pulumi:pulumi:Stack::project-dev")
	parentURN := resource.URN("urn:pulumi:dev::project::pkg:index:Parent::parentRes")
	childURN := resource.URN("urn:pulumi:dev::project::pkg:index:Parent$pkg:index:Child::childRes")
	siblingURN := resource.URN("urn:pulumi:dev::project::pkg:index:Sibling::siblingRes")

	newState := func(urn resource.URN, parent resource.URN) *engine.StepEventStateMetadata {
		return &engine.StepEventStateMetadata{
			URN:    urn,
			Type:   urn.Type(),
			Parent: parent,
			Inputs: resource.PropertyMap{},
		}
	}

	events := []engine.Event{
		engine.NewEvent(engine.ResourcePreEventPayload{
			Metadata: engine.StepEventMetadata{
				URN:  stackURN,
				Type: stackURN.Type(),
				Op:   deploy.OpSame,
				Old:  newState(stackURN, ""),
				New:  newState(stackURN, ""),
				Res:  newState(stackURN, ""),
			},
		}),
		engine.NewEvent(engine.ResourcePreEventPayload{
			Metadata: engine.StepEventMetadata{
				URN:  parentURN,
				Type: parentURN.Type(),
				Op:   deploy.OpSame,
				Old:  newState(parentURN, stackURN),
				New:  newState(parentURN, stackURN),
				Res:  newState(parentURN, stackURN),
			},
		}),
		engine.NewEvent(engine.ResourcePreEventPayload{
			Metadata: engine.StepEventMetadata{
				URN:  siblingURN,
				Type: siblingURN.Type(),
				Op:   deploy.OpCreate,
				New:  newState(siblingURN, stackURN),
				Res:  newState(siblingURN, stackURN),
			},
		}),
		engine.NewEvent(engine.ResourcePreEventPayload{
			Metadata: engine.StepEventMetadata{
				URN:  childURN,
				Type: childURN.Type(),
				Op:   deploy.OpUpdate,
				Old:  newState(childURN, parentURN),
				New:  newState(childURN, parentURN),
				Res:  newState(childURN, parentURN),
			},
		}),
		engine.NewCancelEvent(),
	}

	diff, err := CreateDiff(events, Options{
		Color:                colors.Never,
		ShowSameResources:    false,
		ShowReplacementSteps: true,
	})
	require.NoError(t, err)

	expected := `pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:dev::project::pulumi:pulumi:Stack::project-dev]
    + pkg:index:Sibling: (create)
        [urn=urn:pulumi:dev::project::pkg:index:Sibling::siblingRes]
    ~ pkg:index:Child: (update)
        [urn=urn:pulumi:dev::project::pkg:index:Parent$pkg:index:Child::childRes]
`
	assert.Equal(t, strings.TrimSuffix(expected, "\n"), diff)
}
