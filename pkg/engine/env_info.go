package engine

import (
	"fmt"

	"github.com/pkg/errors"
)

func (eng *Engine) GetCurrentEnvName() string {
	return eng.getCurrentEnv().String()
}

func (eng *Engine) EnvInfo(showIDs bool, showURNs bool) error {
	curr := eng.getCurrentEnv()
	if curr == "" {
		return errors.New("no current environment; either `lumi env init` or `lumi env select` one")
	}
	fmt.Fprintf(eng.Stdout, "Current environment is %v\n", curr)
	fmt.Fprintf(eng.Stdout, "    (use `lumi env select` to change environments; `lumi env ls` lists known ones)\n")
	target, snapshot, checkpoint, err := eng.Environment.GetEnvironment(curr)
	if err != nil {
		return err
	}
	if checkpoint.Latest != nil {
		fmt.Fprintf(eng.Stdout, "Last deployment at %v\n", checkpoint.Latest.Time)
		if checkpoint.Latest.Info != nil {
			fmt.Fprintf(eng.Stdout, "Additional deployment info: %v\n", checkpoint.Latest.Info)
		}
	}
	if target.Config != nil && len(target.Config) > 0 {
		fmt.Fprintf(eng.Stdout, "%v configuration variables set (see `lumi config` for details)\n", len(target.Config))
	}
	if snapshot == nil || len(snapshot.Resources) == 0 {
		fmt.Fprintf(eng.Stdout, "No resources currently in this environment\n")
	} else {
		fmt.Fprintf(eng.Stdout, "%v resources currently in this environment:\n", len(snapshot.Resources))
		fmt.Fprintf(eng.Stdout, "\n")
		fmt.Fprintf(eng.Stdout, "%-48s %s\n", "TYPE", "NAME")
		for _, res := range snapshot.Resources {
			fmt.Fprintf(eng.Stdout, "%-48s %s\n", res.Type(), res.URN().Name())

			// If the ID and/or URN is requested, show it on the following line.  It would be nice to do this
			// on a single line, but they can get quite lengthy and so this formatting makes more sense.
			if showIDs {
				fmt.Fprintf(eng.Stdout, "\tID: %s\n", res.ID)
			}
			if showURNs {
				fmt.Fprintf(eng.Stdout, "\tURN: %s\n", res.URN())
			}
		}
	}
	return nil
}
