// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build !windows

package cmdutil

import (
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// KillChildren calls os.Process.Kill() on every child process of `pid`'s, stoping after the first error (if any). It also only kills
// direct child process, not any children they may have.
func KillChildren(pid int) error {
	contract.Failf("KillChidren only implemented on windows")
	return nil
}
