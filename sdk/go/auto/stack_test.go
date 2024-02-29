// Copyright 2016-2021, Pulumi Corporation.
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

package auto

import (
	"context"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPermalink = "Permalink: https://gotest"

func TestGetPermalink(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		testee string
		want   string
		err    error
	}{
		"successful parsing": {testee: testPermalink + "\n", want: "https://gotest"},
		"failed parsing":     {testee: testPermalink, err: ErrParsePermalinkFailed},
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := GetPermalink(test.testee)
			if err != nil {
				if test.err == nil || test.err != err {
					t.Errorf("got '%v', want '%v'", err, test.err)
				}
			}

			if got != test.want {
				t.Errorf("got '%s', want '%s'", got, test.want)
			}
		})
	}
}

func TestUpdatePlans(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)

	opts := []LocalWorkspaceOption{
		SecretsProvider("passphrase"),
		EnvVars(map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": "password",
		}),
	}

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	}, opts...)
	require.NoError(t, err, "failed to initialize stack, err: %v", err)

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	// first load settings for created stack
	stackConfig, err := s.Workspace().StackSettings(ctx, stackName)
	require.NoError(t, err)
	stackConfig.SecretsProvider = "passphrase"
	assert.NoError(t, s.Workspace().SaveStackSettings(ctx, stackName, stackConfig))

	// -- pulumi preview --
	tempFile, err := os.CreateTemp("", "update_plan.json")
	defer os.Remove(tempFile.Name())

	_, err = s.Preview(ctx, optpreview.Plan(tempFile.Name()))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}

	stat, err := tempFile.Stat()
	if err != nil {
		t.Errorf("state failed, err: %v", err)
		t.FailNow()
	}

	if stat.Size() == 0 {
		t.Errorf("expected update plan size to be non-zero")
		t.FailNow()
	}

	// -- pulumi up --

	upResult, err := s.Up(ctx, optup.Plan(tempFile.Name()))
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "update", upResult.Summary.Kind)
	assert.Equal(t, "succeeded", upResult.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}
