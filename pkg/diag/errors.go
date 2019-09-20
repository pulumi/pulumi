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

package diag

import (
	"github.com/pulumi/pulumi/pkg/resource"
)

// newError registers a new error message underneath the given id.
func newError(urn resource.URN, id ID, message string) *Diag {
	return &Diag{URN: urn, ID: id, Message: message}
}

// Plan and apply errors are in the [2000,3000) range.

func GetPlanApplyFailedError(urn resource.URN) *Diag {
	return newError(urn, 2000, "Plan apply failed: %v")
}

func GetDuplicateResourceURNError(urn resource.URN) *Diag {
	return newError(urn, 2001, "Duplicate resource URN '%v'; try giving it a unique name")
}

func GetResourceInvalidError(urn resource.URN) *Diag {
	return newError(urn, 2002, "%v resource '%v' has a problem: %v")
}

func GetResourcePropertyInvalidValueError(urn resource.URN) *Diag {
	return newError(urn, 2003, "%v resource '%v's property '%v' value %v has a problem: %v")
}

func GetPreviewFailedError(urn resource.URN) *Diag {
	return newError(urn, 2005, "Preview failed: %v")
}

func GetBadProviderError(urn resource.URN) *Diag {
	return newError(urn, 2006, "bad provider reference '%v' for resource '%v': %v")
}

func GetUnknownProviderError(urn resource.URN) *Diag {
	return newError(urn, 2007, "unknown provider '%v' for resource '%v'")
}

func GetDuplicateResourceAliasError(urn resource.URN) *Diag {
	return newError(urn, 2008,
		"Duplicate resource alias '%v' applied to resource with URN '%v' conflicting with resource with URN '%v'",
	)
}

func GetResourceToRefreshCouldNotBeFoundError() *Diag {
	return newError("", 2010, "Resource to refresh '%v' could not be found in the stack.")
}

func GetResourceToRefreshCouldNotBeFoundDidYouForgetError() *Diag {
	return newError("", 2011, "Resource to refresh '%v' could not be found in the stack. "+
		"Did you forget to escape $ in your shell?")
}

func GetResourceToDeleteCouldNotBeFoundError() *Diag {
	return newError("", 2012, "Resource to delete '%v' could not be found in the stack.")
}

func GetResourceToDeleteCouldNotBeFoundDidYouForgetError() *Diag {
	return newError("", 2013, "Resource to delete '%v' could not be found in the stack. "+
		"Did you forget to escape $ in your shell?")
}

func GetCannotDeleteParentResourceWithoutAlsoDeletingChildError() *Diag {
	return newError("", 2014, "Cannot delete parent resource '%v' without also deleting child '%v'.")
}
