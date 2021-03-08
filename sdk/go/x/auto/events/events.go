package events

import "github.com/pulumi/pulumi/sdk/v2/go/common/apitype"

type EngineEvent struct {
	apitype.EngineEvent
	Error error
}
