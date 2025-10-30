package provider

import provider "github.com/pulumi/pulumi/sdk/v3/pkg/resource/provider"

// HostClient is a client interface into the host's engine RPC interface.
type HostClient = provider.HostClient

// NewHostClient dials the target address, connects over gRPC, and returns a client interface.
func NewHostClient(addr string) (*HostClient, error) {
	return provider.NewHostClient(addr)
}

