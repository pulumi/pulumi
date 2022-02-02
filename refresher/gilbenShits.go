package refresher

import (
	httpstateClient "github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	defaultAPIDomainPrefix = "api."
	defaultConsoleDomainPrefix = "app."
)


type cloudBackend struct {
	d              diag.Sink
	url            string
	client         *httpstateClient.Client
	currentProject *workspace.Project
}

type cloudBackendReference struct {
	name    tokens.QName
	project string
	owner   string
	b       *cloudBackend
}

type test struct {
	name    tokens.QName
	project string
	owner   string
}