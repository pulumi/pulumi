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
