package engine

import (
	"fmt"
)

func GetCurrentEnv() error {
	if name := getCurrentEnv(); name != "" {
		fmt.Println(name)
	}
	return nil
}
