// Copyright 2023, Pulumi Corporation.  All rights reserved.

package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/schema"
	"github.com/pulumi/esc/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func accept() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))
}

type errorProvider struct{}

func (errorProvider) Schema() (*schema.Schema, *schema.Schema) {
	return schema.Record(map[string]schema.Builder{"why": schema.String()}).Schema(), schema.Always()
}

func (errorProvider) Open(ctx context.Context, inputs map[string]esc.Value) (esc.Value, error) {
	return esc.Value{}, errors.New(inputs["why"].Value.(string))
}

type testProvider struct{}

func (testProvider) Schema() (*schema.Schema, *schema.Schema) {
	return schema.Always(), schema.Always()
}

func (testProvider) Open(ctx context.Context, inputs map[string]esc.Value) (esc.Value, error) {
	return esc.NewValue(inputs), nil
}

type testProviders struct{}

func (testProviders) LoadProvider(ctx context.Context, name string) (esc.Provider, error) {
	switch name {
	case "error":
		return errorProvider{}, nil
	case "test":
		return testProvider{}, nil
	}
	return nil, fmt.Errorf("unknown provider %q", name)
}

type testEnvironments struct {
	root string
}

func (e *testEnvironments) LoadEnvironment(ctx context.Context, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(e.root, name+".yaml"))
}

func TestEval(t *testing.T) {
	type expectedData struct {
		LoadDiags   syntax.Diagnostics `json:"loadDiags,omitempty"`
		CheckDiags  syntax.Diagnostics `json:"checkDiags,omitempty"`
		EvalDiags   syntax.Diagnostics `json:"evalDiags,omitempty"`
		Environment *esc.Environment   `json:"environment,omitempty"`
	}

	path := filepath.Join("testdata", "eval")
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			basePath := filepath.Join(path, e.Name())
			envPath, expectedPath := filepath.Join(basePath, "env.yaml"), filepath.Join(basePath, "expected.json")

			envBytes, err := os.ReadFile(envPath)
			require.NoError(t, err)

			if accept() {
				env, loadDiags, err := LoadYAMLBytes(e.Name(), envBytes)
				require.NoError(t, err)

				_, checkDiags := CheckEnvironment(context.Background(), e.Name(), env, testProviders{}, &testEnvironments{basePath})

				actual, evalDiags := EvalEnvironment(context.Background(), e.Name(), env, testProviders{}, &testEnvironments{basePath})

				bytes, err := json.MarshalIndent(expectedData{
					LoadDiags:   loadDiags,
					CheckDiags:  checkDiags,
					EvalDiags:   evalDiags,
					Environment: actual,
				}, "", "    ")
				require.NoError(t, err)

				err = os.WriteFile(expectedPath, bytes, 0600)
				require.NoError(t, err)

				return
			}

			var expected expectedData
			expectedBytes, err := os.ReadFile(expectedPath)
			require.NoError(t, err)
			dec := json.NewDecoder(bytes.NewReader(expectedBytes))
			dec.UseNumber()
			err = dec.Decode(&expected)
			require.NoError(t, err)

			env, diags, err := LoadYAMLBytes(e.Name(), envBytes)
			require.NoError(t, err)
			require.Equal(t, expected.LoadDiags, diags)

			_, diags = CheckEnvironment(context.Background(), e.Name(), env, testProviders{}, &testEnvironments{basePath})
			require.Equal(t, expected.CheckDiags, diags)

			actual, diags := EvalEnvironment(context.Background(), e.Name(), env, testProviders{}, &testEnvironments{basePath})
			require.Equal(t, expected.EvalDiags, diags)

			// work around a schema comparison issue due to the 'compiled' field by roundtripping through JSON
			actualBytes, err := json.Marshal(actual)
			require.NoError(t, err)
			dec = json.NewDecoder(bytes.NewReader(actualBytes))
			dec.UseNumber()
			err = dec.Decode(&actual)
			require.NoError(t, err)

			assert.Equal(t, expected.Environment, actual)
		})
	}
}
