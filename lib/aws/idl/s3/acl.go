// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package s3

// CannedACL is a predefined Amazon S3 grant.  Each canned ACL value has a predefined set of grantees and permissions.
type CannedACL string

const (
	// Owner gets `FULL_CONTROL`.  Noone else has access rights (default).
	PrivateACL CannedACL = "private"
	// Owner gets `FULL_CONTROL`.  The `AllUsers` group gets `READ` access.
	PublicReadACL CannedACL = "public-read"
	// Owner gets `FULL_CONTROL`.  The `AllUsers` group gets `READ` and `WRITE` access.
	PublicReadWriteACL CannedACL = "public-read-write"
	// Owner gets `FULL_CONTROL`.  Amazon EC2 gets `READ` access to `GET` an AMI bundle.
	AWSExecReadACL CannedACL = "aws-exec-read"
	// Owner gets `FULL_CONTROL`.  The `AuthenticatedUsers` group gets `READ` access.
	AuthenticatedReadACL CannedACL = "authenticated-read"
	// Object owner gets `FULL_CONTROL`.  Bucket owner gets `READ` access.
	BucketOwnerReadACL CannedACL = "bucket-owner-read"
	// Both object and bucket owner get `FULL_CONTROL` over the object.
	BucketOwnerFullControlACL CannedACL = "bucket-owner-full-control"
	// The `LogDelivery` group gets `WRITE` and `READ_ACP` permissions on this bucket.
	LogDeliveryWriteACL CannedACL = "log-delivery-write"
)
