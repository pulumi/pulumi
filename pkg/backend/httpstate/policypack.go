package httpstate

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/pkg/util/archive"

	"github.com/pulumi/pulumi/pkg/npm"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/result"
	"github.com/pulumi/pulumi/pkg/workspace"
)

type cloudRequiredPolicy struct {
	apitype.RequiredPolicy
	client  *client.Client
	orgName string
}

var _ engine.RequiredPolicy = (*cloudRequiredPolicy)(nil)

func newCloudRequiredPolicy(client *client.Client,
	policy apitype.RequiredPolicy, orgName string) *cloudRequiredPolicy {

	return &cloudRequiredPolicy{
		client:         client,
		RequiredPolicy: policy,
		orgName:        orgName,
	}
}

func (rp *cloudRequiredPolicy) Name() string    { return rp.RequiredPolicy.Name }
func (rp *cloudRequiredPolicy) Version() string { return strconv.Itoa(rp.RequiredPolicy.Version) }
func (rp *cloudRequiredPolicy) OrgName() string { return rp.orgName }

func (rp *cloudRequiredPolicy) Install(ctx context.Context) (string, error) {
	policy := rp.RequiredPolicy

	// If version tag is empty, we use the version tag. This is to support older version of
	// pulumi/policy that do not have a version tag.
	version := policy.VersionTag
	if version == "" {
		version = strconv.Itoa(policy.Version)
	}
	policyPackPath, installed, err := workspace.GetPolicyPath(rp.OrgName(),
		strings.Replace(policy.Name, tokens.QNameDelimiter, "_", -1), version)
	if err != nil {
		// Failed to get a sensible PolicyPack path.
		return "", err
	} else if installed {
		// We've already downloaded and installed the PolicyPack. Return.
		return policyPackPath, nil
	}

	// PolicyPack has not been downloaded and installed. Do this now.
	policyPackTarball, err := rp.client.DownloadPolicyPack(ctx, policy.PackLocation)
	if err != nil {
		return "", err
	}

	return policyPackPath, installRequiredPolicy(policyPackPath, policyPackTarball)
}

func newCloudBackendPolicyPackReference(
	cloudConsoleURL, orgName string, name tokens.QName) *cloudBackendPolicyPackReference {

	return &cloudBackendPolicyPackReference{
		orgName:         orgName,
		name:            name,
		cloudConsoleURL: cloudConsoleURL,
	}
}

// cloudBackendPolicyPackReference is a reference to a PolicyPack implemented by the Pulumi service.
type cloudBackendPolicyPackReference struct {
	// name of the PolicyPack.
	name tokens.QName
	// orgName that administrates the PolicyPack.
	orgName string

	// versionTag of the Policy Pack. This is typically the version specified in
	// a package.json, setup.py, or similar file.
	versionTag string

	// cloudConsoleURL is the root URL of where the Policy Pack can be found in the console. The
	// version must be appended to the returned URL.
	cloudConsoleURL string
}

var _ backend.PolicyPackReference = (*cloudBackendPolicyPackReference)(nil)

func (pr *cloudBackendPolicyPackReference) String() string {
	return fmt.Sprintf("%s/%s", pr.orgName, pr.name)
}

func (pr *cloudBackendPolicyPackReference) OrgName() string {
	return pr.orgName
}

func (pr *cloudBackendPolicyPackReference) Name() tokens.QName {
	return pr.name
}

func (pr *cloudBackendPolicyPackReference) CloudConsoleURL() string {
	return fmt.Sprintf("%s/%s/policypacks/%s", pr.cloudConsoleURL, pr.orgName, pr.Name())
}

// cloudPolicyPack is a the Pulumi service implementation of the PolicyPack interface.
type cloudPolicyPack struct {
	// ref uniquely identifies the PolicyPack in the Pulumi service.
	ref *cloudBackendPolicyPackReference
	// b is a pointer to the backend that this PolicyPack belongs to.
	b *cloudBackend
	// cl is the client used to interact with the backend.
	cl *client.Client
}

var _ backend.PolicyPack = (*cloudPolicyPack)(nil)

func (pack *cloudPolicyPack) Ref() backend.PolicyPackReference {
	return pack.ref
}

func (pack *cloudPolicyPack) Backend() backend.Backend {
	return pack.b
}

