package cloudplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/go-resty/resty/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type PulumiAPI interface {
	CreateStack(ctx context.Context, org, project, stack string) error
	GetStack(ctx context.Context, org, project, stack string) error
	GetDeploymentSettings(ctx context.Context, org, project, stack string) error
	PatchDeploymentSettings(ctx context.Context, org, project, stack string) error
	CreateDeployment(ctx context.Context, org, project, stack string) error
	GetOrg(ctx context.Context, org string) (OrgMetadata, error)
}

type OrgMetadata struct {
	StackCount  int `"json:"stackCount"`
	MemberCount int `"json:"memberCount"`
}

type PulumiAPIClient struct {
	apiURL            string
	pulumiAccessToken string
	client            *resty.Client
}

type PulumiAPIArgs struct {
	// PulumiAPIURL is the URL of the Pulumi API, defaults to $PULUMI_BACKEND_URL, ~/.pulumi/credentials.json, then https://api.pulumi.com
	PulumiAPIURL *string
	// PulumiAccessToken is the access token for the Pulumi API, defaults to $PULUMI_ACCESS_TOKEN or the value in ~/.pulumi/credentials.json
	PulumiAccessToken *string
}

func NewPulumiAPI(args *PulumiAPIArgs) (PulumiAPI, error) {
	if args == nil {
		args = &PulumiAPIArgs{}
	}

	cloudURL := ""
	if args.PulumiAPIURL != nil {
		cloudURL = *args.PulumiAPIURL
	}
	url, err := workspace.GetCurrentCloudURL(nil)
	if err == nil && url != "" {
		cloudURL = url
	}
	if cloudURL == "" {
		cloudURL = "https://api.pulumi.com"
	}

	accessToken := ""
	if args.PulumiAccessToken != nil {
		accessToken = *args.PulumiAccessToken
	}

	if accessToken == "" {
		accessToken = os.Getenv("PULUMI_ACCESS_TOKEN")
	}

	if accessToken == "" {
		pulumiAccessToken, err := workspace.GetAccount(cloudURL)
		if err == nil {
			accessToken = pulumiAccessToken.AccessToken
		}
	}

	if accessToken == "" {
		return nil, fmt.Errorf("Pulumi API access token not found")
	}

	return &PulumiAPIClient{
		apiURL:            cloudURL + "/api",
		pulumiAccessToken: accessToken,
		client:            resty.New(),
	}, nil
}

func (p *PulumiAPIClient) CreateStack(ctx context.Context, org, project, stack string) error {

	return nil
}

func (p *PulumiAPIClient) GetStack(ctx context.Context, org, project, stack string) error {

	return nil
}

func (p *PulumiAPIClient) GetDeploymentSettings(ctx context.Context, org, project, stack string) error {

	return nil
}

func (p *PulumiAPIClient) PatchDeploymentSettings(ctx context.Context, org, project, stack string) error {

	return nil
}

func (p *PulumiAPIClient) CreateDeployment(ctx context.Context, org, project, stack string) error {

	return nil
}

func (p *PulumiAPIClient) GetOrg(ctx context.Context, org string) (OrgMetadata, error) {
	resp, err := p.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "token "+p.pulumiAccessToken).
		SetHeader("Accept", "application/json").
		SetHeader("Accept", "application/vnd.pulumi+8").
		SetDoNotParseResponse(true).
		Get(p.apiURL + path.Join("/orgs", org, "metadata"))

	var result OrgMetadata

	if err != nil {
		return result, err
	}

	switch resp.StatusCode() {
	case http.StatusOK:
		// OK
	default:
		return result, fmt.Errorf("%v: %s", resp.StatusCode(), string(resp.Body()))
	}

	defer resp.RawBody().Close()
	if err = json.NewDecoder(resp.RawBody()).Decode(&result); err != nil {
		return result, err
	}

	return result, nil
}
