// Copyright 2016-2020, Pulumi Corporation.
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

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/pkg/v2/engine"
	resourceanalyzer "github.com/pulumi/pulumi/pkg/v2/resource/analyzer"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/nodejs/npm"
	"github.com/pulumi/pulumi/sdk/v2/python"
)

// PublishOperation publishes a PolicyPack to the backend.
type PublishOperation struct {
	Root       string
	PlugCtx    *plugin.Context
	PolicyPack *workspace.PolicyPackProject
	Scopes     CancellationScopeSource
}

// PolicyPackOperation is used to make various operations against a Policy Pack.
type PolicyPackOperation struct {
	// If nil, the latest version is assumed.
	VersionTag *string
	Scopes     CancellationScopeSource
	Config     map[string]*json.RawMessage
}

// PolicyPack represents a Pulumi Policy Pack.
type PolicyPack struct {
	id backend.PolicyPackIdentifier
	// b is a pointer to the backend that this PolicyPack belongs to.
	b *Backend
	// cl is the client used to interact with the backend.
	cl backend.PolicyClient
}

func (pack *PolicyPack) ID() backend.PolicyPackIdentifier {
	return pack.id
}

func (pack *PolicyPack) Backend() *Backend {
	return pack.b
}

func (pack *PolicyPack) Publish(ctx context.Context, op PublishOperation) result.Result {

	//
	// Get PolicyPack metadata from the plugin.
	//

	fmt.Println("Obtaining policy metadata from policy plugin")

	abs, err := filepath.Abs(op.PlugCtx.Pwd)
	if err != nil {
		return result.FromError(err)
	}

	analyzer, err := op.PlugCtx.Host.PolicyAnalyzer(tokens.QName(abs), op.PlugCtx.Pwd, nil /*opts*/)
	if err != nil {
		return result.FromError(err)
	}

	analyzerInfo, err := analyzer.GetAnalyzerInfo()
	if err != nil {
		return result.FromError(err)
	}

	// Update the name and version tag from the metadata.
	pack.id.Name = analyzerInfo.Name
	pack.id.VersionTag = analyzerInfo.Version

	fmt.Println("Compressing policy pack")

	var packTarball []byte

	// TODO[pulumi/pulumi#1334]: move to the language plugins so we don't have to hard code here.
	runtime := op.PolicyPack.Runtime.Name()
	if strings.EqualFold(runtime, "nodejs") {
		packTarball, err = npm.Pack(op.PlugCtx.Pwd, os.Stderr)
		if err != nil {
			return result.FromError(
				errors.Wrap(err, "could not publish policies because of error running npm pack"))
		}
	} else if strings.EqualFold(runtime, "python") {
		// npm pack puts all the files in a "package" subdirectory inside the .tgz it produces, so we'll do
		// the same for Python. That way, after unpacking, we can look for the PulumiPolicy.yaml inside the
		// package directory to determine the runtime of the policy pack.
		packTarball, err = archive.TGZ(op.PlugCtx.Pwd, "package", true /*useDefaultExcludes*/)
		if err != nil {
			return result.FromError(
				errors.Wrap(err, "could not publish policies because of error creating the .tgz"))
		}
	} else {
		return result.Errorf(
			"failed to publish policies because PulumiPolicy.yaml specifies an unsupported runtime %s",
			runtime)
	}

	//
	// Publish.
	//

	fmt.Println("Uploading policy pack to Pulumi service")

	publishedVersion, err := pack.cl.PublishPolicyPack(ctx, pack.id.OrgName, analyzerInfo, bytes.NewReader(packTarball))
	if err != nil {
		return result.FromError(err)
	}

	fmt.Printf("\nPermalink: %s/%s\n", pack.id.URL(), publishedVersion)

	return nil
}

func (pack *PolicyPack) Enable(ctx context.Context, policyGroup string, op PolicyPackOperation) error {
	versionTag := ""
	if op.VersionTag != nil {
		versionTag = *op.VersionTag
	}
	return pack.cl.EnablePolicyPack(ctx, pack.id.OrgName, policyGroup, pack.id.Name, versionTag, op.Config)
}

func (pack *PolicyPack) Validate(ctx context.Context, op PolicyPackOperation) error {
	schema, err := pack.cl.GetPolicyPackSchema(ctx, pack.id.OrgName, pack.id.Name, *op.VersionTag)
	if err != nil {
		return err
	}
	err = resourceanalyzer.ValidatePolicyPackConfig(schema.ConfigSchema, op.Config)
	if err != nil {
		return err
	}
	return nil
}

func (pack *PolicyPack) Disable(ctx context.Context, policyGroup string, op PolicyPackOperation) error {
	versionTag := ""
	if op.VersionTag != nil {
		versionTag = *op.VersionTag
	}
	return pack.cl.DisablePolicyPack(ctx, pack.id.OrgName, policyGroup, pack.id.Name, versionTag)
}

func (pack *PolicyPack) Remove(ctx context.Context, op PolicyPackOperation) error {
	versionTag := ""
	if op.VersionTag != nil {
		versionTag = *op.VersionTag
	}
	return pack.cl.DeletePolicyPack(ctx, pack.id.OrgName, pack.id.Name, versionTag)
}

