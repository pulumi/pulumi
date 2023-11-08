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

package deploy

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
)

func TestBuiltinCoverUnused(t *testing.T) {
	t.Parallel()
	p := newBuiltinProvider(nil, nil)
	defer p.Close()

	assert.Equal(t, tokens.Package("pulumi"), p.Pkg())

	b, err := p.GetSchema(0)
	assert.NoError(t, err)
	assert.Equal(t, []byte("{}"), b)

	m, str, err := p.GetMapping("", "")
	assert.NoError(t, err)
	assert.Nil(t, m)
	assert.Equal(t, "", str)

	mStr, err := p.GetMappings("")
	assert.NoError(t, err)
	assert.Equal(t, []string{}, mStr)

	props, failures, err := p.CheckConfig(resource.URN(""), nil, nil, false)
	assert.NoError(t, err)
	assert.Nil(t, props)
	assert.Nil(t, failures)

	diffRes, err := p.DiffConfig(resource.URN(""), nil, nil, nil, false, nil)
	assert.NoError(t, err)
	assert.Equal(t, plugin.DiffNone, diffRes.Changes)

	assert.NoError(t, p.Configure(nil))
}

func TestBuiltin_Check(t *testing.T) {
	t.Parallel()

	var err error
	var failure []plugin.CheckFailure

	p := newBuiltinProvider(nil, nil)
	_, failure, err = p.Check(resource.URN(
		"urn:pulumi:dev::prog-aws-typescript::random:index/randomPet:RandomPet::pet-0"), nil, nil, false, nil)
	assert.Error(t, err, "expected wrong type")

	_, failure, err = p.Check(resource.URN(
		"urn:pulumi:dev::prog-aws-typescript::pulumi:pulumi:StackReference::pet-0"), nil, resource.PropertyMap{
		"some-unexpected-property": resource.NewNumberProperty(1),
	}, false, nil)
	assert.NoError(t, err)
	assert.Contains(t, failure[0].Reason, "unknown property")

	_, failure, err = p.Check(resource.URN(
		"urn:pulumi:dev::prog-aws-typescript::pulumi:pulumi:StackReference::pet-0"), nil, resource.PropertyMap{}, false, nil)
	assert.NoError(t, err)
	assert.Contains(t, failure[0].Reason, `missing required property "name"`)

	_, failure, err = p.Check(resource.URN(
		"urn:pulumi:dev::prog-aws-typescript::pulumi:pulumi:StackReference::pet-0"), nil, resource.PropertyMap{
		"name": resource.NewNumberProperty(1),
	}, false, nil)
	assert.NoError(t, err)
	assert.Contains(t, failure[0].Reason, `property "name" must be a string`)

	inputs, failure, err := p.Check(resource.URN(
		"urn:pulumi:dev::prog-aws-typescript::pulumi:pulumi:StackReference::pet-0"), nil, resource.PropertyMap{
		"name": resource.NewStringProperty("my-stack-name"),
	}, false, nil)
	assert.NoError(t, err)
	assert.Empty(t, failure)
	assert.Equal(t, resource.NewStringProperty("my-stack-name"), inputs["name"])
}

func TestBuiltinRest(t *testing.T) {
	t.Parallel()
	p := newBuiltinProvider(nil, nil)
	_ = p.Diff
	_ = p.Create
	_ = p.Update
	_ = p.Delete
	_ = p.Read
	_ = p.Construct
	_ = p.Invoke
	_ = p.StreamInvoke
	_ = p.Call
	_ = p.GetPluginInfo
	_ = p.SignalCancellation
	_ = p.readStackReference
	_ = p.readStackResourceOutputs
	_ = p.getResource
}
