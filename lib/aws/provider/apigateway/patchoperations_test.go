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
		&apigateway.PatchOperation{
			Op:    aws.String("add"),
			Path:  aws.String("/optionalArray/1"),
			Value: aws.String("{\"number\": 1}"),
		},
		&apigateway.PatchOperation{
			Op:    aws.String("replace"),
			Path:  aws.String("/optionalArray/0/number"),
			Value: aws.String("3"),
		},
		&apigateway.PatchOperation{
			Op:    aws.String("add"),
			Path:  aws.String("/optionalArray/0/optionalBool"),
			Value: aws.String("true"),
		},
		&apigateway.PatchOperation{
			Op:   aws.String("remove"),
			Path: aws.String("/optionalBool"),
		},
		&apigateway.PatchOperation{
			Op:    aws.String("add"),
			Path:  aws.String("/optionalNumber"),
			Value: aws.String("3"),
		},
		&apigateway.PatchOperation{
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
