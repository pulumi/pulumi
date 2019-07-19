package httpstate

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/result"
	"github.com/pulumi/pulumi/pkg/workspace"
)

type cloudRequiredPolicy struct {
	apitype.RequiredPolicy
	client *client.Client
}

var _ engine.RequiredPolicy = (*cloudRequiredPolicy)(nil)

func newCloudRequiredPolicy(client *client.Client, policy apitype.RequiredPolicy) *cloudRequiredPolicy {
	return &cloudRequiredPolicy{client: client, RequiredPolicy: policy}
}

func (rp *cloudRequiredPolicy) Name() string    { return rp.RequiredPolicy.Name }
func (rp *cloudRequiredPolicy) Version() string { return strconv.Itoa(rp.RequiredPolicy.Version) }

func (rp *cloudRequiredPolicy) Install(ctx context.Context) (string, error) {
	policy := rp.RequiredPolicy

	policyPackPath, installed, err := workspace.GetPolicyPath(
		policy.Name, strconv.Itoa(policy.Version))
	if err != nil {
		// Failed to get a sensible PolicyPack path.
		return "", err
	} else if installed {
		// We've already downloaded and installed the PolicyPack. Return.
		return policyPackPath, nil
	}

	// PolicyPack has not been downloaded and installed. Do this now.
	policyPackZip, err := rp.client.DownloadPolicyPack(ctx, policy.PackLocation)
	if err != nil {
		return "", err
	}

	return policyPackPath, installRequiredPolicy(policyPackPath, policyPackZip)
}

func newCloudBackendPolicyPackReference(
	orgName string, name tokens.QName) *cloudBackendPolicyPackReference {

	return &cloudBackendPolicyPackReference{orgName: orgName, name: name}
}

// cloudBackendPolicyPackReference is a reference to a PolicyPack implemented by the Pulumi service.
type cloudBackendPolicyPackReference struct {
	// name of the PolicyPack.
	name tokens.QName
	// orgName that administrates the PolicyPack.
	orgName string
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

	analyzer, err := op.PlugCtx.Host.PolicyAnalyzer(pack.Ref().Name(), op.PlugCtx.Pwd)
	if err != nil {
		return result.FromError(err)
	}

	analyzerInfo, err := analyzer.GetAnalyzerInfo()
	if err != nil {
		return result.FromError(err)
	}

	analyzerInfo.Name = string(pack.ref.name)

	fmt.Println("Compressing policy pack")

	dirArchive, err := archive.Process(op.Root, false)
	if err != nil {
		return result.FromError(err)
	}

	//
	// Publish.
	//

	fmt.Println("Uploading policy pack to Pulumi service")

	err = pack.cl.PublishPolicyPack(ctx, pack.ref.orgName, analyzerInfo, dirArchive)
	if err != nil {
		return result.FromError(err)
	}

	return nil
}

func (pack *cloudPolicyPack) Apply(ctx context.Context, op backend.ApplyOperation) error {
	return pack.cl.ApplyPolicyPack(ctx, pack.ref.orgName, string(pack.ref.name), op.Version)
}

func installRequiredPolicy(finalDir string, zip []byte) error {
	// If part of the directory tree is missing, ioutil.TempDir will return an error, so make sure
	// the path we're going to create the temporary folder in actually exists.
	if err := os.MkdirAll(filepath.Dir(finalDir), 0700); err != nil {
		return errors.Wrap(err, "creating plugin root")
	}

	tempDir, err := ioutil.TempDir(filepath.Dir(finalDir), fmt.Sprintf("%s.tmp", filepath.Base(finalDir)))
	if err != nil {
		return errors.Wrapf(err, "creating plugin directory %s", tempDir)
	}

	// If we early out of this function, try to remove the temp folder we created.
	defer func() {
		contract.IgnoreError(os.RemoveAll(tempDir))
	}()

	// Unzip the file. NOTE: It is important that the `Close` calls in this function complete before
	// we try to rename the temp directory. Open file handles here cause issues on Windows.
	err = archive.Unzip(zip, tempDir)
	if err != nil {
		return err
	}

	fmt.Printf("Unpacking policy zip %q %q\n", tempDir, finalDir)

	// If two calls to `plugin install` for the same plugin are racing, the second one will be
	// unable to rename the directory. That's OK, just ignore the error. The temp directory created
	// as part of the install will be cleaned up when we exit by the defer above.
	if err := os.Rename(tempDir, finalDir); err != nil && !os.IsExist(err) {
		return errors.Wrap(err, "moving plugin")
	}

	return nil
}
