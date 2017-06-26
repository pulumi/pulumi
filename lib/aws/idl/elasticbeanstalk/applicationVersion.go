// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
	// An optional version label name.  If you don't specify one, a unique physical ID will be generated and
	// used instead.  If you specify a name, you cannot perform updates that require replacement of this resource.  You
	// can perform updates that require no or some interruption.  If you must replace the resource, specify a new name.
	VersionLabel *string `lumi:"versionLabel,optional,replaces"`
	// A description of this application version.
	Description *string `lumi:"description,optional"`
	// The source bundle for this application version. This supports all the usual Lumi asset schemes, in addition
	// to Amazon Simple Storage Service (S3) bucket locations, indicating with a URI scheme of s3//<bucket>/<object>.
	SourceBundle *s3.Object `lumi:"sourceBundle,replaces"`
}
