// Copyright 2016-2018, Pulumi Corporation.
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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringifyOutput(t *testing.T) {
	t.Parallel()

	num := 42
	str := "ABC"
	arr := []string{"hello", "goodbye"}
	obj := map[string]interface{}{
		"foo": 42,
		"bar": map[string]interface{}{
			"baz": true,
		},
	}
	specialChar := "pass&word"

	assert.Equal(t, "42", stringifyOutput(num))
	assert.Equal(t, "ABC", stringifyOutput(str))
	assert.Equal(t, "[\"hello\",\"goodbye\"]", stringifyOutput(arr))
	assert.Equal(t, "{\"bar\":{\"baz\":true},\"foo\":42}", stringifyOutput(obj))
	assert.Equal(t, "pass&word", stringifyOutput(specialChar))
}

// Tests the output of 'pulumi stack output'
// under different conditions.
//
//nolint:paralleltest // hijacks os.Stdout
func TestStackOutputCmd_plainText(t *testing.T) {
	// This test temporarily hijacks os.Stdout
	// so that we can read the output printed to it.
	// This will not be necessary once the relevant functions
	// are modified to accept io.Writers.

	outputsWithSecret := resource.PropertyMap{
		"bucketName": resource.NewStringProperty("mybucket-1234"),
		"password": resource.NewSecretProperty(&resource.Secret{
			Element: resource.NewStringProperty("hunter2"),
		}),
	}

	tests := []struct {
		desc string

		// Map of stack outputs.
		outputs resource.PropertyMap

		// Whether the --show-secrets flag is set.
		showSecrets bool

		// Any additional command line arguments.
		args []string

		// Expectations from stdout:
		contains    []string
		notContains []string
		equals      string // only valid if non-empty
	}{
		{
			desc:        "default",
			outputs:     outputsWithSecret,
			contains:    []string{"mybucket-1234", "password", "[secret]"},
			notContains: []string{"hunter2"},
		},
		{
			desc:        "show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			contains:    []string{"mybucket-1234", "password", "hunter2"},
		},
		{
			desc:    "single property",
			outputs: outputsWithSecret,
			args:    []string{"bucketName"},
			equals:  "mybucket-1234\n",
		},
		{
			// Should not show the secret even if requested
			// if --show-secrets is not set.
			desc:    "single hidden property",
			outputs: outputsWithSecret,
			args:    []string{"password"},
			equals:  "[secret]\n",
		},
		{
			desc:        "single property with show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			args:        []string{"password"},
			equals:      "hunter2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			getStdout := hijackStdout(t)

			snap := deploy.Snapshot{
				Resources: []*resource.State{
					{
						Type:    resource.RootStackType,
						Outputs: tt.outputs,
					},
				},
			}
			requireStack := func(context.Context,
				string, stackLoadOption, display.Options) (backend.Stack, error) {
				return &backend.MockStack{
					SnapshotF: func(ctx context.Context) (*deploy.Snapshot, error) {
						return &snap, nil
					},
				}, nil
			}

			cmd := stackOutputCmd{
				requireStack: requireStack,
				showSecrets:  tt.showSecrets,
			}
			require.NoError(t, cmd.Run(context.Background(), tt.args))
			stdout := string(getStdout())

			if tt.equals != "" {
				assert.Equal(t, tt.equals, stdout)
			}
			for _, s := range tt.contains {
				assert.Contains(t, stdout, s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, stdout, s)
			}
		})
	}
}

// Tests the output of 'pulumi stack output --json'
// under different conditions.
//
//nolint:paralleltest // hijacks os.Stdout
func TestStackOutputCmd_json(t *testing.T) {
	// This test temporarily hijacks os.Stdout
	// so that we can read the output printed to it.
	// This will not be necessary once the relevant functions
	// are modified to accept io.Writers.

	outputsWithSecret := resource.PropertyMap{
		"bucketName": resource.NewStringProperty("mybucket-1234"),
		"password": resource.NewSecretProperty(&resource.Secret{
			Element: resource.NewStringProperty("hunter2"),
		}),
	}

	tests := []struct {
		desc string

		// Map of stack outputs.
		outputs resource.PropertyMap

		// Whether the --show-secrets flag is set.
		showSecrets bool

		// Any additional command line arguments.
		args []string

		// Expected parsed JSON output.
		want interface{}
	}{
		{
			desc:    "default",
			outputs: outputsWithSecret,
			want: map[string]interface{}{
				"bucketName": "mybucket-1234",
				"password":   "[secret]",
			},
		},
		{
			desc:        "show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			want: map[string]interface{}{
				"bucketName": "mybucket-1234",
				"password":   "hunter2",
			},
		},
		{
			desc:    "single property",
			outputs: outputsWithSecret,
			args:    []string{"bucketName"},
			want:    "mybucket-1234",
		},
		{
			// Should not show the secret even if requested
			// if --show-secrets is not set.
			desc:    "single hidden property",
			outputs: outputsWithSecret,
			args:    []string{"password"},
			want:    "[secret]",
		},
		{
			desc:        "single property with show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			args:        []string{"password"},
			want:        "hunter2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			getStdout := hijackStdout(t)

			snap := deploy.Snapshot{
				Resources: []*resource.State{
					{
						Type:    resource.RootStackType,
						Outputs: tt.outputs,
					},
				},
			}
			requireStack := func(context.Context,
				string, stackLoadOption, display.Options) (backend.Stack, error) {
				return &backend.MockStack{
					SnapshotF: func(ctx context.Context) (*deploy.Snapshot, error) {
						return &snap, nil
					},
				}, nil
			}

			cmd := stackOutputCmd{
				requireStack: requireStack,
				showSecrets:  tt.showSecrets,
				jsonOut:      true,
			}
			require.NoError(t, cmd.Run(context.Background(), tt.args))

			stdout := getStdout()
			var got interface{}
			require.NoError(t, json.Unmarshal(stdout, &got),
				"output is not valid JSON:\n%s", stdout)

			assert.Equal(t, tt.want, got)
		})
	}
}

