package events

import "github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

type EngineEvent struct {
	apitype.EngineEvent
	Error error
}
