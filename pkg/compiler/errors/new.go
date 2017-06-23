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

package errors

import (
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// errors tracks all existing errors, keyed by their unique ID.
var errors = make(map[diag.ID]*diag.Diag)

// newError registers a new error message underneath the given unique ID.
func newError(id diag.ID, message string) *diag.Diag {
	contract.Assert(errors[id] == nil)
	e := &diag.Diag{ID: id, Message: message}
	errors[id] = e
	return e
}

// newWarning registers a new warning message underneath the given unique ID.
func newWarning(id diag.ID, message string) *diag.Diag {
	// At the moment, there isn't a distinction between errors and warnings; however, we use different functions just in
	// case someday down the road there is, and so we don't have to go audit all callsites.
	return newError(id, message)
}
