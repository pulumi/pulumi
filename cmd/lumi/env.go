// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	goerr "github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/diag/colors"
	"github.com/pulumi/lumi/pkg/encoding"
	"github.com/pulumi/lumi/pkg/resource/deploy"
	"github.com/pulumi/lumi/pkg/resource/environment"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/pkg/workspace"
)

func newEnvCmd() *cobra.Command {
	var showIDs bool
	var showURNs bool
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage target environments",
		Long: "Manage target environments\n" +
			"\n" +
			"An environment is a named deployment target, and a single project may have many of them.\n" +
			"Each environment has a configuration and deployment history associated with it, stored in\n" +
			"the workspace, in addition to a full checkpoint of the last known good deployment.\n",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			curr := getCurrentEnv()
			if curr == "" {
				return goerr.New("no current environment; either `lumi env init` or `lumi env select` one")
			}
			fmt.Printf("Current environment is %v\n", curr)
			fmt.Printf("    (use `lumi env select` to change environments; `lumi env ls` lists known ones)\n")
			target, snapshot, checkpoint := readEnv(curr)
			if checkpoint == nil {
				return goerr.Errorf("could not read environment information")
			}
			if checkpoint.Latest != nil {
				fmt.Printf("Last deployment at %v\n", checkpoint.Latest.Time)
				if checkpoint.Latest.Info != nil {
					fmt.Printf("Additional deployment info: %v\n", checkpoint.Latest.Info)
				}
			}
			if target.Config != nil && len(target.Config) > 0 {
				fmt.Printf("%v configuration variables set (see `lumi config` for details)\n", len(target.Config))
			}
			if len(snapshot.Resources) == 0 {
				fmt.Printf("No resources currently in this environment\n")
			} else {
				fmt.Printf("%v resources currently in this environment:\n", len(snapshot.Resources))
				fmt.Printf("\n")
				fmt.Printf("%-48s %s\n", "TYPE", "NAME")
				for _, res := range snapshot.Resources {
					fmt.Printf("%-48s %s\n", res.Type(), res.URN().Name())

					// If the ID and/or URN is requested, show it on the following line.  It would be nice to do this
					// on a single line, but they can get quite lengthy and so this formatting makes more sense.
					if showIDs {
						fmt.Printf("\tID: %s\n", res.ID())
					}
					if showURNs {
						fmt.Printf("\tURN: %s\n", res.URN())
					}
				}
			}
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&showIDs, "show-ids", "i", false, "Display each resource's provider-assigned unique ID")
	cmd.PersistentFlags().BoolVarP(
		&showURNs, "show-urns", "u", false, "Display each resource's Lumi-assigned globally unique URN")

	cmd.AddCommand(newEnvInitCmd())
	cmd.AddCommand(newEnvLsCmd())
	cmd.AddCommand(newEnvRmCmd())
	cmd.AddCommand(newEnvSelectCmd())

	return cmd
}

func initEnvCmd(cmd *cobra.Command, args []string) (*envCmdInfo, error) {
	// Read in the name of the environment to use.
	if len(args) == 0 || args[0] == "" {
		return nil, goerr.Errorf("missing required environment name")
	}
	return initEnvCmdName(tokens.QName(args[0]), args[1:])
}

func initEnvCmdName(name tokens.QName, args []string) (*envCmdInfo, error) {
	// If the name is blank, use the default.
	if name == "" {
		name = getCurrentEnv()
	}
	if name == "" {
		return nil, goerr.Errorf("missing environment name (and no default found)")
	}

	// Read in the deployment information, bailing if an IO error occurs.
	target, snapshot, checkpoint := readEnv(name)
	if checkpoint == nil {
		return nil, goerr.Errorf("could not read environment information")
	}
	contract.Assert(target != nil)
	contract.Assert(checkpoint != nil)
	return &envCmdInfo{
		Target:     target,
		Checkpoint: checkpoint,
		Snapshot:   snapshot,
		Args:       args,
	}, nil
}

type envCmdInfo struct {
	Target     *deploy.Target          // the target environment.
	Checkpoint *environment.Checkpoint // the full serialized checkpoint from which this came.
	Snapshot   *deploy.Snapshot        // the environment's latest deployment snapshot
	Args       []string                // the args after extracting the environment name
}

func confirmPrompt(msg string, name tokens.QName) bool {
	prompt := fmt.Sprintf(msg, name)
	fmt.Print(
		colors.ColorizeText(fmt.Sprintf("%v%v%v\n", colors.SpecAttention, prompt, colors.Reset)))
	fmt.Printf("Please confirm that this is what you'd like to do by typing (\"%v\"): ", name)
	reader := bufio.NewReader(os.Stdin)
	if line, _ := reader.ReadString('\n'); line != string(name)+"\n" {
		fmt.Fprintf(os.Stderr, "Confirmation declined -- exiting without doing anything\n")
		return false
	}
	return true
}

// createEnv just creates a new empty environment without deploying anything into it.
func createEnv(name tokens.QName) {
	env := &deploy.Target{Name: name}
	if success := saveEnv(env, nil, "", false); success {
		fmt.Printf("Environment '%v' initialized; see `lumi deploy` to deploy into it\n", name)
		setCurrentEnv(name, false)
	}
}

// newWorkspace creates a new workspace using the current working directory.
func newWorkspace() (workspace.W, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ctx := core.NewContext(pwd, nil, &core.Options{})
	return workspace.New(ctx)
}

