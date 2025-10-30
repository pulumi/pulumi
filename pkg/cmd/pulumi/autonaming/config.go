package autonaming

import autonaming "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/autonaming"

func ParseAutonamingConfig(s StackContext, cfg config.Map, decrypter config.Decrypter) (autonaming.Autonamer, error) {
	return autonaming.ParseAutonamingConfig(s, cfg, decrypter)
}

