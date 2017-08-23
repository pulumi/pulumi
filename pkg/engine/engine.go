package engine

import (
	"io"
	"os"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/core"
	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

var (
	E Engine
)

type Engine struct {
	Stdout io.Writer
	Stderr io.Writer
	snk    diag.Sink
}

func init() {
	E.Stdout = os.Stdout
	E.Stderr = os.Stderr
}

func (e *Engine) Diag() diag.Sink {
	if e.snk == nil {
		e.snk = core.DefaultSink("")
	}

	return e.snk
}

func (e *Engine) InitDiag(opts diag.FormatOptions) {
	contract.Assertf(e.snk == nil, "Cannot initialize diagnostics sink more than once")
	e.snk = diag.DefaultSink(opts)
}
