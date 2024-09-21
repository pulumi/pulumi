// Copyright 2023-2024, Pulumi Corporation.
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

package pcl_test

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
)

func TestSingleOrNoneErrorsWithoutArguments(t *testing.T) {
	t.Parallel()
	source := "value = singleOrNone()"
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	contract.Ignore(diags)
	assert.Nil(t, program, "The program doesn't bind")
	assert.ErrorContains(t, err, "'singleOrNone' only expects one argument")
}

func TestSingleOrNoneErrorsWithManyArguments(t *testing.T) {
	t.Parallel()
	source := "value = singleOrNone(1, 2, 3)"
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	contract.Ignore(diags)
	assert.Nil(t, program, "The program doesn't bind")
	assert.ErrorContains(t, err, "'singleOrNone' only expects one argument")
}

func TestSingleOrNoneErrorsWhenFirstArgumentIsNotListOrTuple(t *testing.T) {
	t.Parallel()
	source := "value = singleOrNone(\"hello\")"
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	contract.Ignore(diags)
	assert.Nil(t, program, "The program doesn't bind")
	assert.ErrorContains(t, err, "the first argument to 'singleOrNone' must be a list or tuple")
}

func TestSingleOrNoneBindsCorrectlyWhenFirstArgumentIsList(t *testing.T) {
	t.Parallel()
	// Test that the expression binds and type checks correctly
	// and assert that value: Option<'T> when singleOrNone accepts `T[]
	source := "value = singleOrNone([1, 2, 3])"
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	contract.Ignore(diags)
	assert.NotNil(t, program, "The program doesn't bind")
	assert.Nil(t, err, "There is no bind error")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	localVariable, ok := program.Nodes[0].(*pcl.LocalVariable)
	assert.True(t, ok, "first node is a local variable variable")
	assert.Equal(t, localVariable.Name(), "value")
	variableType := localVariable.Type()
	assert.True(t, model.IsOptionalType(variableType), "the type is an optional")
	elementType := pcl.UnwrapOption(variableType)
	assert.True(t, model.IsConstType(elementType), "element type must the type of one element in the list")
}

func TestBindingInvokeThatReturnsRecursiveType(t *testing.T) {
	t.Parallel()
	source := `value = invoke("recursive:index:getRecursiveType", { name = "foo" })`
	program, diags, err := ParseAndBindProgram(t, source, "program.pp")
	contract.Ignore(diags)
	assert.NotNil(t, program, "The program doesn't bind")
	assert.Nil(t, err, "There is no bind error")
	assert.Equal(t, len(program.Nodes), 1, "there is one node")
	localVariable, ok := program.Nodes[0].(*pcl.LocalVariable)
	assert.True(t, ok, "first node is a local variable variable")
	assert.Equal(t, localVariable.Name(), "value")
	variableType := localVariable.Type()
	_, isPromise := variableType.(*model.PromiseType)
	assert.True(t, isPromise, "the type is a promise")
}
