package debug

import "fmt"

type LoggingOptions struct {
	// LogLevel - choose verbosity level of at least 1 (least verbose).
	// If not specified, reverts to default log level.
	// Note - These logs may include sensitive information that is provided from your
	// execution environment to your cloud provider (and which Pulumi may not even
	// itself be aware of).
	LogLevel *uint
	// LogToStdErr specifies that all logs should be sent directly to stderr - making it
	// more accessible and avoiding OS level buffering.
	LogToStdErr bool
	// FlowToPlugins reflects the logging settings to plugins as well.
	FlowToPlugins bool
}

func AddArgs(debugLogOpts *LoggingOptions, sharedArgs []string) []string {
	if debugLogOpts.LogToStdErr {
		sharedArgs = append(sharedArgs, "--logtostderr")
	}
	if debugLogOpts.LogLevel != nil {
		if *debugLogOpts.LogLevel == 0 {
			*debugLogOpts.LogLevel = 1
		}
		sharedArgs = append(sharedArgs, fmt.Sprintf("-v=%d", *debugLogOpts.LogLevel))
	}
	if debugLogOpts.FlowToPlugins {
		sharedArgs = append(sharedArgs, "--logflow")
	}
	return sharedArgs
}
