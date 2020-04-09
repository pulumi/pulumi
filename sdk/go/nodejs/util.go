package nodejs

import (
	"github.com/pulumi/pulumi/sdk/go/common/util/cmdutil"
	"os"
)

// PreferYarn returns true if the `PULUMI_PREFER_YARN` environment variable is set.
func PreferYarn() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_PREFER_YARN"))
}
