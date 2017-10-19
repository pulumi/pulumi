// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"io"

	"github.com/pulumi/pulumi/pkg/tokens"
)

// LanguageRuntime is a convenient interface for interacting with language runtime plugins.  These tend to be
// dynamically loaded as plugins, although this interface hides this fact from the calling code.
type LanguageRuntime interface {
	// Closer closes any underlying OS resources associated with this plugin (like processes, RPC channels, etc).
	io.Closer
	// Run executes a program in the language runtime for planning or deployment purposes.  If info.DryRun is true,
	// the code must not assume that side-effects or final values resulting from resource deployments are actually
	// available.  If it is false, on the other hand, a real deployment is occurring and it may safely depend on these.
	Run(info RunInfo) (string, error)
}

// RunInfo contains all of the information required to perform a plan or deployment operation.
type RunInfo struct {
	Project  string                         // the project name housing the program being run.
	Stack    string                         // the stack name being evaluated.
	Pwd      string                         // the program's working directory.
	Program  string                         // the path to the program to execute.
	Args     []string                       // any arguments to pass to the program.
	Config   map[tokens.ModuleMember]string // the configuration variables to apply before running.
	DryRun   bool                           // true if we are performing a dry-run (preview).
	Parallel int                            // the degree of parallelism for resource operations (<=1 for serial).
}
