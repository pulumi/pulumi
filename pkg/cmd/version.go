package cmd

import (
	"fmt"
	"github.com/pulumi/pulumi/pkg/v3/version"
)

func GetVersion() string {
	return fmt.Sprintf("%v", version.Version)
}
