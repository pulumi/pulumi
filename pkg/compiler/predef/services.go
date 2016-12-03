// Copyright 2016 Marapongo, Inc. All rights reserved.

package predef

import (
	"reflect"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/util"
)

// ExtensionService contains the extra properties that are specific to MuExtension stacks.
type ExtensionService struct {
	ast.Service
	Provider ast.Name `json:"-"` // the extensible provider name.
}

// AsExtensionService converts a given service to a MuExtensionService, validating it as we go.
func AsExtensionService(svc *ast.Service) *ExtensionService {
	util.AssertM(svc.BoundType == Extension, "ServiceToMuExtension expects a bound MuExtension service type")

	p, ok := svc.BoundProps["provider"]
	util.AssertM(ok, "Extension is expected to have a required 'provider' property")
	lit, ok := p.(ast.StringLiteral)
	util.AssertMF(ok, "Extension 'provider' property is expected to be of type 'string'; got '%v'", reflect.TypeOf(p))

	return &ExtensionService{
		Service:  *svc,
		Provider: ast.Name(lit.String),
	}
}
