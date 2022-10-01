//go:build !darwin || !arm64
// +build !darwin !arm64

// Copyright 2016-2019, Pulumi Corporation.
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

package backend

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/rjeczalik/notify"
	logs "go.opentelemetry.io/proto/otlp/logs/v1"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Watch watches the project's working directory for changes and automatically updates the active
// stack.
func Watch(ctx context.Context, b Backend, stack Stack, op UpdateOperation,
	apply Applier, paths []string) result.Result {

	opts := ApplierOptions{
		DryRun:   false,
		ShowLink: false,
	}

	startTime := time.Now()

	go func() {
		shown := map[operations.LogEntry]bool{}
		for {
			logs, err := getLogs(ctx, stack, op, startTime)
			if err != nil {
				logging.V(5).Infof("failed to get logs: %v", err.Error())
			}

			for _, logEntry := range logs {
				if _, shownAlready := shown[logEntry]; !shownAlready {
					eventTime := time.Unix(0, logEntry.Timestamp*1000000)

					message := strings.TrimRight(logEntry.Message, "\n")
					display.PrintfWithWatchPrefix(eventTime, string(logEntry.ID), "%s\n", message)

					shown[logEntry] = true
				}
			}
			time.Sleep(10 * time.Second)
		}
	}()

	events := make(chan notify.EventInfo, 1)

	for _, p := range paths {
		// Provided paths can be both relative and absolute.
		watchPath := ""
		if path.IsAbs(p) {
			watchPath = path.Join(p, "...")
		} else {
			watchPath = path.Join(op.Root, p, "...")
		}

		if err := notify.Watch(watchPath, events, notify.All); err != nil {
			return result.FromError(err)
		}
	}

	defer notify.Stop(events)

	fmt.Printf(op.Opts.Display.Color.Colorize(
		colors.SpecHeadline+"Watching (%s):"+colors.Reset+"\n"), stack.Ref())

	for range events {
		display.PrintfWithWatchPrefix(time.Now(), "",
			op.Opts.Display.Color.Colorize(colors.SpecImportant+"Updating..."+colors.Reset+"\n"))

		// Perform the update operation
		_, _, res := apply(ctx, apitype.UpdateUpdate, stack, op, opts, nil)
		if res != nil {
			logging.V(5).Infof("watch update failed: %v", res.Error())
			if res.Error() == context.Canceled {
				return res
			}
			display.PrintfWithWatchPrefix(time.Now(), "",
				op.Opts.Display.Color.Colorize(colors.SpecImportant+"Update failed."+colors.Reset+"\n"))
		} else {
			display.PrintfWithWatchPrefix(time.Now(), "",
				op.Opts.Display.Color.Colorize(colors.SpecImportant+"Update complete."+colors.Reset+"\n"))
		}

	}

	return nil
}

type logsInfo struct {
	root    string
	project *workspace.Project
	target  *deploy.Target
}

func (i *logsInfo) GetRoot() string {
	return i.root
}

func (i *logsInfo) GetProject() *workspace.Project {
	return i.project
}

func (i *logsInfo) GetTarget() *deploy.Target {
	return i.target
}

func getLogs(ctx context.Context, s Stack, op UpdateOperation, startTime time.Time) ([]operations.LogEntry, error) {
	untypedDeployment, err := s.ExportDeployment(ctx)
	if err != nil {
		return nil, fmt.Errorf("exporting deployment: %w", err)
	}

	snapshot, err := stack.DeserializeUntypedDeployment(untypedDeployment, stack.DefaultSecretsProvider)
	if err != nil {
		return nil, fmt.Errorf("deserializing deployment: %w", err)
	}

	info := &logsInfo{
		root:    op.Root,
		project: op.Proj,
		target: &deploy.Target{
			Name:      s.Ref().Name(),
			Config:    op.StackConfiguration.Config,
			Decrypter: op.StackConfiguration.Decrypter,
			Snapshot:  snapshot,
		},
	}
	sink := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{Color: op.Opts.Display.Color})
	providers, err := engine.LoadProviders(info, engine.ProvidersOptions{
		Diag:       sink,
		StatusDiag: sink,
	})
	if err != nil {
		return nil, err
	}
	tree := operations.NewResourceTree(snapshot.Resources)
	ops := tree.OperationsProvider(providers)

	var entries []*logs.ResourceLogs
	var token interface{}
	for {
		batch, nextToken, err := ops.GetLogs(operations.LogQuery{
			StartTime:         &startTime,
			ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}
		entries = append(entries, batch...)
		if nextToken == nil {
			break
		}
	}
	return operations.PivotLogs(entries), nil
}
