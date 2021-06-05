package cmd

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/version"
)

func GetVersion() string {
	v := version.Version
	if v == "" {
		v = "v3.4.0" // todo figure out better versioning story. version relies on compiler flag
	}
	return fmt.Sprintf("%v", v)
}
