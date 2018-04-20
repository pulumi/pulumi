package fsutil

import (
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/tokens"
)

// QnamePath just cleans a name and makes sure it's appropriate to use as a path.
func QnamePath(nm tokens.QName) string {
	return strings.Replace(string(nm), tokens.QNameDelimiter, string(os.PathSeparator), -1)
}
