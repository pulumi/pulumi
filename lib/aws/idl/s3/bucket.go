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

// Bucket represents an Amazon Simple Storage Service (Amazon S3) bucket.
// TODO: support all the various configuration settings (CORS, lifecycle, logging, and so on).
type Bucket struct {
	idl.NamedResource
	// BucketName is a name for the bucket.  If you don't specify a name, a unique physical ID is generated.  The name
	// must contain only lowercase letters, numbers, periods (`.`), and dashes (`-`).
	BucketName *string `lumi:"bucketName,replaces,optional"`
	// accessControl is a canned access control list (ACL) that grants predefined permissions to the bucket.
	AccessControl *CannedACL `lumi:"accessControl,optional"`
}
