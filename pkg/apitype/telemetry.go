// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package apitype

const (
	// TelemetryErrorSeverityWarning represents a warning reported to the service.
	TelemetryErrorSeverityWarning = "warning"

	// TelemetryErrorSeverityError represents an error reported to the service.
	TelemetryErrorSeverityError = "error"

	// TelemetryErrorSeverityPanic represents a panic reported to the service.
	TelemetryErrorSeverityPanic = "panic"
)

// TelemetryEnvironmentInfo is some basic information about the environment that
// caused a warning, error, or panic.
type TelemetryEnvironmentInfo struct {
	PulumiVersion string `json:"pulumiVersion"` // The version of this CLI
	GoVersion     string `json:"goVersion"`     // The version of Go that compiled the CLI
	OS            string `json:"os"`            // The current operating system
	Arch          string `json:"arch"`          // The current arch
}

// TelemetryError represents an error, warning, or panic seen by the CLI.
type TelemetryError struct {
	Environment TelemetryEnvironmentInfo `json:"environment"` // Machine environment information
	Severity    string                   `json:"severity"`    // The severity of the error
	Message     string                   `json:"message"`     // The message of the error
	DryRun      bool                     `json:"dryRun"`      // If the error occurred during plan execution,
	// whether we were doing a preview or an update
}
