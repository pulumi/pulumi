package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

// ResourceHookFunction is the shape of a resource hook.
type ResourceHookFunction = deploy.ResourceHookFunction

// ResourceHook represents a resource hook with its (wrapped) callback and options.
type ResourceHook = deploy.ResourceHook

// ResourceHooks is a registry of all resource hooks provided by a program.
type ResourceHooks = deploy.ResourceHooks

func NewResourceHooks(dialOptions DialOptions) *ResourceHooks {
	return deploy.NewResourceHooks(dialOptions)
}

