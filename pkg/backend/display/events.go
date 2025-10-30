package display

import display "github.com/pulumi/pulumi/sdk/v3/pkg/backend/display"

// ConvertEngineEvent converts a raw engine.Event into an apitype.EngineEvent used in the Pulumi
// REST API. Returns an error if the engine event is unknown or not in an expected format.
// EngineEvent.{ Sequence, Timestamp } are expected to be set by the caller.
// 
// IMPORTANT: Any resource secret data stored in the engine event will be encrypted using the
// blinding encrypter, and unrecoverable. So this operation is inherently lossy.
func ConvertEngineEvent(e engine.Event, showSecrets bool) (apitype.EngineEvent, error) {
	return display.ConvertEngineEvent(e, showSecrets)
}

// ConvertJSONEvent converts an apitype.EngineEvent from the Pulumi REST API into a raw engine.Event
// Returns an error if the engine event is unknown or not in an expected format.
// 
// IMPORTANT: Any resource secret data stored in the engine event will be encrypted using the
// blinding encrypter, and unrecoverable. So this operation is inherently lossy.
func ConvertJSONEvent(apiEvent apitype.EngineEvent) (engine.Event, error) {
	return display.ConvertJSONEvent(apiEvent)
}

