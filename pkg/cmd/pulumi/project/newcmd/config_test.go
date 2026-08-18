// Copyright 2024, Pulumi Corporation.
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

package newcmd

import (
	"fmt"
	"slices"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfigSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Array    []string
		Path     bool
		Expected config.Map
	}{
		{
			Array:    []string{},
			Expected: config.Map{},
		},
		{
			Array: []string{"my:testKey"},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue(""),
			},
		},
		{
			Array: []string{"my:testKey="},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue(""),
			},
		},
		{
			Array: []string{"my:testKey=testValue"},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{"my:testKey=test=Value"},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("test=Value"),
			},
		},
		{
			Array: []string{
				"my:testKey=testValue",
				"my:testKey=rewritten",
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("rewritten"),
			},
		},
		{
			Array: []string{
				"my:testKey=testValue",
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				"my:test.Key=testValue",
			},
			Expected: config.Map{
				config.MustMakeKey("my", "test.Key"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				"my:testKey=testValue",
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				"my:0=testValue",
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "0"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				"my:true=testValue",
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "true"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				`my:["test.Key"]=testValue`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "test.Key"): config.NewValue("testValue"),
			},
		},
		{
			Array: []string{
				`my:outer.inner=value`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "outer"): config.NewObjectValue(`{"inner":"value"}`),
			},
		},
		{
			Array: []string{
				`my:outer.inner.nested=value`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "outer"): config.NewObjectValue(`{"inner":{"nested":"value"}}`),
			},
		},
		{
			Array: []string{
				`my:name[0]=value`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "name"): config.NewObjectValue(`["value"]`),
			},
		},
		{
			Array: []string{
				`my:name[0][0]=value`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "name"): config.NewObjectValue(`[["value"]]`),
			},
		},
		{
			Array: []string{
				`my:servers[0].name=foo`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "servers"): config.NewObjectValue(`[{"name":"foo"}]`),
			},
		},
		{
			Array: []string{
				`my:testKey=false`,
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("false"),
			},
		},
		{
			Array: []string{
				`my:testKey=true`,
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("true"),
			},
		},
		{
			Array: []string{
				`my:testKey=10`,
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("10"),
			},
		},
		{
			Array: []string{
				`my:testKey=-1`,
			},
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewValue("-1"),
			},
		},
		{
			Array: []string{
				`my:testKey[0]=false`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewObjectValue(`[false]`),
			},
		},
		{
			Array: []string{
				`my:testKey[0]=true`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewObjectValue(`[true]`),
			},
		},
		{
			Array: []string{
				`my:testKey[0]=10`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewObjectValue(`[10]`),
			},
		},
		{
			Array: []string{
				`my:testKey[0]=-1`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "testKey"): config.NewObjectValue(`[-1]`),
			},
		},
		{
			Array: []string{
				`my:names[0]=a`,
				`my:names[1]=b`,
				`my:names[2]=c`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "names"): config.NewObjectValue(`["a","b","c"]`),
			},
		},
		{
			Array: []string{
				`my:names[0]=a`,
				`my:names[1]=b`,
				`my:names[2]=c`,
				`my:names[0]=rewritten`,
			},
			Path: true,
			Expected: config.Map{
				config.MustMakeKey("my", "names"): config.NewObjectValue(`["rewritten","b","c"]`),
			},
		},
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			t.Parallel()

			actual, err := ParseConfig(test.Array, test.Path)
			require.NoError(t, err)
			assert.Equal(t, test.Expected, actual)
		})
	}
}

func TestSetFail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Array    []string
		Expected config.Map
	}{
		{
			Array: []string{`my:[""]=value`},
		},
		{
			Array: []string{"my:[0]=value"},
		},
		{
			Array: []string{`my:name[-1]=value`},
		},
		{
			Array: []string{`my:key.secure=value`},
		},
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			t.Parallel()

			_, err := ParseConfig(test.Array, true /*path*/)
			assert.Error(t, err)
		})
	}
}

