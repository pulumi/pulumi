package engine

import (
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

func (eng *Engine) RemoveEnv(envName string, force bool) error {
	contract.Assert(envName != "")

	info, err := eng.initEnvCmd(envName, "")

	if err != nil {
		return err
	}

	// Don't remove environments that still have resources.
	if !force && info.Snapshot != nil && len(info.Snapshot.Resources) > 0 {
		return errors.Errorf(
			"'%v' still has resources; removal rejected; pass --force to override", info.Target.Name)
	}

	return eng.removeTarget(info.Target)
}
