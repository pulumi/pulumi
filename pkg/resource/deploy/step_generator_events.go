package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

// ContinueResourceDiffEvent is a step that asks the engine to continue provisioning a resource after completing its
// diff, it is always created from a base RegisterResourceEvent.
type ContinueResourceDiffEvent = deploy.ContinueResourceDiffEvent

// ContinueResourceRefreshEvent is a step that asks the engine to continue provisioning a resource after a
// refresh, it is always created from a base RegisterResourceEvent.
type ContinueResourceRefreshEvent = deploy.ContinueResourceRefreshEvent

// ContinueResourceImportEvent is a step that asks the engine to continue provisioning a resource after an import, it is
// always created from a base RegisterResourceEvent.
type ContinueResourceImportEvent = deploy.ContinueResourceImportEvent

