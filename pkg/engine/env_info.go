package engine

import (
	"fmt"

	"github.com/pkg/errors"
)

func EnvInfo(showIDs bool, showURNs bool) error {
	curr := getCurrentEnv()
	if curr == "" {
		return errors.New("no current environment; either `lumi env init` or `lumi env select` one")
	}
	fmt.Printf("Current environment is %v\n", curr)
	fmt.Printf("    (use `lumi env select` to change environments; `lumi env ls` lists known ones)\n")
	target, snapshot, checkpoint := readEnv(curr)
	if checkpoint == nil {
		return errors.Errorf("could not read environment information")
	}
	if checkpoint.Latest != nil {
		fmt.Printf("Last deployment at %v\n", checkpoint.Latest.Time)
		if checkpoint.Latest.Info != nil {
			fmt.Printf("Additional deployment info: %v\n", checkpoint.Latest.Info)
		}
	}
	if target.Config != nil && len(target.Config) > 0 {
		fmt.Printf("%v configuration variables set (see `lumi config` for details)\n", len(target.Config))
	}
	if snapshot == nil || len(snapshot.Resources) == 0 {
		fmt.Printf("No resources currently in this environment\n")
	} else {
		fmt.Printf("%v resources currently in this environment:\n", len(snapshot.Resources))
		fmt.Printf("\n")
		fmt.Printf("%-48s %s\n", "TYPE", "NAME")
		for _, res := range snapshot.Resources {
			fmt.Printf("%-48s %s\n", res.Type(), res.URN().Name())

			// If the ID and/or URN is requested, show it on the following line.  It would be nice to do this
			// on a single line, but they can get quite lengthy and so this formatting makes more sense.
			if showIDs {
				fmt.Printf("\tID: %s\n", res.ID)
			}
			if showURNs {
				fmt.Printf("\tURN: %s\n", res.URN())
			}
		}
	}
	return nil
}
