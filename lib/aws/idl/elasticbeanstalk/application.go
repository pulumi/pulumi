// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
