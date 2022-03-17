// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"io"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// LanguageRuntime is a convenient interface for interacting with language runtime plugins.  These tend to be
// dynamically loaded as plugins, although this interface hides this fact from the calling code.
type LanguageRuntime interface {
	// Closer closes any underlying OS resources associated with this plugin (like processes, RPC channels, etc).
	io.Closer
	// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
	GetRequiredPlugins(info ProgInfo) ([]workspace.PluginInfo, error)
	// Run executes a program in the language runtime for planning or deployment purposes.  If
	// info.DryRun is true, the code must not assume that side-effects or final values resulting
	// from resource deployments are actually available.  If it is false, on the other hand, a real
	// deployment is occurring and it may safely depend on these.
	//
	// Returns a triple of "error message", "bail", or real "error".  If "bail", the caller should
	// return result.Bail immediately and not print any further messages to the user.
	Run(info RunInfo) (string, bool, error)
	// GetPluginInfo returns this plugin's information.
	GetPluginInfo() (workspace.PluginInfo, error)
}

// ProgInfo contains minimal information about the program to be run.
type ProgInfo struct {
	Proj    *workspace.Project // the program project/package.
	Pwd     string             // the program's working directory.
	Program string             // the path to the program to execute.
}

// RunInfo contains all of the information required to perform a plan or deployment operation.
type RunInfo struct {
	MonitorAddress   string                // the RPC address to the host resource monitor.
	Project          string                // the project name housing the program being run.
	Stack            string                // the stack name being evaluated.
	Pwd              string                // the program's working directory.
	Program          string                // the path to the program to execute.
	Args             []string              // any arguments to pass to the program.
	Config           map[config.Key]string // the configuration variables to apply before running.
	ConfigSecretKeys []config.Key          // the configuration keys that have secret values.
	DryRun           bool                  // true if we are performing a dry-run (preview).
	QueryMode        bool                  // true if we're only doing a query.
	Parallel         int                   // the degree of parallelism for resource operations (<=1 for serial).
}
