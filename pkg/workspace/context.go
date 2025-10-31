package workspace

import workspace "github.com/pulumi/pulumi/sdk/v3/pkg/workspace"

// Context is an interface that represents the context of a workspace. It provides access to loading projects and
// plugins.
type Context = workspace.Context

var Instance = workspace.Instance

