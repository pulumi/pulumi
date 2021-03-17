// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmdutil

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func InitProfiling(prefix string) error {
	cpu, err := os.Create(fmt.Sprintf("%s.%v.cpu", prefix, os.Getpid()))
	if err != nil {
		return errors.Wrap(err, "could not start CPU profile")
	}
	if err = pprof.StartCPUProfile(cpu); err != nil {
		return errors.Wrap(err, "could not start CPU profile")
	}

	exec, err := os.Create(fmt.Sprintf("%s.%v.trace", prefix, os.Getpid()))
	if err != nil {
		return errors.Wrap(err, "could not start execution trace")
	}
	if err = trace.Start(exec); err != nil {
		return errors.Wrap(err, "could not start execution trace")
	}

	return nil
}

func CloseProfiling(prefix string) error {
	pprof.StopCPUProfile()
	trace.Stop()

	mem, err := os.Create(fmt.Sprintf("%s.%v.mem", prefix, os.Getpid()))
	if err != nil {
		return errors.Wrap(err, "could not create memory profile")
	}
	defer contract.IgnoreClose(mem)

	runtime.GC() // get up-to-date statistics
	if err = pprof.WriteHeapProfile(mem); err != nil {
		return errors.Wrap(err, "could not write memory profile")
	}

	return nil
}
