package test

import "github.com/pulumi/pulumi/pkg/resource/deploy/deploytest"

var AWS = &deploytest.Provider{
	GetSchemaF: func(version int) ([]byte, error) {
		return awsSchema, nil
	},
}
