// Copyright 2023, Pulumi Corporation.

package cli

import "os"

type environ interface {
	Get(key string) string
	Vars() []string
}

type defaultEnviron int

func newEnviron() environ {
	return defaultEnviron(0)
}

func (defaultEnviron) Get(key string) string {
	return os.Getenv(key)
}

func (defaultEnviron) Vars() []string {
	return os.Environ()
}
