package engine

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	goerr "github.com/pkg/errors"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/core"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/errors"
	"github.com/pulumi/pulumi-fabric/pkg/diag/colors"
	"github.com/pulumi/pulumi-fabric/pkg/encoding"
	"github.com/pulumi/pulumi-fabric/pkg/resource/deploy"
	"github.com/pulumi/pulumi-fabric/pkg/resource/environment"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
	"github.com/pulumi/pulumi-fabric/pkg/util/mapper"
	"github.com/pulumi/pulumi-fabric/pkg/workspace"
)

func (eng *Engine) initEnvCmd(name string, pkgarg string) (*envCmdInfo, error) {
	return eng.initEnvCmdName(tokens.QName(name), pkgarg)
}

func (eng *Engine) initEnvCmdName(name tokens.QName, pkgarg string) (*envCmdInfo, error) {
	// If the name is blank, use the default.
	if name == "" {
		name = eng.getCurrentEnv()
	}
	if name == "" {
		return nil, goerr.Errorf("missing environment name (and no default found)")
	}

	// Read in the deployment information, bailing if an IO error occurs.
	target, snapshot, checkpoint := eng.readEnv(name)
	if checkpoint == nil {
		return nil, goerr.Errorf("could not read environment information")
	}

	contract.Assert(target != nil)
	contract.Assert(checkpoint != nil)
	return &envCmdInfo{
		Target:     target,
		Checkpoint: checkpoint,
		Snapshot:   snapshot,
		PackageArg: pkgarg,
	}, nil
}

type envCmdInfo struct {
	Target     *deploy.Target          // the target environment.
	Checkpoint *environment.Checkpoint // the full serialized checkpoint from which this came.
	Snapshot   *deploy.Snapshot        // the environment's latest deployment snapshot
	PackageArg string                  // an optional path to a package to pass to the compiler
}

// createEnv just creates a new empty environment without deploying anything into it.
func (eng *Engine) createEnv(name tokens.QName) {
	env := &deploy.Target{Name: name}
	if success := eng.saveEnv(env, nil, "", false); success {
		fmt.Fprintf(eng.Stdout, "Environment '%v' initialized; see `lumi deploy` to deploy into it\n", name)
		eng.setCurrentEnv(name, false)
	}
}

// newWorkspace creates a new workspace using the current working directory.
func (eng *Engine) newWorkspace() (workspace.W, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ctx := core.NewContext(pwd, eng.Diag(), &core.Options{})
	return workspace.New(ctx)
}

// getCurrentEnv reads the current environment.
func (eng *Engine) getCurrentEnv() tokens.QName {
	var name tokens.QName
	w, err := eng.newWorkspace()
	if err == nil {
		name = w.Settings().Env
	}
	if err != nil {
		eng.Diag().Errorf(errors.ErrorIO, err)
	}
	return name
}

// setCurrentEnv changes the current environment to the given environment name, issuing an error if it doesn't exist.
func (eng *Engine) setCurrentEnv(name tokens.QName, verify bool) {
	if verify {
		if _, _, checkpoint := eng.readEnv(name); checkpoint == nil {
			return // no environment by this name exists, bail out.
		}
	}

	// Switch the current workspace to that environment.
	w, err := eng.newWorkspace()
	if err == nil {
		w.Settings().Env = name
		err = w.Save()
	}
	if err != nil {
		eng.Diag().Errorf(errors.ErrorIO, err)
	}
}

// removeTarget permanently deletes the environment's information from the local workstation.
func (eng *Engine) removeTarget(env *deploy.Target) {
	deleteTarget(env)
	msg := fmt.Sprintf("%sEnvironment '%s' has been removed!%s\n",
		colors.SpecAttention, env.Name, colors.Reset)
	fmt.Fprint(eng.Stdout, colors.ColorizeText(msg))
}

// backupTarget makes a backup of an existing file, in preparation for writing a new one.  Instead of a copy, it
// simply renames the file, which is simpler, more efficient, etc.
func backupTarget(file string) {
	contract.Require(file != "", "file")
	err := os.Rename(file, file+".bak")
	contract.IgnoreError(err) // ignore errors.
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
func (eng *Engine) readEnv(name tokens.QName) (*deploy.Target, *deploy.Snapshot, *environment.Checkpoint) {
	contract.Require(name != "", "name")
	file := workspace.EnvPath(name)

	// Detect the encoding of the file so we can do our initial unmarshaling.
	m, ext := encoding.Detect(file)
	if m == nil {
		eng.Diag().Errorf(errors.ErrorIllegalMarkupExtension, ext)
		return nil, nil, nil
	}

	// Now read the whole file into a byte blob.
	b, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			eng.Diag().Errorf(errors.ErrorInvalidEnvName, name)
		} else {
			eng.Diag().Errorf(errors.ErrorIO, err)
		}
		return nil, nil, nil
	}

	// Unmarshal the contents into a checkpoint structure.
	var checkpoint environment.Checkpoint
	if err = m.Unmarshal(b, &checkpoint); err != nil {
		eng.Diag().Errorf(errors.ErrorCantReadDeployment, file, err)
		return nil, nil, nil
	}

	// Next, use the mapping infrastructure to validate the contents.
	// IDEA: we can eliminate this redundant unmarshaling once Go supports strict unmarshaling.
	var obj map[string]interface{}
	if err = m.Unmarshal(b, &obj); err != nil {
		eng.Diag().Errorf(errors.ErrorCantReadDeployment, file, err)
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
		eng.Diag().Errorf(errors.ErrorCantReadDeployment, file, err)
		return nil, nil, nil
	}

	target, snapshot := environment.DeserializeCheckpoint(&checkpoint)
	contract.Assert(target != nil)
	return target, snapshot, &checkpoint
}

// saveEnv saves a new snapshot at the given location, backing up any existing ones.
func (eng *Engine) saveEnv(env *deploy.Target, snap *deploy.Snapshot, file string, existok bool) bool {
	contract.Require(env != nil, "env")
	if file == "" {
		file = workspace.EnvPath(env.Name)
	}

	// Make a serializable LumiGL data structure and then use the encoder to encode it.
	m, ext := encoding.Detect(file)
	if m == nil {
		eng.Diag().Errorf(errors.ErrorIllegalMarkupExtension, ext)
		return false
	}
	if filepath.Ext(file) == "" {
		file = file + ext
	}
	dep := environment.SerializeCheckpoint(env, snap)
	b, err := m.Marshal(dep)
	if err != nil {
		eng.Diag().Errorf(errors.ErrorIO, err)
		return false
	}

	// If it's not ok for the file to already exist, ensure that it doesn't.
	if !existok {
		if _, staterr := os.Stat(file); staterr == nil {
			eng.Diag().Errorf(errors.ErrorIO, goerr.Errorf("file '%v' already exists", file))
			return false
		}
	}

	// Back up the existing file if it already exists.
	backupTarget(file)

	// Ensure the directory exists.
	if err = os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		eng.Diag().Errorf(errors.ErrorIO, err)
		return false
	}

	// And now write out the new snapshot file, overwriting that location.
	if err = ioutil.WriteFile(file, b, 0600); err != nil {
		eng.Diag().Errorf(errors.ErrorIO, err)
		return false
	}

	return true
}
