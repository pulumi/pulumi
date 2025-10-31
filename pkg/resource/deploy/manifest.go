package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

// Manifest captures versions for all binaries used to construct this snapshot.
type Manifest = deploy.Manifest

// DeserializeManifest deserializes a typed ManifestV1 into a `deploy.Manifest`.
func DeserializeManifest(m apitype.ManifestV1) (*Manifest, error) {
	return deploy.DeserializeManifest(m)
}

