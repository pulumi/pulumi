// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package s3

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// Bucket represents an Amazon Simple Storage Service (Amazon S3) bucket.
// TODO[pulumi/lumi#218]: support all the various configuration settings (CORS, lifecycle, logging, and so on).
type Bucket struct {
	idl.NamedResource
	// BucketName is a name for the bucket.  If you don't specify a name, a unique physical ID is generated.  The name
	// must contain only lowercase letters, numbers, periods (`.`), and dashes (`-`).
	BucketName *string `lumi:"bucketName,replaces,optional"`
	// accessControl is a canned access control list (ACL) that grants predefined permissions to the bucket.
	AccessControl *CannedACL `lumi:"accessControl,optional"`
}
