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

package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/host"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newAboutCmd() *cobra.Command {
	short := "Print information about the pulumi enviroment."
	cmd :=
		&cobra.Command{
			Use:   "about",
			Short: short,
			Long: short + "\n" +
				"\n" +
				"TODO",
			Args: cmdutil.MaximumNArgs(0),
			Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
				var err error

				fmt.Printf("CLI Version:%v\n", version.Version)
				fmt.Print("\n")

				if err = formatPluginAbout(); err != nil {
					fmt.Printf("Failed to get plugin versions: %s\n")
				}
				fmt.Print("\n")

				if err = formatHostAbout(); err != nil {
					fmt.Printf("Failed to get host version information: %s\n", err)
				}
				fmt.Print("\n")

				var proj *workspace.Project
				var pwd string
				if proj, pwd, err = readProject(); err != nil {
					fmt.Printf("%s\n", err)
				} else {
					fmt.Printf("This is a %s project.\n\n", proj.Runtime.Name())

					if err = formatProgramDependenciesAbout(proj.Runtime.Name(), pwd); err != nil {
						fmt.Printf("Failed to get information about the puluimi program's plugins: %s", err)
					}
					fmt.Print("\n")
				}

				var backend backend.Backend
				backend, err = currentBackend(display.Options{Color: cmdutil.GetGlobalColorization()})
				if err != nil {
					fmt.Print("Could not access the backend: %s", err)
				} else {
					if err = formatCurrentStackAbout(backend); err != nil {
						fmt.Printf("Failed to get information about the host: %s\n", err)
					}
					fmt.Print("\n")

					if err = formatBackendAbout(backend); err != nil {
						fmt.Printf("Failed to gather information on the current backend: %s\n", err)
					}

				}

				return errors.New("I'm just testing this\n")
			},
			),
		}

	return cmd
}

func formatPluginAbout() error {
	var plugins []workspace.PluginInfo
	var err error
	plugins, err = workspace.GetPluginsWithMetadata()
	if err != nil {
		return err
	}
	fmt.Print("Plugins\n")
	sort.Slice(plugins, func(i, j int) bool {
		pi, pj := plugins[i], plugins[j]
		if pi.Name < pj.Name {
			return true
		} else if pi.Name == pj.Name && pi.Kind == pj.Kind &&
			(pi.Version == nil || (pj.Version != nil && pi.Version.GT(*pj.Version))) {
			return true
		}
		return false
	})
	rows := []cmdutil.TableRow{}
	for _, plugin := range plugins {
		name := plugin.Name
		version := plugin.Version.String()
		rows = append(rows, cmdutil.TableRow{
			Columns: []string{name, version},
		})
	}
	cmdutil.PrintTable(cmdutil.Table{
		Headers: []string{"PLUGIN", "VERSION"},
		Rows:    rows,
	})
	return nil
}

func formatHostAbout() error {
	var err error
	stats, err := host.Info()
	if err != nil {
		return err
	}

	cmdutil.PrintTable(cmdutil.Table{
		Headers: []string{"Host", ""},
		Rows: simpleTableRows([][]string{
			[]string{"OS", stats.Platform},
			[]string{"Version", stats.PlatformVersion},
			[]string{"Arch", stats.KernelArch},
		}),
	})

	return nil
}

func formatBackendAbout(b backend.Backend) error {
	var err error
	var currentUser string
	currentUser, err = b.CurrentUser()
	if err != nil {
		currentUser = "Unknown"
	}
	cmdutil.PrintTable(cmdutil.Table{
		Headers: []string{"Backend", ""},
		Rows: simpleTableRows([][]string{
			[]string{"Name", b.Name()},
			[]string{"URL", b.URL()},
			[]string{"User", currentUser},
		}),
	})

	return nil
}

func formatCurrentStackAbout(b backend.Backend) error {
	context := commandContext()
	var stack backend.Stack
	var err error
	stack, err = state.CurrentStack(context, b)
	if err != nil {
		return err
	}
	if stack == nil {
		return errors.New("stack is nil")
	}
	if stack.Ref() == nil {
		return errors.New("Stack.Ref() is nil")
	}
	name := stack.Ref().String()
	var snapshot *deploy.Snapshot
	snapshot, err = stack.Snapshot(context)
	if err != nil {
		return err
	}
	var resources = snapshot.Resources
	var pendingOps = snapshot.PendingOperations

	cmdutil.PrintTable(cmdutil.Table{
		Headers: []string{"Backend", ""},
		Rows: simpleTableRows([][]string{
			[]string{"Name", name},
			[]string{"Resources", strconv.Itoa(len(resources))},
			[]string{"pending Operations", strconv.Itoa(len(pendingOps))},
		}),
	})
	return nil
}

func simpleTableRows(arr [][]string) []cmdutil.TableRow {
	rows := make([]cmdutil.TableRow, len(arr))
	for i, e := range arr {
		rows[i] = cmdutil.TableRow{
			Columns: e,
		}
	}
	return rows
}

func formatProgramDependenciesAbout(language, root string) error {
	fmt.Print("This should just point at requirements.txt\n")
	var depInfo = ""
	switch language {
	case "nodejs":
		depInfo = "package.json"
	case "python":
		depInfo = "requirements.txt"
	case "go":
		depInfo = "go.mod"
	case "dotnet":
		// TODO figure out how dotnet does this
		return errors.New("I don't know about .NET")
	default:
		return errors.New(fmt.Sprintf("Unknown Language: %s", language))
	}

	path := filepath.Join(root, depInfo)

	fmt.Printf("Please include the contents of \"%s\" in your report.\n", path)

	return nil
}
