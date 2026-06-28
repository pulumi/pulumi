// Standalone module for the OCI registry proxy (a smoke-test dev tool). Kept out of
// pkg/ so go-containerregistry does not enter the product module graph.
module github.com/pulumi/pulumi/pkg/oci/smoketest/registry-proxy

go 1.25

require github.com/google/go-containerregistry v0.20.2

require (
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc3 // indirect
	github.com/vbatts/tar-split v0.11.3 // indirect
	golang.org/x/sync v0.2.0 // indirect
)
