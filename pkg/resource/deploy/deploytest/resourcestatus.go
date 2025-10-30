package deploytest

import deploytest "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy/deploytest"

type ResourceStatus = deploytest.ResourceStatus

type ViewStep = deploytest.ViewStep

type ViewStepState = deploytest.ViewStepState

func NewResourceStatus(address string) (*ResourceStatus, error) {
	return deploytest.NewResourceStatus(address)
}