type requiredPolicy struct {
	meta    apitype.RequiredPolicy
	orgName string
	client  backend.Client
}

var _ engine.RequiredPolicy = (*requiredPolicy)(nil)

func (pack *requiredPolicy) Name() string                        { return pack.meta.Name }
func (pack *requiredPolicy) Version() string                     { return strconv.Itoa(pack.meta.Version) }
func (pack *requiredPolicy) Config() map[string]*json.RawMessage { return pack.meta.Config }

func (pack *requiredPolicy) Install(ctx context.Context) (string, error) {
	// If version tag is empty, we use the version tag. This is to support older version of
	// pulumi/policy that do not have a version tag.
	version := pack.meta.VersionTag
	if version == "" {
		version = pack.Version()
	}
	policyPackPath, installed, err := workspace.GetPolicyPath(pack.orgName,
		strings.Replace(pack.meta.Name, tokens.QNameDelimiter, "_", -1), version)
	if err != nil {
		// Failed to get a sensible PolicyPack path.
		return "", err
	} else if installed {
		// We've already downloaded and installed the PolicyPack. Return.
		return policyPackPath, nil
	}

	fmt.Printf("Installing policy pack %s %s...\n", pack.meta.Name, version)

	client, ok := pack.client.(backend.PolicyClient)
	if !ok {
		return "", fmt.Errorf("the %v client does not support policy packs", pack.client.Name())
	}

	// PolicyPack has not been downloaded and installed. Do this now.
	policyPackTarball, err := client.GetPolicyPack(ctx, pack.meta.PackLocation)
	if err != nil {
		return "", err
	}

	return policyPackPath, installRequiredPolicy(policyPackPath, policyPackTarball)
}

func installRequiredPolicy(finalDir string, tarball []byte) error {
	const packageDir = "package"

	// If part of the directory tree is missing, ioutil.TempDir will return an error, so make sure
	// the path we're going to create the temporary folder in actually exists.
	if err := os.MkdirAll(filepath.Dir(finalDir), 0700); err != nil {
		return errors.Wrap(err, "creating plugin root")
	}

	tempDir, err := ioutil.TempDir(filepath.Dir(finalDir), fmt.Sprintf("%s.tmp", filepath.Base(finalDir)))
	if err != nil {
		return errors.Wrapf(err, "creating plugin directory %s", tempDir)
	}

	// The policy pack files are actually in a directory called `package`.
	tempPackageDir := filepath.Join(tempDir, packageDir)
	if err := os.MkdirAll(tempPackageDir, 0700); err != nil {
		return errors.Wrap(err, "creating plugin root")
	}

	// If we early out of this function, try to remove the temp folder we created.
	defer func() {
		contract.IgnoreError(os.RemoveAll(tempDir))
	}()

	// Uncompress the policy pack.
	err = archive.UnTGZ(tarball, tempDir)
	if err != nil {
		return err
	}

	logging.V(7).Infof("Unpacking policy pack %q %q\n", tempDir, finalDir)

	// If two calls to `plugin install` for the same plugin are racing, the second one will be
	// unable to rename the directory. That's OK, just ignore the error. The temp directory created
	// as part of the install will be cleaned up when we exit by the defer above.
	if err := os.Rename(tempPackageDir, finalDir); err != nil && !os.IsExist(err) {
		return errors.Wrap(err, "moving plugin")
	}

	projPath := filepath.Join(finalDir, "PulumiPolicy.yaml")
	proj, err := workspace.LoadPolicyPack(projPath)
	if err != nil {
		return errors.Wrapf(err, "failed to load policy project at %s", finalDir)
	}

	// TODO[pulumi/pulumi#1334]: move to the language plugins so we don't have to hard code here.
	if strings.EqualFold(proj.Runtime.Name(), "nodejs") {
		if err := completeNodeJSInstall(finalDir); err != nil {
			return err
		}
	} else if strings.EqualFold(proj.Runtime.Name(), "python") {
		if err := completePythonInstall(finalDir, projPath, proj); err != nil {
			return err
		}
	}

	fmt.Println("Finished installing policy pack")
	fmt.Println()

	return nil
}

func completeNodeJSInstall(finalDir string) error {
	if bin, err := npm.Install(finalDir, nil, os.Stderr); err != nil {
		return errors.Wrapf(
			err,
			"failed to install dependencies of policy pack; you may need to re-run `%s install` "+
				"in %q before this policy pack works", bin, finalDir)
	}

	return nil
}

func completePythonInstall(finalDir, projPath string, proj *workspace.PolicyPackProject) error {
	const venvDir = "venv"
	if err := python.InstallDependencies(finalDir, venvDir, false /*showOutput*/); err != nil {
		return err
	}

	// Save project with venv info.
	proj.Runtime.SetOption("virtualenv", venvDir)
	if err := proj.Save(projPath); err != nil {
		return errors.Wrapf(err, "saving project at %s", projPath)
	}

	return nil
}
