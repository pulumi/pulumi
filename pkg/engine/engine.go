package engine

import (
	"io"

	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

type Engine struct {
	Stdout io.Writer
	Stderr io.Writer
	snk    diag.Sink
}

func (e *Engine) Diag() diag.Sink {
	if e.snk == nil {
		e.InitDiag(diag.FormatOptions{})
	}

	return e.snk
}

func (e *Engine) InitDiag(opts diag.FormatOptions) {
	contract.Assertf(e.snk == nil, "Cannot initialize diagnostics sink more than once")

	// Force using our stdout and stderr
	opts.Stdout = e.Stdout
	opts.Stderr = e.Stderr

	e.snk = diag.DefaultSink(opts)
}
