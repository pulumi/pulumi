package deploytest

import deploytest "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy/deploytest"

type ResourceMonitor = deploytest.ResourceMonitor

type ResourceHook = deploytest.ResourceHook

type ResourceHookBindings = deploytest.ResourceHookBindings

type ResourceHookFunc = deploytest.ResourceHookFunc

type ResourceOptions = deploytest.ResourceOptions

type RegisterResourceResponse = deploytest.RegisterResourceResponse

func NewResourceMonitor(resmon pulumirpc.ResourceMonitorClient) *ResourceMonitor {
	return deploytest.NewResourceMonitor(resmon)
}

func NewHook(monitor *ResourceMonitor, callbacks *CallbackServer, name string, f ResourceHookFunc, onDryRun bool) (*ResourceHook, error) {
	return deploytest.NewHook(monitor, callbacks, name, f, onDryRun)
}

