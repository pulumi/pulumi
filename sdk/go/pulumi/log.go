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
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
	"golang.org/x/net/context"
)

// Log is a group of logging functions that can be called from a Go application that will be logged
// to the Pulumi log stream.  These events will be printed in the terminal while the Pulumi app
// runs, and will be available from the Web console afterwards.
type Log interface {
	Debug(msg string, resource Resource, streamID int32, ephemeral bool) error
	Info(msg string, resource Resource, streamID int32, ephemeral bool) error
	Warn(msg string, resource Resource, streamID int32, ephemeral bool) error
	Error(msg string, resource Resource, streamID int32, ephemeral bool) error
}

type logState struct {
	engine pulumirpc.EngineClient
	ctx    context.Context
}

// Debug logs a debug-level message that is generally hidden from end-users.
func (log *logState) Debug(msg string, resource Resource, streamID int32, ephemeral bool) error {
	return _log(log.ctx, log.engine, pulumirpc.LogSeverity_DEBUG, msg, resource, streamID, ephemeral)
}

// Logs an informational message that is generally printed to stdout during resource
func (log *logState) Info(msg string, resource Resource, streamID int32, ephemeral bool) error {
	return _log(log.ctx, log.engine, pulumirpc.LogSeverity_INFO, msg, resource, streamID, ephemeral)
}

// Logs a warning to indicate that something went wrong, but not catastrophically so.
func (log *logState) Warn(msg string, resource Resource, streamID int32, ephemeral bool) error {
	return _log(log.ctx, log.engine, pulumirpc.LogSeverity_WARNING, msg, resource, streamID, ephemeral)
}

// Logs a fatal error to indicate that the tool should stop processing resource
func (log *logState) Error(msg string, resource Resource, streamID int32, ephemeral bool) error {
	return _log(log.ctx, log.engine, pulumirpc.LogSeverity_ERROR, msg, resource, streamID, ephemeral)
}

func _log(ctx context.Context, engine pulumirpc.EngineClient, severity pulumirpc.LogSeverity,
	message string, resource Resource, streamID int32, ephemeral bool) error {

	resolvedUrn, _, _, err := resource.URN().awaitURN(ctx)
	if err != nil {
		return err
	}

	logRequest := &pulumirpc.LogRequest{
		Severity:  severity,
		Message:   message,
		Urn:       string(resolvedUrn),
		StreamId:  streamID,
		Ephemeral: ephemeral,
	}
	_, err = engine.Log(ctx, logRequest)
	return err
}
