package rpcdebug

import rpcdebug "github.com/pulumi/pulumi/sdk/v3/pkg/util/rpcdebug"

type DebugInterceptor = rpcdebug.DebugInterceptor

type DebugInterceptorOptions = rpcdebug.DebugInterceptorOptions

type LogOptions = rpcdebug.LogOptions

func NewDebugInterceptor(opts DebugInterceptorOptions) (*DebugInterceptor, error) {
	return rpcdebug.NewDebugInterceptor(opts)
}

