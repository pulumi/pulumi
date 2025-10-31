package diy

import diy "github.com/pulumi/pulumi/sdk/v3/pkg/backend/diy"

// DisableIntegrityChecking can be set to true to disable checkpoint state integrity verification.  This is not
// recommended, because it could mean proceeding even in the face of a corrupted checkpoint state file, but can
// be used as a last resort when a command absolutely must be run.
var DisableIntegrityChecking = diy.DisableIntegrityChecking

