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
	OptionalString *string       `json:"string,omitempty"`
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
			Path:  aws.String("/OptionalArray/1"),
			Value: aws.String("{\"Number\": 1}"),
		},
		&apigateway.PatchOperation{
			Op:    aws.String("replace"),
			Path:  aws.String("/OptionalArray/0/Number"),
			Value: aws.String("3"),
		},
		&apigateway.PatchOperation{
			Op:    aws.String("add"),
			Path:  aws.String("/OptionalArray/0/OptionalBool"),
			Value: aws.String("true"),
		},
		&apigateway.PatchOperation{
			Op:   aws.String("remove"),
			Path: aws.String("/OptionalBool"),
		},
		&apigateway.PatchOperation{
			Op:    aws.String("add"),
			Path:  aws.String("/OptionalNumber"),
			Value: aws.String("3"),
		},
		&apigateway.PatchOperation{
			Op:    aws.String("replace"),
			Path:  aws.String("/OptionalString"),
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
