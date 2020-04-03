package test

import (
	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/go/common/diag"
	"github.com/pulumi/pulumi/sdk/go/common/resource/plugin"

	"github.com/pulumi/pulumi/pkg/resource/deploy/deploytest"
)

func NewHost(sink diag.Sink) plugin.Host {
	return deploytest.NewPluginHost(sink, sink, nil,
		deploytest.NewProviderLoader("aws", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return AWS, nil
		}))
}
