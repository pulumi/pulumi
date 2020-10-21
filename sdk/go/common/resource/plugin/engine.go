package plugin

import (
	"context"
	"sync/atomic"

	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
)

// Engine is an auxiliary service offered to language and resource provider plugins. Its main purpose today is
// to serve as a common logging endpoint, but it also serves as a state storage mechanism for language hosts
// that can't store their own global state.
type Engine interface {
	Logger

	// GetRootResource gets the URN of the root resource, the resource that should be the root of all
	// otherwise-unparented resources.
	GetRootResource(ctx context.Context) (resource.URN, error)

	// SetRootResource sets the URN of the root resource.
	SetRootResource(ctx context.Context, urn resource.URN) error
}

type defaultEngine struct {
	// The Logger for this engine.
	logger Logger

	// hostServer contains little bits of state that can't be saved in the language host.
	rootUrn atomic.Value // a root resource URN that has been saved via SetRootResource
}

func NewDefaultEngine(logger Logger) Engine {
	engine := &defaultEngine{logger: logger}
	engine.rootUrn.Store(resource.URN(""))
	return engine
}

func (e *defaultEngine) Log(ctx context.Context, sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	e.logger.Log(ctx, sev, urn, msg, streamID)
}

func (e *defaultEngine) LogStatus(ctx context.Context,
	sev diag.Severity, urn resource.URN, msg string, streamID int32) {

	e.logger.LogStatus(ctx, sev, urn, msg, streamID)
}

// GetRootResource returns the current root resource's URN, which will serve as the parent of resources that are
// otherwise left unparented.
func (e *defaultEngine) GetRootResource(ctx context.Context) (resource.URN, error) {
	return e.rootUrn.Load().(resource.URN), nil
}

// SetRootResources sets the current root resource's URN. Generally only called on startup when the Stack resource is
// registered.
func (e *defaultEngine) SetRootResource(ctx context.Context, urn resource.URN) error {
	e.rootUrn.Store(urn)
	return nil
}
