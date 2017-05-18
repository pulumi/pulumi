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

// Watch is a specification of a Kubernetes watch along with a URL to post events to.
type Watch struct {
	idl.NamedResource
	Namespace     string    `lumi:"namespace"`
	ObjType       string    `lumi:"objType"`
	LabelSelector string    `lumi:"labelSelector"`
	FieldSelector string    `lumi:"fieldSelector"`
	Function      *Function `lumi:"function"`
	Target        string    `lumi:"target"` // watch publish target (URL, NATS stream, etc)
}
