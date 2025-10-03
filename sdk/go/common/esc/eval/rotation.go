// Copyright 2025, Pulumi Corporation.
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

package eval

import "github.com/pulumi/esc/syntax"

type RotationStatus string

const (
	RotationSucceeded    RotationStatus = "succeeded"
	RotationFailed       RotationStatus = "failed"
	RotationNotEvaluated RotationStatus = "not-evaluated"
)

// A RotationResult stores the result of secret rotations
type RotationResult []*Rotation

// A Rotation stores secret rotation information and diagnostics
type Rotation struct {
	Path   string             // document path where the rotation was defined
	Status RotationStatus     // status of the rotation
	Diags  syntax.Diagnostics // diagnostics from the rotation
	Patch  *Patch             // updated rotation state generated during evaluation, to be written back to the environment definition
}

func (r RotationResult) Patches() []*Patch {
	var patches []*Patch
	for _, rotation := range r {
		if rotation.Patch != nil {
			patches = append(patches, rotation.Patch)
		}
	}

	return patches
}
