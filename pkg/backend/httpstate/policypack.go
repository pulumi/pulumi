package httpstate

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/pkg/util/archive"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/result"
)

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

	analyzer, err := op.PlugCtx.Host.Analyzer(pack.Ref().Name())
	if err != nil {
		return result.FromError(err)
	}

	analyzerInfo, err := analyzer.GetAnalyzerInfo()
	if err != nil {
		return result.FromError(err)
	}

	dirArchive, err := archive.Process(op.Root, false)
	if err != nil {
		return result.FromError(err)
	}

	err = pack.cl.PublishPolicyPack(ctx, pack.ref.orgName, analyzerInfo, dirArchive)
	if err != nil {
		return result.FromError(err)
	}

	return nil
}

func (pack *cloudPolicyPack) Apply(ctx context.Context, op backend.ApplyOperation) error {
	return pack.cl.ApplyPolicyPack(ctx, pack.ref.orgName, string(pack.ref.name), op.Version)
}
