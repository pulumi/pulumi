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

package s3

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// Object represents an Amazon Simple Storage Service (S3) object (key/value blob).
type Object struct {
	idl.Resource
	// The Key that uniquely identifies this object.
	Key string `lumi:"key,replaces"`
	// The Bucket this object belongs to.
	Bucket *Bucket `lumi:"bucket,replaces"`
	// The Source of content for this object.
	Source *idl.Asset `lumi:"source,replaces,in"`
}
