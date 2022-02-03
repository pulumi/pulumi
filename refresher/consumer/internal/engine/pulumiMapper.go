package engine

import (
	"context"
	"github.com/infralight/pulumi/refresher/common"
	"github.com/rs/zerolog"
)

func pulumiMapper(
	ctx context.Context,
	logger *zerolog.Logger,
	consumer *common.Consumer,
	accountId, integrationId, stackName, projectName, organizationName string) error{

	return nil

}