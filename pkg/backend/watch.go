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
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	sdkDisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// Watch watches the project's working directory for changes and automatically updates the active
// stack.
func Watch(ctx context.Context, b Backend, stack Stack, op UpdateOperation,
	apply Applier, paths []string,
) error {
	opts := ApplierOptions{
		DryRun:   false,
		ShowLink: false,
	}

	startTime := time.Now()

	color := op.Opts.Display.Color

	go func() {
		shown := map[operations.LogEntry]bool{}
		for {
			logs, err := b.GetLogs(ctx, op.SecretsProvider, stack, op.StackConfiguration, operations.LogQuery{
				StartTime: &startTime,
			})
			if err != nil {
				logging.V(5).Infof("failed to get logs: %v", err.Error())
			}

			for _, logEntry := range logs {
				if _, shownAlready := shown[logEntry]; !shownAlready {
					eventTime := time.Unix(0, logEntry.Timestamp*1000000)

					message := strings.TrimRight(logEntry.Message, "\n")
					display.WatchPrefixPrintf(eventTime, color, logEntry.ID, "%s\n", message)

					shown[logEntry] = true
				}
			}
			time.Sleep(10 * time.Second)
		}
	}()

	// Provided paths can be both relative and absolute.
	events, stop, err := watchPaths(op.Root, paths)
	if err != nil {
		return err
	}
	defer stop()

	fmt.Printf(color.Colorize(
		colors.SpecHeadline+"👀 Watch mode enabled. "+colors.Reset+colors.Magenta+
			"Make an edit to %s and save to deploy.\n"+colors.Reset),
		stack.Ref().FullyQualifiedName())

	for range events {
		fmt.Print(color.Colorize(
			colors.SpecHeadline + "🚀 Save detected. " +
				colors.Reset + colors.Magenta + "Deploying changes...\n" + colors.Reset))

		// Perform the update operation
		_, changes, err := apply(ctx, apitype.UpdateUpdate, stack, op, opts, nil)

		// Display the kinds of updates performed.
		var opKinds []string
		for opKind := range changes {
			opKinds = append(opKinds, string(opKind))
		}
		opCount := 0
		sort.Strings(opKinds)
		var summary string
		for _, opKind := range opKinds {
			if opKind == "same" {
				continue
			}
			if c := changes[sdkDisplay.StepOp(opKind)]; c > 0 {
				if summary != "" {
					summary += ", "
				}
				summary += fmt.Sprintf("%d %s", c, opKind)
				opCount += c
			}
		}
		if opCount == 0 {
			summary += "None"
		}
		summary = "changes made: " + summary

		// Now summarize the outcome.
		if err != nil {
			logging.V(5).Infof("watch update failed: %v", err)
			if err == context.Canceled {
				return err
			}
			display.WatchPrefixPrintf(time.Now(), color, "",
				colors.BrightRed+colors.Bold+"❌ Update failed; "+
					colors.Reset+colors.Red+"%s."+colors.Reset+"\n", summary)
			display.WatchPrefixPrintf(time.Now(), color, "",
				colors.SpecError+fmt.Sprintf("error: %v", err)+colors.Reset+"\n")
		} else {
			display.WatchPrefixPrintf(time.Now(), color, "",
				colors.BrightGreen+colors.Bold+"🎉 Update complete; "+
					colors.Reset+colors.Green+"%s."+colors.Reset+"\n", summary)
		}
	}

	return nil
}

func watchPaths(root string, paths []string) (chan string, func(), error) {
	args := []string{"--origin", root}
	for _, p := range paths {
		var watchPath string
		if path.IsAbs(p) {
			watchPath = p
		} else {
			watchPath = path.Join(root, p)
		}

		args = append(args, "--watch", watchPath)
	}

	watchCmd, err := getWatchUtil()
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.Command(watchCmd, args...)
	cmdutil.RegisterProcessGroup(cmd)
	reader, _ := cmd.StdoutPipe()

	scanner := bufio.NewScanner(reader)
	events := make(chan string)
	go stdoutToChannel(scanner, events)
	err = cmd.Start()
	if err != nil {
		return nil, nil, fmt.Errorf("error starting pulumi-watch: %w", err)
	}

	stop := func() {
		err := cmd.Process.Kill()
		contract.AssertNoErrorf(err, "Unexpected error stopping pulumi-watch process: %v", err)
	}

	return events, stop, nil
}

const windowsGOOS = "windows"

func getWatchUtil() (string, error) {
	program := "pulumi-watch"
	if runtime.GOOS == windowsGOOS {
		program = "pulumi-watch.exe"
	}

	watchCmd, err := exec.LookPath("pulumi-watch")
	if err == nil {
		return watchCmd, nil
	}

	exePath, exeErr := os.Executable()
	if exeErr == nil {
		fullPath, fullErr := filepath.EvalSymlinks(exePath)
		if fullErr == nil {
			candidate := filepath.Join(filepath.Dir(fullPath), program)

			// Let's see if the file is executable. On Windows, os.Stat() returns a mode of "-rw-rw-rw" so on
			// on windows we just trust the fact that the .exe can actually be launched.
			if stat, err := os.Stat(candidate); err == nil {
				if stat.Mode()&0o100 != 0 || runtime.GOOS == windowsGOOS {
					return candidate, nil
				}
				return "", fmt.Errorf("Could not locate an executable pulumi-watch, found %v without execute bit", fullPath)
			}
		}
	}

	return "", errors.New("Could not locate pulumi-watch binary")
}

func stdoutToChannel(scanner *bufio.Scanner, out chan string) {
	for scanner.Scan() {
		out <- scanner.Text()
	}
}
