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

package elasticbeanstalk

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// ApplicationVersion is an application version, an iteration of deployable code, for an Elastic Beanstalk application.
// For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-beanstalk-environment.html.
type Environment struct {
	idl.NamedResource
	// The name of the application that is associated with this environment.
	Application *Application `lumi:"application,replaces"`
	// A prefix for your Elastic Beanstalk environment URL.
	CNAMEPrefix *string `lumi:"cnamePrefix,optional,replaces"`
	// A description that helps you identify this environment.
	Description *string `lumi:"description,optional"`
	// A name for the Elastic Beanstalk environment.
	EnvironmentName *string `lumi:"environmentName,optional,replaces"`
	// Key-value pairs defining configuration options for this environment, such as the instance type. These options
	// override the values that are defined in the solution stack or the configuration template. If you remove any
	// options during a stack update, the removed options revert to default values.
	OptionSettings *[]OptionSetting `lumi:"optionSettings,optional"`
	// The name of an Elastic Beanstalk solution stack that this configuration will use. For more information, see
	// http://docs.aws.amazon.com/elasticbeanstalk/latest/dg/concepts.platforms.html. You must specify either this
	// parameter or an Elastic Beanstalk configuration template name.
	SolutionStackName *string `lumi:"solutionStackName,optional,replaces"`
	// An arbitrary set of tags (key–value pairs) for this environment.
	Tags *[]Tag `lumi:"tags,optional,replaces"`
	// The name of the Elastic Beanstalk configuration template to use with the environment. You must specify either
	// this parameter or a solution stack name.
	TemplateName *string `lumi:"templateName,optional"`
	// Specifies the tier to use in creating this environment. The environment tier that you choose determines whether
	// Elastic Beanstalk provisions resources to support a web application that handles HTTP(S) requests or a web
	// application that handles background-processing tasks.
	Tier *Tier `lumi:"tier,optional,replaces"`
	// The version to associate with the environment.
	Version *ApplicationVersion `lumi:"version,optional"`
	// The URL to the load balancer for this environment.
	EndpointURL string `lumi:"endpointURL,out"`
	// Key-value pairs defining all of the configuration options for this environment, including both values provided
	// in the OptionSettings input, as well as settings with default values.
	AllOptionSettings *[]OptionSetting `lumi:"allOptionSettings,out"`
}

// OptionSetting specifies options for an Elastic Beanstalk environment.
type OptionSetting struct {
	// A unique namespace identifying the option's associated AWS resource. For a list of namespaces that you can use,
	// see http://docs.aws.amazon.com/elasticbeanstalk/latest/dg/command-options.html.
	Namespace string `lumi:"namespace"`
	// The name of the configuration option. For a list of options that you can use, see
	// http://docs.aws.amazon.com/elasticbeanstalk/latest/dg/command-options.html.
	OptionName string `lumi:"optionName"`
	// The value of the setting.
	Value string `lumi:"value"`
}

// A Tag helps to identify and categorize resources.
type Tag struct {
	// The key name of the tag. You can specify a value that is 1 to 127 Unicode characters in length and cannot be
	// prefixed with aws:. You can use any of the following characters: the set of Unicode letters, digits, whitespace,
	// _, ., /, =, +, and -.
	Key string `lumi:"key"`
	// The value for the tag. You can specify a value that is 1 to 255 Unicode characters in length and cannot be
	// prefixed with aws:. You can use any of the following characters: the set of Unicode letters, digits, whitespace,
	// _, ., /, =, +, and -.
	Value string `lumi:"value"`
}

// The Tier for an Elastic Beanstalk Environment.
type Tier string

const (
	WebServerTier Tier = "WebServer::Standard::1.0"
	WorkerTier    Tier = "Worker::SQS/HTTP::1.0"
)
