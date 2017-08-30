// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"fmt"
)

func (eng *Engine) GetCurrentEnv() error {
	if name := eng.getCurrentEnv(); name != "" {
		fmt.Println(name)
	}
	return nil
}
