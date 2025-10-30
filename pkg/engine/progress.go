package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

// Creates a new ReadCloser that reports ProgressEvents as bytes are read and
// when it is closed. A Done ProgressEvent will only be reported once, on the
// first call to Close(). Subsequent calls to Close() will be forwarded to the
// underlying ReadCloser, but will not yield duplicate ProgressEvents.
func NewProgressReportingCloser(events eventEmitter, typ ProgressType, id string, message string, size int64, reportingInterval time.Duration, closer io.ReadCloser) io.ReadCloser {
	return engine.NewProgressReportingCloser(events, typ, id, message, size, reportingInterval, closer)
}

