package a

import (
	"os"
	osalias "os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// main is an exempt entrypoint: os.Exit is permitted, including inside nested
// closures within its body.
func main() {
	os.Exit(0)
	cmdutil.Exit(nil)
	defer func() { os.Exit(1) }()
}

// TestMain is an exempt entrypoint.
func TestMain() {
	os.Exit(0)
}

func helper() {
	os.Exit(1) // want `do not call os\.Exit outside of main or TestMain`
}

func nested() {
	if true {
		os.Exit(1) // want `do not call os\.Exit outside of main or TestMain`
	}
}

func aliased() {
	osalias.Exit(1) // want `do not call os\.Exit outside of main or TestMain`
}

func cmdutilExits() {
	cmdutil.Exit(nil)      // want `do not call cmdutil\.Exit outside of main or TestMain`
	cmdutil.ExitError("x") // want `do not call cmdutil\.ExitError outside of main or TestMain`
}

func init() {
	os.Exit(1) // want `do not call os\.Exit outside of main or TestMain`
}

type t struct{}

// A method named main is not the package entrypoint and is not exempt.
func (t) main() {
	os.Exit(1) // want `do not call os\.Exit outside of main or TestMain`
}
