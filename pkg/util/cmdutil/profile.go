package cmdutil

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/util/contract"
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
