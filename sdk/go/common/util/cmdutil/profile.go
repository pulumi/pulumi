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
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func InitProfiling(prefix string, memProfileRate int) error {
	cpu, err := os.Create(fmt.Sprintf("%s.%v.cpu", prefix, os.Getpid()))
	if err != nil {
		return fmt.Errorf("could not start CPU profile: %w", err)
	}
	if err = pprof.StartCPUProfile(cpu); err != nil {
		return fmt.Errorf("could not start CPU profile: %w", err)
	}

	exec, err := os.Create(fmt.Sprintf("%s.%v.trace", prefix, os.Getpid()))
	if err != nil {
		return fmt.Errorf("could not start execution trace: %w", err)
	}
	if err = trace.Start(exec); err != nil {
		return fmt.Errorf("could not start execution trace: %w", err)
	}

	if memProfileRate > 0 {
		runtime.MemProfileRate = memProfileRate
		go memoryProfileWriteLoop(prefix)
	}

	return nil
}

func CloseProfiling(prefix string) error {
	pprof.StopCPUProfile()
	trace.Stop()

	// get up-to-date statistics
	return writeMemoryProfile(prefix)
}

func writeMemoryProfile(prefix string) error {
	mem, err := os.Create(fmt.Sprintf("%s.%v.mem", prefix, os.Getpid()))
	if err != nil {
		return fmt.Errorf("could not create memory profile: %w", err)
	}
	defer contract.IgnoreClose(mem)

	runtime.GC()
	if err = pprof.Lookup("allocs").WriteTo(mem, 0); err != nil {
		return fmt.Errorf("could not write memory profile: %w", err)
	}
	return nil
}

func memoryProfileWriteLoop(prefix string) {
	// Every 5 seconds write a memory profile (in case we crash before we get a chance)
	for i := 0; ; i++ {
		time.Sleep(5 * time.Second)

		mem, err := os.Create(fmt.Sprintf("%s.%v.mem.%d", prefix, os.Getpid(), i))
		if err != nil {
			contract.IgnoreClose(mem)
			fmt.Fprintf(os.Stderr, "could not create memory profile: %s\n", err.Error())

			return
		}

		runtime.GC() // get up-to-date statistics
		if err = pprof.Lookup("allocs").WriteTo(mem, 0); err != nil {
			fmt.Fprintf(os.Stderr, "could not create memory profile: %s\n", err.Error())
		}

		contract.IgnoreClose(mem)
		os.Remove(fmt.Sprintf("%s.%v.mem.%d", prefix, os.Getpid(), i-1))
	}
}
