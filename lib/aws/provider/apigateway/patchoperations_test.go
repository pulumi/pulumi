// Copyright 2016-2017, Pulumi Corporation
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

package apigateway

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/stretchr/testify/assert"
)

type TestStruct struct {
	Number         float64       `json:"number"`
	OptionalString *string       `json:"optionalString,omitempty"`
	OptionalNumber *float64      `json:"optionalNumber,omitempty"`
	OptionalBool   *bool         `json:"optionalBool,omitempty"`
	OptionalArray  *[]TestStruct `json:"optionalArray,omitempty"`
	OptionalObject *TestStruct   `json:"optionalObject,omitempty"`
}

func Test(t *testing.T) {
	before := TestStruct{
		Number:         1,
		OptionalString: aws.String("hello"),
		OptionalNumber: nil,
		OptionalBool:   aws.Bool(true),
		OptionalArray: &[]TestStruct{
			{Number: 1},
		},
	}
	after := TestStruct{
		Number:         1,
		OptionalString: aws.String("goodbye"),
		OptionalNumber: aws.Float64(3),
		OptionalArray: &[]TestStruct{
			{
				Number:       3,
				OptionalBool: aws.Bool(true),
			},
			{
				Number: 1,
			},
		},
	}
	expectedPatchOps := []*apigateway.PatchOperation{
		{
			Op:    aws.String("add"),
			Path:  aws.String("/optionalArray/1"),
			Value: aws.String("{\"number\": 1}"),
		},
		{
			Op:    aws.String("replace"),
			Path:  aws.String("/optionalArray/0/number"),
			Value: aws.String("3"),
		},
		{
			Op:    aws.String("add"),
			Path:  aws.String("/optionalArray/0/optionalBool"),
			Value: aws.String("true"),
		},
		{
			Op:   aws.String("remove"),
			Path: aws.String("/optionalBool"),
		},
		{
			Op:    aws.String("add"),
			Path:  aws.String("/optionalNumber"),
			Value: aws.String("3"),
		},
		{
			Op:    aws.String("replace"),
			Path:  aws.String("/optionalString"),
			Value: aws.String("goodbye"),
		},
	}
	beforeProps := resource.NewPropertyMap(before)
	afterProps := resource.NewPropertyMap(after)
	diff := beforeProps.Diff(afterProps)
	assert.NotEqual(t, nil, diff, "expected diff should not be nil")

	patchOps, err := patchOperations(diff)
	assert.Nil(t, err, "expected no error generating patch operations")
	assert.EqualValues(t, expectedPatchOps, patchOps)
}
