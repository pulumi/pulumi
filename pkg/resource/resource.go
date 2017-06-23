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

package resource

import (
	"github.com/pulumi/lumi/pkg/tokens"
)

// Resource is an instance of a resource with an ID, type, and bag of state.
type Resource interface {
	URN() URN          // the resource's object URN: a human-friendly, unique name for the resource.
	Type() tokens.Type // the resource's type.
}

// Status is returned when an error has occurred during a resource provider operation.  It indicates whether the
// operation could be rolled back cleanly (OK).  If not, it means the resource was left in an indeterminate state.
type Status int

const (
	StatusOK Status = iota
	StatusUnknown
)

// HasURN returns true if the resource has been assigned a universal resource name (URN).
func HasURN(r Resource) bool {
	return r.URN() != ""
}
