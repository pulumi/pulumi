package display

import display "github.com/pulumi/pulumi/sdk/v3/pkg/backend/display"

// MassageSecrets takes a property map and returns a new map by transforming each value with massagePropertyValue
// This allows us to serialize the resulting map using our existing serialization logic we use for deployments, to
// produce sane output for stackOutputs.  If we did not do this, SecretValues would be serialized as objects
// with the signature key and value.
func MassageSecrets(m resource.PropertyMap, showSecrets bool) resource.PropertyMap {
	return display.MassageSecrets(m, showSecrets)
}

// ShowJSONEvents renders incremental engine events to stdout.
func ShowJSONEvents(events <-chan engine.StampedEvent, done chan<- bool, opts Options) {
	display.ShowJSONEvents(events, done, opts)
}

// ShowPreviewDigest renders engine events from a preview into a well-formed JSON document. Note that this does not
// emit events incrementally so that it can guarantee anything emitted to stdout is well-formed. This means that,
// if used interactively, the experience will lead to potentially very long pauses. If run in CI, it is up to the
// end user to ensure that output is periodically printed to prevent tools from thinking preview has hung.
func ShowPreviewDigest(events <-chan engine.Event, done chan<- bool, opts Options) {
	display.ShowPreviewDigest(events, done, opts)
}

