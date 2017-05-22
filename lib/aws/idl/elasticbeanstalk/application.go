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
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// Application is an Elastic Beanstalk application.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-beanstalk-environment.html.
type Application struct {
	idl.NamedResource
	// ApplicationName is a name for the application.  If you don't specify a name, a unique physical ID is used instead.
	ApplicationName *string `lumi:"applicationName,optional,replaces"`
	// An optional description of this application.
	Description *string `lumi:"description,optional"`
}