func TestTemplateConfigResolvesWithoutPrompting(t *testing.T) {
	t.Parallel()

	values, err := resolveTemplateConfig(
		"proj",
		map[string]workspace.ProjectTemplateConfigValue{
			"aws:region":  {Description: "The AWS region to deploy into", Default: "us-east-1"},
			"gcp:zone":    {Description: "The zone"},
			"gcp:project": {Description: "The Google Cloud project to deploy into"},
			"auth0:token": {Description: "The token", Secret: true},
		},
		config.Map{config.MustMakeKey("gcp", "zone"): config.NewValue("z")},
		nil, nil,
	)
	require.NoError(t, err)
	assert.True(t, slices.ContainsFunc(values, templateConfigValue.unset),
		"the no-default keys are unset, which is what earns a question")

	prompts := 0
	prompt := func(yes bool, valueType, defaultValue string, secret bool,
		isValidFn func(string) error, opts display.Options,
	) (string, error) {
		prompts++
		return "typed:" + valueType, nil
	}
	require.NoError(t, askTemplateConfig(values, prompt, false, askUnset, display.Options{}))

	byKey := map[string]templateConfigValue{}
	for _, v := range values {
		byKey[v.key.String()] = v
	}
	assert.Equal(t, "us-east-1", byKey["aws:region"].value, "a default is filled in silently")
	assert.Equal(t, "z", byKey["gcp:zone"].value, "a flag value is filled in silently")
	assert.True(t, byKey["gcp:zone"].fromFlag)
	assert.Equal(t,
		"typed:The Google Cloud project to deploy into (gcp:project)",
		byKey["gcp:project"].value, "no default means a prompt, with promptForConfig's prompt text")
	assert.True(t, byKey["auth0:token"].secret)
	assert.Equal(t, 2, prompts, "only the two no-default keys prompt")
}

func TestTemplateConfigAsksForEveryKey(t *testing.T) {
	t.Parallel()

	values, err := resolveTemplateConfig(
		"proj",
		map[string]workspace.ProjectTemplateConfigValue{
			"aws:region": {Default: "us-east-1"},
			"gcp:zone":   {Default: "a"},
		},
		config.Map{config.MustMakeKey("gcp", "zone"): config.NewValue("z")},
		nil, nil,
	)
	require.NoError(t, err)
	require.Len(t, values, 2)
	// Overlay a prior value, as the decline path does with what its confirmation showed.
	values[0].value = "eu-west-1"

	defaults := map[string]string{}
	prompt := func(yes bool, valueType, defaultValue string, secret bool,
		isValidFn func(string) error, opts display.Options,
	) (string, error) {
		defaults[valueType] = defaultValue
		return "typed", nil
	}
	require.NoError(t, askTemplateConfig(values, prompt, false, askAll, display.Options{}))

	assert.Equal(t, map[string]string{"aws:region": "eu-west-1"}, defaults,
		"the resolved value pre-fills the prompt, and a key fixed by --config is never asked")
	byKey := map[string]templateConfigValue{}
	for _, v := range values {
		byKey[v.key.String()] = v
	}
	assert.Equal(t, "typed", byKey["aws:region"].value)
	assert.Equal(t, "z", byKey["gcp:zone"].value, "the flag value survives, unasked")
}

func TestTemplateConfigNamespacesBareKeys(t *testing.T) {
	t.Parallel()

	// A bare key takes the project's namespace. Resolving it must not depend on a Pulumi.yaml
	// being on disk, which it is not when the guided flow gathers config before creating one.
	values, err := resolveTemplateConfig(
		"my-proj",
		map[string]workspace.ProjectTemplateConfigValue{
			"bucketName": {Description: "The bucket"},
		},
		nil, nil, nil,
	)
	require.NoError(t, err)
	require.Len(t, values, 1)
	assert.Equal(t, "my-proj:bucketName", values[0].key.String())
	assert.Equal(t, "bucketName", values[0].templateKey)

	prompt := func(yes bool, valueType, defaultValue string, secret bool,
		isValidFn func(string) error, opts display.Options,
	) (string, error) {
		return "typed:" + valueType, nil
	}
	require.NoError(t, askTemplateConfig(values, prompt, false, askUnset, display.Options{}))
	assert.Equal(t, "typed:The bucket (bucketName)", values[0].value,
		"the prompt elides the namespace when it is the project's own")
}

func TestTemplateConfigOrdersKeys(t *testing.T) {
	t.Parallel()

	values, err := resolveTemplateConfig(
		"proj",
		map[string]workspace.ProjectTemplateConfigValue{
			"b:two": {Default: "2"}, "a:one": {Default: "1"},
		},
		nil, nil, nil,
	)
	require.NoError(t, err)
	require.Len(t, values, 2)
	assert.Equal(t, "a:one", values[0].key.String())
	assert.Equal(t, "b:two", values[1].key.String())
	assert.False(t, slices.ContainsFunc(values, templateConfigValue.unset),
		"every key had a default, so nothing would be asked")
}

func TestParseConfigForProjectNamespacesBareKeys(t *testing.T) {
	t.Parallel()

	c, err := ParseConfigForProject("my-proj", []string{"bucketName=b", "aws:region=us-east-1"}, false)
	require.NoError(t, err)
	assert.Equal(t, config.Map{
		config.MustMakeKey("my-proj", "bucketName"): config.NewValue("b"),
		config.MustMakeKey("aws", "region"):         config.NewValue("us-east-1"),
	}, c)
}
