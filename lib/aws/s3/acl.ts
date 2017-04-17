// Copyright 2017 Pulumi, Inc. All rights reserved.

// CannedACL is a predefined Amazon S3 grant.  Each canned ACL value has a predefined set of grantees and permissions.
export type CannedACL =
    "private"                   | // Owner gets `FULL_CONTROL`.  Noone else has access rights (default).
    "public-read"               | // Owner gets `FULL_CONTROL`.  The `AllUsers` group gets `READ` access.
    "public-read-write"         | // Owner gets `FULL_CONTROL`.  The `AllUsers` group gets `READ` and `WRITE` access.
    "aws-exec-read"             | // Owner gets `FULL_CONTROL`.  Amazon EC2 gets `READ` access to `GET` an AMI bundle.
    "authenticated-read"        | // Owner gets `FULL_CONTROL`.  The `AuthenticatedUsers` group gets `READ` access.
    "bucket-owner-read"         | // Object owner gets `FULL_CONTROL`.  Bucket owner gets `READ` access.
    "bucket-owner-full-control" | // Both object and bucket owner get `FULL_CONTROL` over the object. 
    "log-delivery-write"        ; // The `LogDelivery` group gets `WRITE` and `READ_ACP` permissions on this bucket.