// Tests the output of 'pulumi stack output --shell'
// under different conditions.
//
//nolint:paralleltest // hijacks os.Stdout
func TestStackOutputCmd_shell(t *testing.T) {
	// This test temporarily hijacks os.Stdout
	// so that we can read the output printed to it.
	// This will not be necessary once the relevant functions
	// are modified to accept io.Writers.

	outputsWithSecret := resource.PropertyMap{
		"bucketName": resource.NewStringProperty("mybucket-1234"),
		"password": resource.NewSecretProperty(&resource.Secret{
			Element: resource.NewStringProperty("hunter2"),
		}),
	}

	tests := []struct {
		desc string

		// Map of stack outputs.
		outputs resource.PropertyMap

		// Whether the --show-secrets flag is set.
		showSecrets bool

		// Any additional command line arguments.
		args []string

		// Lines expected in the output.
		want []string
	}{
		{
			desc:    "default",
			outputs: outputsWithSecret,
			want: []string{
				"bucketName=mybucket-1234",
				`password=\[secret]`,
			},
		},
		{
			desc:        "show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			want: []string{
				"bucketName=mybucket-1234",
				"password=hunter2",
			},
		},
		{
			desc:    "single property",
			outputs: outputsWithSecret,
			args:    []string{"bucketName"},
			want:    []string{"bucketName=mybucket-1234"},
		},
		{
			// Should not show the secret even if requested
			// if --show-secrets is not set.
			desc:    "single hidden property",
			outputs: outputsWithSecret,
			args:    []string{"password"},
			want:    []string{`password=\[secret]`},
		},
		{
			desc:        "single property with show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			args:        []string{"password"},
			want:        []string{"password=hunter2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			getStdout := hijackStdout(t)

			snap := deploy.Snapshot{
				Resources: []*resource.State{
					{
						Type:    resource.RootStackType,
						Outputs: tt.outputs,
					},
				},
			}
			requireStack := func(context.Context,
				string, stackLoadOption, display.Options) (backend.Stack, error) {
				return &backend.MockStack{
					SnapshotF: func(ctx context.Context) (*deploy.Snapshot, error) {
						return &snap, nil
					},
				}, nil
			}

			cmd := stackOutputCmd{
				requireStack: requireStack,
				showSecrets:  tt.showSecrets,
				shellOut:     true,
			}
			require.NoError(t, cmd.Run(context.Background(), tt.args))

			// Drop trailing "\n" from stdout
			// rather than add a "" at the end of every tt.want.
			stdout := strings.TrimSuffix(string(getStdout()), "\n")
			got := strings.Split(stdout, "\n")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStackOutputCmd_jsonAndShellConflict(t *testing.T) {
	t.Parallel()

	cmd := stackOutputCmd{
		requireStack: func(context.Context, string, stackLoadOption, display.Options) (backend.Stack, error) {
			t.Fatal("This function should not be called")
			return nil, errors.New("should not be called")
		},
		shellOut: true,
		jsonOut:  true,
	}

	err := cmd.Run(context.Background(), nil)
	assert.ErrorContains(t, err, "only one of --json and --shell may be set")
}

func TestShellStackOutputWriter_quoting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give interface{}
		want string // lines expected in the output
	}{
		{
			desc: "string",
			give: "foo",
			want: "foo",
		},
		{
			desc: "number",
			give: 42,
			want: "42",
		},
		{
			desc: "string/spaces",
			give: "foo bar",
			want: "'foo bar'",
		},
		{
			desc: "string/double quotes",
			give: `foo "bar" baz`,
			want: `'foo "bar" baz'`,
		},
		{
			desc: "string/single quotes",
			give: "foo 'bar' baz",
			want: `'foo '\''bar'\'' baz'`,
		},
		{
			desc: "string/single and double quotes",
			give: `foo "bar" 'baz' qux`,
			want: `'foo "bar" '\''baz'\'' qux'`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			var got bytes.Buffer
			writer := shellStackOutputWriter{W: &got}

			err := writer.WriteMany(map[string]interface{}{
				"myoutput": tt.give,
			})
			require.NoError(t, err)

			want := "myoutput=" + tt.want + "\n"
			assert.Equal(t, want, got.String())
		})
	}
}

// hijackStdout replaces os.Stdout and captures anything written to it.
// It returns a function that:
//
//   - restores the original stdout
//   - reports what was written to our hijacked stdout
//   - is safe to call multiple times
//
// Use of this function is techincal debt.
// Pay it by updating relevant printing functions
// to accept an io.Writer.
func hijackStdout(t *testing.T) (restore func() []byte) {
	oldStdout := os.Stdout
	stdoutReader, newStdout, err := os.Pipe()
	require.NoError(t, err)

	done := make(chan struct{})
	var buff bytes.Buffer
	go func() {
		defer close(done)
		_, err := io.Copy(&buff, stdoutReader)
		if errors.Is(err, io.EOF) {
			err = nil
		}
		assert.NoError(t, err, "Error reading stdout")
	}()

	// Always enqueue a restore so that
	// if the caller fails to do it,
	// the test doesn't go silent.
	t.Cleanup(func() { os.Stdout = oldStdout })
	os.Stdout = newStdout
	var once sync.Once
	return func() []byte {
		once.Do(func() {
			os.Stdout = oldStdout
			assert.NoError(t, newStdout.Close(), "Error closing fake stdout")
			<-done
		})
		return buff.Bytes()
	}
}
