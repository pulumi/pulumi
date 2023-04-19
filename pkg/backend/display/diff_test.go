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

	//nolint:paralleltest
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

	//nolint:paralleltest
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

func TestDiffNoSecret(t *testing.T) {
	t.Parallel()
	// Test that we don't show the secret value in the diff.
	event := engine.StepEventMetadata{
		Op:   "create",
		URN:  "urn:pulumi:dev::secret-random-yaml::random:index/randomPet:RandomPet::param",
		Type: "random:index/randomPet:RandomPet",
		Old: &engine.StepEventStateMetadata{
			Inputs: resource.PropertyMap{
				"length": {
					V: float64(222),
				},
			},
			Outputs: resource.PropertyMap{
				"length": {
					V: float64(222),
				},
			},
		},
	}
	details := getResourcePropertiesDetails(
		event,
		1,     // indent
		true,  // planning
		false, // summary
		false, // truncateOutput
		false, // debug
	)
	assert.Contains(t, details, "222")
	assert.NotContains(t, details, "[secret]")
}

func TestDiffSecretOld(t *testing.T) {
	t.Parallel()
	// Test that we don't show the secret value in the diff.
	event := engine.StepEventMetadata{
		Op:   "create",
		URN:  "urn:pulumi:dev::secret-random-yaml::random:index/randomPet:RandomPet::param",
		Type: "random:index/randomPet:RandomPet",
		Old: &engine.StepEventStateMetadata{
			Inputs: resource.PropertyMap{
				"length": {
					V: &resource.Secret{
						Element: resource.PropertyValue{
							V: float64(222),
						},
					},
				},
			},
			Outputs: resource.PropertyMap{
				"length": {
					V: float64(222),
				},
			},
		},
	}
	details := getResourcePropertiesDetails(
		event,
		1,     // indent
		true,  // planning
		false, // summary
		false, // truncateOutput
		false, // debug
	)
	assert.NotContains(t, details, "222")
	assert.Contains(t, details, "[secret]")
}

func TestDiffSecretCreate(t *testing.T) {
	t.Parallel()
	// Test that we don't show the secret value in the diff.
	event := engine.StepEventMetadata{
		Op:   "create",
		URN:  "urn:pulumi:dev::secret-random-yaml::random:index/randomPet:RandomPet::param",
		Type: "random:index/randomPet:RandomPet",
		New: &engine.StepEventStateMetadata{
			Inputs: resource.PropertyMap{
				"length": {
					V: &resource.Secret{
						Element: resource.PropertyValue{
							V: float64(222),
						},
					},
				},
			},
			Outputs: resource.PropertyMap{
				"length": {
					V: float64(222),
				},
			},
		},
	}
	details := getResourcePropertiesDetails(
		event,
		1,     // indent
		true,  // planning
		true,  // summary
		false, // truncateOutput
		false, // debug
	)
	assert.NotContains(t, details, "222")
	assert.Contains(t, details, "[secret]")
}

func TestDiffSecret(t *testing.T) {
	t.Parallel()
	// Test that we don't show the secret value in the diff.
	event := engine.StepEventMetadata{
		Op:   "update",
		URN:  "urn:pulumi:dev::secret-random-yaml::random:index/randomPet:RandomPet::param",
		Type: "random:index/randomPet:RandomPet",
		New: &engine.StepEventStateMetadata{
			Inputs: resource.PropertyMap{
				"length": {
					V: &resource.Secret{
						Element: resource.PropertyValue{
							V: float64(222),
						},
					},
				},
			},
			Outputs: resource.PropertyMap{
				"length": {
					V: float64(333),
				},
			},
		},
		Old: &engine.StepEventStateMetadata{
			Inputs: resource.PropertyMap{
				"length": {
					V: &resource.Secret{
						Element: resource.PropertyValue{
							V: float64(333),
						},
					},
				},
			},
			Outputs: resource.PropertyMap{
				"length": {
					V: float64(333),
				},
			},
		},
	}
	details := getResourcePropertiesDetails(
		event,
		1,     // indent
		true,  // planning
		true,  // summary
		false, // truncateOutput
		false, // debug
	)
	assert.NotContains(t, details, "333")
	assert.NotContains(t, details, "222")
	assert.Contains(t, details, "[secret]")
}

func TestDiffReplaceSecret(t *testing.T) {
	t.Parallel()
	// Test that we don't show the secret value in the diff.
	event := engine.StepEventMetadata{
		Op:   "delete-replaced",
		URN:  "urn:pulumi:dev::secret-random-yaml::random:index/randomPet:RandomPet::param",
		Type: "random:index/randomPet:RandomPet",
		Old: &engine.StepEventStateMetadata{
			Inputs: resource.PropertyMap{
				"length": {
					V: &resource.Secret{
						Element: resource.PropertyValue{
							V: "[secret]",
						},
					},
				},
			},
			Outputs: resource.PropertyMap{
				"length": {
					V: float64(222),
				},
			},
		},
		New: &engine.StepEventStateMetadata{
			Inputs: resource.PropertyMap{
				"length": {
					V: &resource.Secret{
						Element: resource.PropertyValue{
							V: float64(333),
						},
					},
				},
			},
			Outputs: resource.PropertyMap{
				"length": {
					V: float64(222),
				},
			},
		},
	}
	details := getResourcePropertiesDetails(
		event,
		1,     // indent
		true,  // planning
		false, // summary
		false, // truncateOutput
		false, // debug
	)

	assert.NotContains(t, details, "333")
	assert.NotContains(t, details, "222")
	assert.Contains(t, details, "[secret]")
}
