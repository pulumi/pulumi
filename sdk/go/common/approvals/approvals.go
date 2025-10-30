package approvals

import (
	"errors"

	"github.com/gofrs/uuid"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type Approvals interface {
	IsSupported() bool

	CreateApproval(apitype.CreateChangeRequestRequest) (apitype.CreateChangeRequestResponse, error)
}

func NewNilApprovals() Approvals {
	return &nilApprovals{}
}

type nilApprovals struct{}

func (a *nilApprovals) IsSupported() bool {
	return false
}

func (a *nilApprovals) CreateApproval(apitype.CreateChangeRequestRequest) (apitype.CreateChangeRequestResponse, error) {
	return apitype.CreateChangeRequestResponse{}, errors.New("approvals not supported")
}

func NewApprovals(client *client.Client) Approvals {
	return &cloudApprovals{
		client: client,
	}
}

type cloudApprovals struct {
	client *client.Client
}

func (a *cloudApprovals) IsSupported() bool {
	return true // TODO: check server capabilities
}

func (a *cloudApprovals) CreateApproval(req apitype.CreateChangeRequestRequest) (apitype.CreateChangeRequestResponse, error) {
	changeRequestId, err := uuid.NewV4()
	if err != nil {
		return apitype.CreateChangeRequestResponse{}, err
	}
	return apitype.CreateChangeRequestResponse{
		ChangeRequestID: changeRequestId.String(),
		Etag:            "etag",
	}, nil
}
