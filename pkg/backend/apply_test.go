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

package backend

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Netflix/go-expect"
	backenddisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeUpdateStats tests that the number of non-stack resources and the number of retained resources is correct.
func TestComputeUpdateStats(t *testing.T) {
	t.Parallel()

	events := []engine.Event{
		makeResourcePreEvent("res1", "pulumi:pulumi:Stack", deploy.OpCreate, false),
		makeResourcePreEvent("res2", "custom:resource:Type", deploy.OpCreate, false),
		makeResourcePreEvent("res3", "custom:resource:Type", deploy.OpDelete, true),
		makeResourcePreEvent("res4", "custom:resource:Type", deploy.OpReplace, true),
		makeResourcePreEvent("res5", "custom:resource:Type", deploy.OpSame, false),
	}

	stats := computeUpdateStats(events)

	assert.Equal(t, 4, stats.numNonStackResources)
	assert.Equal(t, 2, len(stats.retainedResources))
}

func makeResourcePreEvent(urn, resType string, op display.StepOp, retainOnDelete bool) engine.Event {
	event := engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: engine.StepEventMetadata{
			Op:   op,
			URN:  resource.URN(urn),
			Type: tokens.Type(resType),
			Old: &engine.StepEventStateMetadata{
				State: &resource.State{
					URN:            resource.URN(urn),
					Type:           tokens.Type(resType),
					RetainOnDelete: retainOnDelete,
				},
			},
		},
	})

	return event
}

// errorExplainer always returns an error for Explain and enables the feature.
type errorExplainer struct{}

func (e *errorExplainer) Explain(
	ctx context.Context, stackRef StackReference, kind apitype.UpdateKind, op UpdateOperation, events []engine.Event,
) (string, error) {
	return "", errors.New("explainer failed")
}
func (e *errorExplainer) IsCopilotFeatureEnabled(opts backenddisplay.Options) bool { return true }

func TestConfirmBeforeUpdating_ExplainerErrorDoesNotCrash(t *testing.T) {
	t.Parallel()

	// Set up the console with a timeout to avoid hangs
	console, err := expect.NewConsole(expect.WithDefaultTimeout(2 * time.Second))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, console.Close())
	}()

	displayOpts := backenddisplay.Options{
		Color:  colors.Never,
		Stdout: console.Tty(),
	}

	ctx := context.Background()
	kind := apitype.UpdateUpdate
	var stackRef StackReference
	op := UpdateOperation{
		Opts: UpdateOptions{
			Display: displayOpts,
		},
	}
	events := []engine.Event{}
	plan := &deploy.Plan{}
	explainer := &errorExplainer{}

	done := make(chan struct{})
	var resultPlan *deploy.Plan
	var callErr error
	go func() {
		defer close(done)
		askOpt := survey.WithStdio(console.Tty(), console.Tty(), console.Tty())
		resultPlan, callErr = confirmBeforeUpdating(ctx, kind, stackRef, op, events, plan, explainer, askOpt)
	}()

	_, err = console.ExpectString("Do you want to perform this update?")
	require.NoError(t, err)
	_, err = console.SendLine("explain")
	require.NoError(t, err)
	_, err = console.ExpectString("An error occurred while explaining the changes:")
	require.NoError(t, err)
	// this is on the next line
	_, err = console.ExpectString("explainer failed")
	require.NoError(t, err)
	_, err = console.SendLine("no")
	require.NoError(t, err)

	<-done

	// Instead of waiting for EOF, just assert on the error and result
	require.Nil(t, resultPlan)
	require.Error(t, callErr)
	require.Contains(t, callErr.Error(), "confirmation declined")
}
