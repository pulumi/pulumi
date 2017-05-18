// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubefission

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// Environment identifies the language and OS specific resources that a function depends on.  For now this includes
// only the function run container image.  Later, this will also include build containers, as well as support tools
// like debuggers, profilers, etc.
type Environment struct {
	idl.NamedResource
	RunContainerImageURL string `lumi:"runContainerImageURL"`
}
