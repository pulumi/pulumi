package engine

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/core"
	"github.com/pulumi/pulumi-fabric/pkg/eval"
	"github.com/pulumi/pulumi-fabric/pkg/resource/deploy"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

func PackEval(configEnv string, args []string) error {
	// First, load and compile the package.
	result := compile(pkgargFromArgs(args))
	if result == nil {
		return nil
	}

	// Now fire up an interpreter so we can run the program.
	e := eval.New(result.B.Ctx(), nil)

	// If configuration was requested, load it up and populate the object state.
	if configEnv != "" {
		envInfo, err := initEnvCmdName(tokens.QName(configEnv), pkgargFromArgs(args))
		if err != nil {
			return err
		}
		if err := deploy.InitEvalConfig(result.B.Ctx(), e, envInfo.Target.Config); err != nil {
			return err
		}
	}

	// Finally, execute the entire program, and serialize the return value (if any).
	packArgs := dashdashArgsToMap(args)
	if obj, _ := e.EvaluatePackage(result.Pkg, packArgs); obj != nil {
		fmt.Print(obj)
	}
	return nil
}

func pkgargFromArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}

	return args[0]
}

// dashdashArgsToMap is a simple args parser that places incoming key/value pairs into a map.  These are then used
// during package compilation as inputs to the main entrypoint function.
// IDEA: this is fairly rudimentary; we eventually want to support arrays, maps, and complex types.
func dashdashArgsToMap(args []string) core.Args {
	mapped := make(core.Args)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Eat - or -- at the start.
		if arg[0] == '-' {
			arg = arg[1:]
			if arg[0] == '-' {
				arg = arg[1:]
			}
		}

		// Now find a k=v, and split the k/v part.
		if eq := strings.IndexByte(arg, '='); eq != -1 {
			// For --k=v, simply store v underneath k's entry.
			mapped[tokens.Name(arg[:eq])] = arg[eq+1:]
		} else {
			if i+1 < len(args) && args[i+1][0] != '-' {
				// If the next arg doesn't start with '-' (i.e., another flag) use its value.
				mapped[tokens.Name(arg)] = args[i+1]
				i++
			} else if arg[0:3] == "no-" {
				// For --no-k style args, strip off the no- prefix and store false underneath k.
				mapped[tokens.Name(arg[3:])] = false
			} else {
				// For all other --k args, assume this is a boolean flag, and set the value of k to true.
				mapped[tokens.Name(arg)] = true
			}
		}
	}

	return mapped
}
