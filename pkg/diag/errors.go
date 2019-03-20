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

func GetAnalyzeResourceFailureError(urn resource.URN) *Diag {
	return newError(urn, 2004,
		"Analyzer '%v' reported a resource error:\n"+
			"\tResource: %v\n"+
			"\tProperty: %v\n"+
			"\tReason: %v")
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

func GetConfigurationUnknownError(urn resource.URN) *Diag {
	const message = "The provider for this resource has inputs that are not known during preview.\n" +
		"This preview may not correctly represent the changes that will be applied during an update."
	return newError(urn, 2008, message)
}

func GetAnalyzerCouldNotBeLoadedError(urn resource.URN) *Diag {
	return newError(urn, 2009, "analyzer '%v' could not be loaded from your $PATH")
}

func GetConfirmationCancelledError() *Diag {
	return newError("", 2010, "confirmation cancelled, not proceeding with the %s")
}

func GetConfirmationDeclinedError() *Diag {
	return newError("", 2011, "confirmation declined, not proceeding with the %s")
}
