package engine

import (
	"fmt"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/core"
	"github.com/pulumi/pulumi-fabric/pkg/eval"
	"github.com/pulumi/pulumi-fabric/pkg/resource/deploy"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

func (eng *Engine) PackEval(configEnv string, pkg string, packArgs core.Args) error {
	// First, load and compile the package.
	result := eng.compile(pkg)
	if result == nil {
		return nil
	}

	// Now fire up an interpreter so we can run the program.
	e := eval.New(result.B.Ctx(), nil)

	// If configuration was requested, load it up and populate the object state.
	if configEnv != "" {
		envInfo, err := eng.initEnvCmdName(tokens.QName(configEnv), pkg)
		if err != nil {
			return err
		}
		if err := deploy.InitEvalConfig(result.B.Ctx(), e, envInfo.Target.Config); err != nil {
			return err
		}
	}

	// Finally, execute the entire program, and serialize the return value (if any).
	if obj, _ := e.EvaluatePackage(result.Pkg, packArgs); obj != nil {
		fmt.Fprint(eng.Stdout, obj)
	}
	return nil
}

func pkgargFromArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}

	return args[0]
}
