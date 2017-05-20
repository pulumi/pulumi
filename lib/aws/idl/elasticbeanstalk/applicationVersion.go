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

package elasticbeanstalk

import (
	"github.com/pulumi/lumi/lib/aws/idl/s3"
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// ApplicationVersion is an application version, an iteration of deployable code, for an Elastic Beanstalk application.
// For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-beanstalk-environment.html.
type ApplicationVersion struct {
	idl.NamedResource
	// Name of the Elastic Beanstalk application that is associated with this application version.
	Application *Application `lumi:"application,replaces"`
	// A description of this application version.
	Description *string `lumi:"description,optional"`
	// The source bundle for this application version. This supports all the usual Lumi asset schemes, in addition
	// to Amazon Simple Storage Service (S3) bucket locations, indicating with a URI scheme of s3//<bucket>/<object>.
	SourceBundle *s3.Object `lumi:"sourceBundle,replaces"`
}