// getCurrentEnv reads the current environment.
func getCurrentEnv() tokens.QName {
	var name tokens.QName
	w, err := newWorkspace()
	if err == nil {
		name = w.Settings().Env
	}
	if err != nil {
		cmdutil.Diag().Errorf(errors.ErrorIO, err)
	}
	return name
}

// setCurrentEnv changes the current environment to the given environment name, issuing an error if it doesn't exist.
func setCurrentEnv(name tokens.QName, verify bool) {
	if verify {
		if _, _, checkpoint := readEnv(name); checkpoint == nil {
			return // no environment by this name exists, bail out.
		}
	}

	// Switch the current workspace to that environment.
	w, err := newWorkspace()
	if err == nil {
		w.Settings().Env = name
		err = w.Save()
	}
	if err != nil {
		cmdutil.Diag().Errorf(errors.ErrorIO, err)
	}
}

// removeTarget permanently deletes the environment's information from the local workstation.
func removeTarget(env *deploy.Target) {
	deleteTarget(env)
	msg := fmt.Sprintf("%sEnvironment '%s' has been removed!%s\n",
		colors.SpecAttention, env.Name, colors.Reset)
	fmt.Print(colors.ColorizeText(msg))
}

// backupTarget makes a backup of an existing file, in preparation for writing a new one.  Instead of a copy, it
// simply renames the file, which is simpler, more efficient, etc.
func backupTarget(file string) {
	contract.Require(file != "", "file")
	if err := os.Rename(file, file+".bak"); err != nil {
		// ignore errors.
	}
	// IDEA: consider multiple backups (.bak.bak.bak...etc).
}

// deleteTarget removes an existing snapshot file, leaving behind a backup.
func deleteTarget(env *deploy.Target) {
	contract.Require(env != nil, "env")
	// Just make a backup of the file and don't write out anything new.
	file := workspace.EnvPath(env.Name)
	backupTarget(file)
}

// readEnv reads in an existing snapshot file, issuing an error and returning nil if something goes awry.
func readEnv(name tokens.QName) (*deploy.Target, *deploy.Snapshot, *environment.Checkpoint) {
	contract.Require(name != "", "name")
	file := workspace.EnvPath(name)

	// Detect the encoding of the file so we can do our initial unmarshaling.
	m, ext := encoding.Detect(file)
	if m == nil {
		cmdutil.Diag().Errorf(errors.ErrorIllegalMarkupExtension, ext)
		return nil, nil, nil
	}

	// Now read the whole file into a byte blob.
	b, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			cmdutil.Diag().Errorf(errors.ErrorInvalidEnvName, name)
		} else {
			cmdutil.Diag().Errorf(errors.ErrorIO, err)
		}
		return nil, nil, nil
	}

	// Unmarshal the contents into a checkpoint structure.
	var checkpoint environment.Checkpoint
	if err = m.Unmarshal(b, &checkpoint); err != nil {
		cmdutil.Diag().Errorf(errors.ErrorCantReadDeployment, file, err)
		return nil, nil, nil
	}

	// Next, use the mapping infrastructure to validate the contents.
	// IDEA: we can eliminate this redundant unmarshaling once Go supports strict unmarshaling.
	var obj map[string]interface{}
	if err = m.Unmarshal(b, &obj); err != nil {
		cmdutil.Diag().Errorf(errors.ErrorCantReadDeployment, file, err)
		return nil, nil, nil
	}

	if obj["latest"] != nil {
		if latest, islatest := obj["latest"].(map[string]interface{}); islatest {
			delete(latest, "resources") // remove the resources, since they require custom marshaling.
		}
	}
	md := mapper.New(nil)
	var ignore environment.Checkpoint // just for errors.
	if err = md.Decode(obj, &ignore); err != nil {
		cmdutil.Diag().Errorf(errors.ErrorCantReadDeployment, file, err)
		return nil, nil, nil
	}

	target, snapshot := environment.DeserializeCheckpoint(&checkpoint)
	contract.Assert(target != nil)
	return target, snapshot, &checkpoint
}

// saveEnv saves a new snapshot at the given location, backing up any existing ones.
func saveEnv(env *deploy.Target, snap *deploy.Snapshot, file string, existok bool) bool {
	contract.Require(env != nil, "env")
	if file == "" {
		file = workspace.EnvPath(env.Name)
	}

	// Make a serializable LumiGL data structure and then use the encoder to encode it.
	m, ext := encoding.Detect(file)
	if m == nil {
		cmdutil.Diag().Errorf(errors.ErrorIllegalMarkupExtension, ext)
		return false
	}
	if filepath.Ext(file) == "" {
		file = file + ext
	}
	dep := environment.SerializeCheckpoint(env, snap)
	b, err := m.Marshal(dep)
	if err != nil {
		cmdutil.Diag().Errorf(errors.ErrorIO, err)
		return false
	}

	// If it's not ok for the file to already exist, ensure that it doesn't.
	if !existok {
		if _, staterr := os.Stat(file); staterr == nil {
			cmdutil.Diag().Errorf(errors.ErrorIO, goerr.Errorf("file '%v' already exists", file))
			return false
		}
	}

	// Back up the existing file if it already exists.
	backupTarget(file)

	// Ensure the directory exists.
	if err = os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		cmdutil.Diag().Errorf(errors.ErrorIO, err)
		return false
	}

	// And now write out the new snapshot file, overwriting that location.
	if err = ioutil.WriteFile(file, b, 0600); err != nil {
		cmdutil.Diag().Errorf(errors.ErrorIO, err)
		return false
	}

	return true
}
