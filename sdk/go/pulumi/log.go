// Copyright 2016-2020, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The log module logs messages in a way that tightly integrates with the resource engine's interface.

package pulumi

import (
	"strings"
	"sync"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"golang.org/x/net/context"
)

// Log is a group of logging functions that can be called from a Go application that will be logged
// to the Pulumi log stream.  These events will be printed in the terminal while the Pulumi app
// runs, and will be available from the Web console afterwards.
type Log interface {
	Debug(msg string, args *LogArgs) error
	Info(msg string, args *LogArgs) error
	Warn(msg string, args *LogArgs) error
	Error(msg string, args *LogArgs) error
}

type logState struct {
	engine pulumirpc.EngineClient
	ctx    context.Context
	join   *sync.WaitGroup
}

// LogArgs may be used to specify arguments to be used for logging.
type LogArgs struct {
	// Optional resource this log is associated with.
	Resource Resource

	// Optional stream id that a stream of log messages can be associated with. This allows
	// clients to not have to buffer a large set of log messages that they all want to be
	// conceptually connected.  Instead the messages can be sent as chunks (with the same stream id)
	// and the end display can show the messages as they arrive, while still stitching them together
	// into one total log message.
	StreamID int32

	// Optional value indicating whether this is a status message.
	Ephemeral bool
}

// Debug logs a debug-level message that is generally hidden from end-users.
func (log *logState) Debug(msg string, args *LogArgs) error {
	return log._log(pulumirpc.LogSeverity_DEBUG, msg, args)
}

// Logs an informational message that is generally printed to stdout during resource
func (log *logState) Info(msg string, args *LogArgs) error {
	return log._log(pulumirpc.LogSeverity_INFO, msg, args)
}

// Logs a warning to indicate that something went wrong, but not catastrophically so.
func (log *logState) Warn(msg string, args *LogArgs) error {
	return log._log(pulumirpc.LogSeverity_WARNING, msg, args)
}

// Logs a fatal condition. Consider returning a non-nil error object
// after calling Error to stop the Pulumi program.
func (log *logState) Error(msg string, args *LogArgs) error {
	return log._log(pulumirpc.LogSeverity_ERROR, msg, args)
}

func (log *logState) _log(severity pulumirpc.LogSeverity, message string, args *LogArgs) error {
	if log.join != nil {
		log.join.Add(1)
		defer log.join.Done()
	}

	if args == nil {
		args = &LogArgs{}
	}

	var urn string
	if args.Resource != nil {
		resolvedUrn, _, _, err := args.Resource.URN().awaitURN(log.ctx)
		if err != nil {
			return err
		}
		urn = string(resolvedUrn)
	}

	logRequest := &pulumirpc.LogRequest{
		Severity:  severity,
		Message:   strings.ToValidUTF8(message, "�"),
		Urn:       urn,
		StreamId:  args.StreamID,
		Ephemeral: args.Ephemeral,
	}
	_, err := log.engine.Log(log.ctx, logRequest)
	return err
}