func (pack *cloudPolicyPack) Publish(
	ctx context.Context, op backend.PublishOperation) result.Result {

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
	pack.ref.name = tokens.QName(analyzerInfo.Name)
	pack.ref.versionTag = analyzerInfo.Version

	fmt.Println("Compressing policy pack")

	if runtime := op.PolicyPack.Runtime.Name(); !strings.EqualFold(runtime, "nodejs") {
		return result.Errorf(
			"failed to publish policies because Pulumi.yaml requests unsupported runtime %s",
			runtime)
	}

	// TODO[pulumi/pulumi#1307]: move to the language plugins so we don't have to hard code here.
	packTarball, err := npm.Pack(op.PlugCtx.Pwd, os.Stderr)
	if err != nil {
		return result.FromError(
			errors.Wrapf(err, "could not publish policies because of error running npm pack"))
	}

	//
	// Publish.
	//

	fmt.Println("Uploading Policy Pack to Pulumi service")

	publishedVersion, err := pack.cl.PublishPolicyPack(ctx, pack.ref.orgName, analyzerInfo, bytes.NewReader(packTarball))
	if err != nil {
		return result.FromError(err)
	}

	fmt.Printf("\nPermalink: %s/%s\n", pack.ref.CloudConsoleURL(), publishedVersion)

	return nil
}

func (pack *cloudPolicyPack) Enable(ctx context.Context, policyGroup string, op backend.PolicyPackOperation) error {
	if op.VersionTag == nil {
		return pack.cl.ApplyPolicyPack(ctx, pack.ref.orgName, policyGroup, string(pack.ref.name), "" /* versionTag */)
	}
	return pack.cl.ApplyPolicyPack(ctx, pack.ref.orgName, policyGroup, string(pack.ref.name), *op.VersionTag)
}

func (pack *cloudPolicyPack) Disable(ctx context.Context, policyGroup string, op backend.PolicyPackOperation) error {
	if op.VersionTag == nil {
		return pack.cl.DisablePolicyPack(ctx, pack.ref.orgName, policyGroup, string(pack.ref.name), "" /* versionTag */)
	}
	return pack.cl.DisablePolicyPack(ctx, pack.ref.orgName, policyGroup, string(pack.ref.name), *op.VersionTag)
}

func (pack *cloudPolicyPack) Remove(ctx context.Context, op backend.PolicyPackOperation) error {
	if op.VersionTag == nil {
		return pack.cl.RemovePolicyPack(ctx, pack.ref.orgName, string(pack.ref.name))
	}
	return pack.cl.RemovePolicyPackByVersion(ctx, pack.ref.orgName, string(pack.ref.name), *op.VersionTag)
}

const npmPackageDir = "package"

func installRequiredPolicy(finalDir string, tarball []byte) error {
	// If part of the directory tree is missing, ioutil.TempDir will return an error, so make sure
	// the path we're going to create the temporary folder in actually exists.
	if err := os.MkdirAll(filepath.Dir(finalDir), 0700); err != nil {
		return errors.Wrap(err, "creating plugin root")
	}

	tempDir, err := ioutil.TempDir(filepath.Dir(finalDir), fmt.Sprintf("%s.tmp", filepath.Base(finalDir)))
	if err != nil {
		return errors.Wrapf(err, "creating plugin directory %s", tempDir)
	}

	// npm unpacks into a directory called `package`.
	tempNPMPkgDir := path.Join(tempDir, npmPackageDir)
	if err := os.MkdirAll(tempNPMPkgDir, 0700); err != nil {
		return errors.Wrap(err, "creating plugin root")
	}

	// If we early out of this function, try to remove the temp folder we created.
	defer func() {
		contract.IgnoreError(os.RemoveAll(tempDir))
	}()

	// Uncompress the policy pack.
	err = archive.Untgz(tarball, tempDir)
	if err != nil {
		return err
	}

	fmt.Printf("Unpacking policy zip %q %q\n", tempDir, finalDir)

	// If two calls to `plugin install` for the same plugin are racing, the second one will be
	// unable to rename the directory. That's OK, just ignore the error. The temp directory created
	// as part of the install will be cleaned up when we exit by the defer above.
	if err := os.Rename(tempNPMPkgDir, finalDir); err != nil && !os.IsExist(err) {
		return errors.Wrap(err, "moving plugin")
	}

	proj, err := workspace.LoadPolicyPack(path.Join(finalDir, "PulumiPolicy.yaml"))
	if err != nil {
		return errors.Wrapf(err, "failed to load policy project at %s", finalDir)
	}

	// TODO[pulumi/pulumi#1307]: move to the language plugins so we don't have to hard code here.
	if !strings.EqualFold(proj.Runtime.Name(), "nodejs") {
		return fmt.Errorf("unsupported policy runtime %s", proj.Runtime.Name())
	}

	fmt.Println("Installing dependencies...")
	fmt.Println()

	// TODO[pulumi/pulumi#1307]: move to the language plugins so we don't have to hard code here.
	if bin, err := npm.Install(finalDir, nil, os.Stderr); err != nil {
		return errors.Wrapf(
			err,
			"failed to install dependencies of policy pack; you may need to re-run `%s install` "+
				"in %q before this policy pack works", bin, finalDir)
	}

	fmt.Println("Finished installing dependencies")
	fmt.Println()

	return nil
}
