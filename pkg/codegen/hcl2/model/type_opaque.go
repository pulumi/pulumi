package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// OpaqueType represents a type that is named by a string.
type OpaqueType = model.OpaqueType

func NewOpaqueType(name string) *OpaqueType {
	return model.NewOpaqueType(name)
}

