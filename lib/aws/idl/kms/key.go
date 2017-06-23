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

package kms

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The Key resource creates a customer master key (CMK) in AWS Key Management Service (AWS KMS).  Users (customers) can
// use the master key to encrypt their data stored in AWS services that are integrated with AWS KMS or within their
// applications.  For more information, see http://docs.aws.amazon.com/kms/latest/developerguide/.
type Key struct {
	idl.NamedResource
	// KeyPolicy attaches a KMS policy to this key.  Use a policy to specify who has permission to use the key and which
	// actions they can perform.  For more information, see
	// http://docs.aws.amazon.com/kms/latest/developerguide/key-policies.html.
	KeyPolicy interface{} `lumi:"keyPolicy"` // IDEA: schematize this.
	// Description is an optional description of the key.  Use a description that helps your users decide whether the
	// key is appropriate for a particular task.
	Description *string `lumi:"description,optional"`
	// Enabled indicates whether the key is available for use.  This value is `true` by default.
	Enabled *bool `lumi:"enabled,optional"`
	// EnableKeyRotation indicates whether AWS KMS rotates the key.  This value is `false` by default.
	EnableKeyRotation *bool `lumi:"enableKeyRotation,optional"`
}
