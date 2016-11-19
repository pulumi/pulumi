// Copyright 2016 Marapongo, Inc. All rights reserved.

package predef

import (
	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/util"
)

// MuExtensionService contains the extra properties that are specific to MuExtension stacks.
type MuExtensionService struct {
	ast.Service
	Provider ast.Name `json:"-"` // the extensible provider name.
}

// AsMuExtensionService converts a given service to a MuExtensionService, validating it as we go.
func AsMuExtensionService(svc *ast.Service) *MuExtensionService {
	util.AssertM(svc.BoundType == MuExtension, "ServiceToMuExtension expects a bound MuExtension service type")

	p, ok := svc.Extra["provider"]
	util.AssertM(ok, "MuExtension is expected to have a required 'provider' property")
	prov, ok := p.(string)
	util.AssertMF(ok, "MuExtension's 'provider' property is expected to be of type 'string'; got %v", p)

	return &MuExtensionService{
		Service:  *svc,
		Provider: ast.Name(prov),
	}
}
