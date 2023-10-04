// Copyright 2023, Pulumi Corporation.

package cli

import "os/exec"

type cmdExec interface {
	LookPath(command string) (string, error)
	Run(cmd *exec.Cmd) error
}

type defaultCmdExec int

func newCmdExec() cmdExec {
	return defaultCmdExec(0)
}

func (defaultCmdExec) LookPath(command string) (string, error) {
	return exec.LookPath(command)
}

func (defaultCmdExec) Run(cmd *exec.Cmd) error {
	return cmd.Run()
}
