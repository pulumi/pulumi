package diy

import diy "github.com/pulumi/pulumi/sdk/v3/pkg/backend/diy"

// Bucket is a wrapper around an underlying gocloud blob.Bucket.  It ensures that we pass all paths
// to it normalized to forward-slash form like it requires.
type Bucket = diy.Bucket

