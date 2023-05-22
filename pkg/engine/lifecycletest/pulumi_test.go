// Copyright 2016-2022, Pulumi Corporation.
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

package lifecycletest

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func SuccessfulSteps(entries JournalEntries) []deploy.Step {
	var steps []deploy.Step
	for _, entry := range entries {
		if entry.Kind == JournalEntrySuccess {
			steps = append(steps, entry.Step)
		}
	}
	return steps
}

type StepSummary struct {
	Op  display.StepOp
	URN resource.URN
}

func AssertSameSteps(t *testing.T, expected []StepSummary, actual []deploy.Step) bool {
	assert.Equal(t, len(expected), len(actual))
	for _, exp := range expected {
		act := actual[0]
		actual = actual[1:]

		if !assert.Equal(t, exp.Op, act.Op()) || !assert.Equal(t, exp.URN, act.URN()) {
			return false
		}
	}
	return true
}

func ExpectDiagMessage(t *testing.T, messagePattern string) ValidateFunc {
	validate := func(
		project workspace.Project, target deploy.Target,
		entries JournalEntries, events []Event,
		res result.Result,
	) result.Result {
		assert.NotNil(t, res)

		for i := range events {
			if events[i].Type == "diag" {
				payload := events[i].Payload().(engine.DiagEventPayload)
				match, err := regexp.MatchString(messagePattern, payload.Message)
				if err != nil {
					return result.FromError(err)
				}
				if match {
					return nil
				}
				return result.Errorf("Unexpected diag message: %s", payload.Message)
			}
		}
		return result.Error("Expected a diagnostic message, got none")
	}
	return validate
}

func pickURN(t *testing.T, urns []resource.URN, names []string, target string) resource.URN {
	assert.Equal(t, len(urns), len(names))
	assert.Contains(t, names, target)

	for i, name := range names {
		if name == target {
			return urns[i]
		}
	}

	t.Fatalf("Could not find target: %v in %v", target, names)
	return ""
}

func TestMain(m *testing.M) {
	grpcDefault := flag.Bool("grpc-plugins", false, "enable or disable gRPC providers by default")

	flag.Parse()

	if *grpcDefault {
		deploytest.UseGrpcPluginsByDefault = true
	}

	os.Exit(m.Run())
}

func TestEmptyProgramLifecycle(t *testing.T) {
	t.Parallel()

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 0),
	}
	p.Run(t, nil)
}

func TestSingleResourceDiffUnavailable(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{}, plugin.DiffUnavailable("diff unavailable")
				},
			}, nil
		}),
	}

	inputs := resource.PropertyMap{}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update.
	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// Now run a preview. Expect a warning because the diff is unavailable.
	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result,
		) result.Result {
			found := false
			for _, e := range events {
				if e.Type == DiagEvent {
					p := e.Payload().(DiagEventPayload)
					if p.URN == resURN && p.Severity == diag.Warning && p.Message == "<{%reset%}>diff unavailable<{%reset%}>\n" {
						found = true
						break
					}
				}
			}
			assert.True(t, found)
			return res
		})
	assert.Nil(t, res)
}

// Test that ensures that we log diagnostics for resources that receive an error from Check. (Note that this
// is distinct from receiving non-error failures from Check.)
func TestCheckFailureRecord(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(urn resource.URN,
					olds, news resource.PropertyMap, randomSeed []byte,
				) (resource.PropertyMap, []plugin.CheckFailure, error) {
					return nil, nil, errors.New("oh no, check had an error")
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)
		return err
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps: []TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, res result.Result,
			) result.Result {
				sawFailure := false
				for _, evt := range evts {
					if evt.Type == DiagEvent {
						e := evt.Payload().(DiagEventPayload)
						msg := colors.Never.Colorize(e.Message)
						sawFailure = msg == "oh no, check had an error\n" && e.Severity == diag.Error
					}
				}

				assert.True(t, sawFailure)
				return res
			},
		}},
	}

	p.Run(t, nil)
}

// Test that checks that we emit diagnostics for properties that check says are invalid.
func TestCheckFailureInvalidPropertyRecord(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(urn resource.URN,
					olds, news resource.PropertyMap, randomSeed []byte,
				) (resource.PropertyMap, []plugin.CheckFailure, error) {
					return nil, []plugin.CheckFailure{{
						Property: "someprop",
						Reason:   "field is not valid",
					}}, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)
		return err
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps: []TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, res result.Result,
			) result.Result {
				sawFailure := false
				for _, evt := range evts {
					if evt.Type == DiagEvent {
						e := evt.Payload().(DiagEventPayload)
						msg := colors.Never.Colorize(e.Message)
						sawFailure = strings.Contains(msg, "field is not valid") && e.Severity == diag.Error
						if sawFailure {
							break
						}
					}
				}

				assert.True(t, sawFailure)
				return res
			},
		}},
	}

	p.Run(t, nil)
}

// Tests that errors returned directly from the language host get logged by the engine.
func TestLanguageHostDiagnostics(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	errorText := "oh no"
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		// Exiting immediately with an error simulates a language exiting immediately with a non-zero exit code.
		return errors.New(errorText)
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps: []TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, res result.Result,
			) result.Result {
				assertIsErrorOrBailResult(t, res)
				sawExitCode := false
				for _, evt := range evts {
					if evt.Type == DiagEvent {
						e := evt.Payload().(DiagEventPayload)
						msg := colors.Never.Colorize(e.Message)
						sawExitCode = strings.Contains(msg, errorText) && e.Severity == diag.Error
						if sawExitCode {
							break
						}
					}
				}

				assert.True(t, sawExitCode)
				return res
			},
		}},
	}

	p.Run(t, nil)
}

type brokenDecrypter struct {
	ErrorMessage string
}

func (b brokenDecrypter) DecryptValue(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf(b.ErrorMessage)
}

func (b brokenDecrypter) BulkDecrypt(_ context.Context, _ []string) (map[string]string, error) {
	return nil, fmt.Errorf(b.ErrorMessage)
}

// Tests that the engine presents a reasonable error message when a decrypter fails to decrypt a config value.
func TestBrokenDecrypter(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	key := config.MustMakeKey("foo", "bar")
	msg := "decryption failed"
	configMap := make(config.Map)
	configMap[key] = config.NewSecureValue("hunter2")
	p := &TestPlan{
		Options:   UpdateOptions{Host: host},
		Decrypter: brokenDecrypter{ErrorMessage: msg},
		Config:    configMap,
		Steps: []TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, res result.Result,
			) result.Result {
				assertIsErrorOrBailResult(t, res)
				decryptErr := res.Error().(DecryptError)
				assert.Equal(t, key, decryptErr.Key)
				assert.Contains(t, decryptErr.Err.Error(), msg)
				return res
			},
		}},
	}

	p.Run(t, nil)
}

func TestBadResourceType(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, _, err := mon.RegisterResource("very:bad", "resA", true)
		assert.Error(t, err)
		rpcerr, ok := rpcerror.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, rpcerr.Code())
		assert.Contains(t, rpcerr.Message(), "Type 'very:bad' is not a valid type token")

		_, _, err = mon.ReadResource("very:bad", "someResource", "someId", "", resource.PropertyMap{}, "", "")
		assert.Error(t, err)
		rpcerr, ok = rpcerror.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, rpcerr.Code())
		assert.Contains(t, rpcerr.Message(), "Type 'very:bad' is not a valid type token")

		// Component resources may have any format type.
		_, _, _, noErr := mon.RegisterResource("a:component", "resB", false)
		assert.NoError(t, noErr)

		_, _, _, noErr = mon.RegisterResource("singlename", "resC", false)
		assert.NoError(t, noErr)

		return err
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps: []TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
		}},
	}

	p.Run(t, nil)
}

// Tests that provider cancellation occurs as expected.
func TestProviderCancellation(t *testing.T) {
	t.Parallel()

	const resourceCount = 4

	// Set up a cancelable context for the refresh operation.
	ctx, cancel := context.WithCancel(context.Background())

	// Wait for our resource ops, then cancel.
	var ops sync.WaitGroup
	ops.Add(resourceCount)
	go func() {
		ops.Wait()
		cancel()
	}()

	// Set up an independent cancelable context for the provider's operations.
	provCtx, provCancel := context.WithCancel(context.Background())
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, inputs resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					// Inform the waiter that we've entered a provider op and wait for cancellation.
					ops.Done()
					<-provCtx.Done()

					return resource.ID(urn.Name()), resource.PropertyMap{}, resource.StatusOK, nil
				},
				CancelF: func() error {
					provCancel()
					return nil
				},
			}, nil
		}),
	}

	done := make(chan bool)
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		errors := make([]error, resourceCount)
		var resources sync.WaitGroup
		resources.Add(resourceCount)
		for i := 0; i < resourceCount; i++ {
			go func(idx int) {
				_, _, _, errors[idx] = monitor.RegisterResource("pkgA:m:typA", fmt.Sprintf("res%d", idx), true)
				resources.Done()
			}(i)
		}
		resources.Wait()
		for _, err := range errors {
			assert.NoError(t, err)
		}
		close(done)
		return nil
	})

	p := &TestPlan{}
	op := TestOp(Update)
	options := UpdateOptions{
		Parallel: resourceCount,
		Host:     deploytest.NewPluginHost(nil, nil, program, loaders...),
	}
	project, target := p.GetProject(), p.GetTarget(t, nil)

	_, res := op.RunWithContext(ctx, project, target, options, false, nil, nil)
	assertIsErrorOrBailResult(t, res)

	// Wait for the program to finish.
	<-done
}

// Tests that a preview works for a stack with pending operations.
func TestPreviewWithPendingOperations(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}

	const resType = "pkgA:m:typA"
	urnA := p.NewURN(resType, "resA", "")

	newResource := func(urn resource.URN, id resource.ID, delete bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       delete,
			ID:           id,
			Inputs:       resource.PropertyMap{},
			Outputs:      resource.PropertyMap{},
			Dependencies: dependencies,
		}
	}

	old := &deploy.Snapshot{
		PendingOperations: []resource.Operation{{
			Resource: newResource(urnA, "0", false),
			Type:     resource.OperationTypeUpdating,
		}},
		Resources: []*resource.State{
			newResource(urnA, "0", false),
		},
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})

	op := TestOp(Update)
	options := UpdateOptions{Host: deploytest.NewPluginHost(nil, nil, program, loaders...)}
	project, target := p.GetProject(), p.GetTarget(t, old)

	// A preview should succeed despite the pending operations.
	_, res := op.Run(project, target, options, true, nil, nil)
	assert.Nil(t, res)
}

// Tests that a refresh works for a stack with pending operations.
func TestRefreshWithPendingOperations(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}

	const resType = "pkgA:m:typA"
	urnA := p.NewURN(resType, "resA", "")

	newResource := func(urn resource.URN, id resource.ID, delete bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       delete,
			ID:           id,
			Inputs:       resource.PropertyMap{},
			Outputs:      resource.PropertyMap{},
			Dependencies: dependencies,
		}
	}

	old := &deploy.Snapshot{
		PendingOperations: []resource.Operation{{
			Resource: newResource(urnA, "0", false),
			Type:     resource.OperationTypeUpdating,
		}},
		Resources: []*resource.State{
			newResource(urnA, "0", false),
		},
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})

	op := TestOp(Update)
	options := UpdateOptions{Host: deploytest.NewPluginHost(nil, nil, program, loaders...)}
	project, target := p.GetProject(), p.GetTarget(t, old)

	// With a refresh, the update should succeed.
	withRefresh := options
	withRefresh.Refresh = true
	new, res := op.Run(project, target, withRefresh, false, nil, nil)
	assert.Nil(t, res)
	assert.Len(t, new.PendingOperations, 0)

	// Similarly, the update should succeed if performed after a separate refresh.
	new, res = TestOp(Refresh).Run(project, target, options, false, nil, nil)
	assert.Nil(t, res)
	assert.Len(t, new.PendingOperations, 0)

	_, res = op.Run(project, p.GetTarget(t, new), options, false, nil, nil)
	assert.Nil(t, res)
}

// Test to make sure that if we pulumi refresh
// while having pending CREATE operations,
// that these are preserved after the refresh.
func TestRefreshPreservesPendingCreateOperations(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}

	const resType = "pkgA:m:typA"
	urnA := p.NewURN(resType, "resA", "")
	urnB := p.NewURN(resType, "resB", "")

	newResource := func(urn resource.URN, id resource.ID, delete bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       delete,
			ID:           id,
			Inputs:       resource.PropertyMap{},
			Outputs:      resource.PropertyMap{},
			Dependencies: dependencies,
		}
	}

	// Notice here, we have two pending operations: update and create
	// After a refresh, only the pending CREATE operation should
	// be in the updated snapshot
	resA := newResource(urnA, "0", false)
	resB := newResource(urnB, "0", false)
	old := &deploy.Snapshot{
		PendingOperations: []resource.Operation{
			{
				Resource: resA,
				Type:     resource.OperationTypeUpdating,
			},
			{
				Resource: resB,
				Type:     resource.OperationTypeCreating,
			},
		},
		Resources: []*resource.State{
			resA,
		},
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})

	op := TestOp(Update)
	options := UpdateOptions{Host: deploytest.NewPluginHost(nil, nil, program, loaders...)}
	project, target := p.GetProject(), p.GetTarget(t, old)

	// With a refresh, the update should succeed.
	withRefresh := options
	withRefresh.Refresh = true
	new, res := op.Run(project, target, withRefresh, false, nil, nil)
	assert.Nil(t, res)
	// Assert that pending CREATE operation was preserved
	assert.Len(t, new.PendingOperations, 1)
	assert.Equal(t, resource.OperationTypeCreating, new.PendingOperations[0].Type)
	assert.Equal(t, urnB, new.PendingOperations[0].Resource.URN)
}

func findPendingOperationsByType(opType resource.OperationType, snapshot *deploy.Snapshot) []resource.Operation {
	var operations []resource.Operation
	for _, operation := range snapshot.PendingOperations {
		if operation.Type == opType {
			operations = append(operations, operation)
		}
	}
	return operations
}

// Update succeeds but gives a warning when there are pending operations
func TestUpdateShowsWarningWithPendingOperations(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}

	const resType = "pkgA:m:typA"
	urnA := p.NewURN(resType, "resA", "")
	urnB := p.NewURN(resType, "resB", "")

	newResource := func(urn resource.URN, id resource.ID, delete bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       delete,
			ID:           id,
			Inputs:       resource.PropertyMap{},
			Outputs:      resource.PropertyMap{},
			Dependencies: dependencies,
		}
	}

	old := &deploy.Snapshot{
		PendingOperations: []resource.Operation{
			{
				Resource: newResource(urnA, "0", false),
				Type:     resource.OperationTypeUpdating,
			},
			{
				Resource: newResource(urnB, "1", false),
				Type:     resource.OperationTypeCreating,
			},
		},
		Resources: []*resource.State{
			newResource(urnA, "0", false),
		},
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})

	op := TestOp(Update)
	options := UpdateOptions{Host: deploytest.NewPluginHost(nil, nil, program, loaders...)}
	project, target := p.GetProject(), p.GetTarget(t, old)

	// The update should succeed but give a warning
	initialPartOfMessage := "Attempting to deploy or update resources with 1 pending operations from previous deployment."
	validate := func(
		project workspace.Project, target deploy.Target,
		entries JournalEntries, events []Event,
		res result.Result,
	) result.Result {
		for i := range events {
			if events[i].Type == "diag" {
				payload := events[i].Payload().(engine.DiagEventPayload)

				if payload.Severity == "warning" && strings.Contains(payload.Message, initialPartOfMessage) {
					return nil
				}
				return result.Errorf("Unexpected warning diag message: %s", payload.Message)
			}
		}
		return result.Error("Expected a diagnostic message, got none")
	}

	new, _ := op.Run(project, target, options, false, nil, validate)
	assert.NotNil(t, new)

	assert.Equal(t, resource.OperationTypeCreating, new.PendingOperations[0].Type)

	// Assert that CREATE pending operations are retained
	// TODO: should revisit whether non-CREATE pending operations should also be retained
	assert.Equal(t, 1, len(new.PendingOperations))
	createOperations := findPendingOperationsByType(resource.OperationTypeCreating, new)
	assert.Equal(t, 1, len(createOperations))
	assert.Equal(t, urnB, createOperations[0].Resource.URN)
}

// Tests that a failed partial update causes the engine to persist the resource's old inputs and new outputs.
func TestUpdatePartialFailure(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},

				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					outputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"output_prop": 42,
					})

					return outputs, resource.StatusPartialFailure, errors.New("update failed to apply")
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, _, err := mon.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"input_prop": "new inputs",
			}),
		})
		return err
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{Options: UpdateOptions{Host: host}}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: true,
		SkipPreview:   true,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result,
		) result.Result {
			assertIsErrorOrBailResult(t, res)
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case resURN:
					assert.Equal(t, deploy.OpUpdate, entry.Step.Op())
					switch entry.Kind {
					case JournalEntryBegin:
						continue
					case JournalEntrySuccess:
						inputs := entry.Step.New().Inputs
						outputs := entry.Step.New().Outputs
						assert.Len(t, inputs, 1)
						assert.Len(t, outputs, 1)
						assert.Equal(t,
							resource.NewStringProperty("old inputs"), inputs[resource.PropertyKey("input_prop")])
						assert.Equal(t,
							resource.NewNumberProperty(42), outputs[resource.PropertyKey("output_prop")])
					default:
						t.Fatalf("unexpected journal operation: %d", entry.Kind)
					}
				}
			}

			return res
		},
	}}

	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:   resURN.Type(),
				URN:    resURN,
				Custom: true,
				ID:     "1",
				Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
					"input_prop": "old inputs",
				}),
				Outputs: resource.NewPropertyMapFromMap(map[string]interface{}{
					"output_prop": 1,
				}),
			},
		},
	}

	p.Run(t, old)
}

// Tests that the StackReference resource works as intended,
func TestStackReference(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{}

	// Test that the normal lifecycle works correctly.
	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, state, err := mon.RegisterResource("pulumi:pulumi:StackReference", "other", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "other",
			}),
		})
		assert.NoError(t, err)
		if !info.DryRun {
			assert.Equal(t, "bar", state["outputs"].ObjectValue()["foo"].StringValue())
		}
		return nil
	})
	p := &TestPlan{
		BackendClient: &deploytest.BackendClient{
			GetStackOutputsF: func(ctx context.Context, name string) (resource.PropertyMap, error) {
				switch name {
				case "other":
					return resource.NewPropertyMapFromMap(map[string]interface{}{
						"foo": "bar",
					}), nil
				default:
					return nil, fmt.Errorf("unknown stack \"%s\"", name)
				}
			},
		},
		Options: UpdateOptions{Host: deploytest.NewPluginHost(nil, nil, program, loaders...)},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)

	// Test that changes to `name` cause replacement.
	resURN := p.NewURN("pulumi:pulumi:StackReference", "other", "")
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:   resURN.Type(),
				URN:    resURN,
				Custom: true,
				ID:     "1",
				Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
					"name": "other2",
				}),
				Outputs: resource.NewPropertyMapFromMap(map[string]interface{}{
					"name":    "other2",
					"outputs": resource.PropertyMap{},
				}),
			},
		},
	}
	p.Steps = []TestStep{{
		Op:          Update,
		SkipPreview: true,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result,
		) result.Result {
			assert.Nil(t, res)
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case resURN:
					switch entry.Step.Op() {
					case deploy.OpCreateReplacement, deploy.OpDeleteReplaced, deploy.OpReplace:
						// OK
					default:
						t.Fatalf("unexpected journal operation: %v", entry.Step.Op())
					}
				}
			}

			return res
		},
	}}
	p.Run(t, old)

	// Test that unknown stacks are handled appropriately.
	program = deploytest.NewLanguageRuntime(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, _, err := mon.RegisterResource("pulumi:pulumi:StackReference", "other", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "rehto",
			}),
		})
		assert.Error(t, err)
		return err
	})
	p.Options = UpdateOptions{Host: deploytest.NewPluginHost(nil, nil, program, loaders...)}
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: true,
		SkipPreview:   true,
	}}
	p.Run(t, nil)

	// Test that unknown properties cause errors.
	program = deploytest.NewLanguageRuntime(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, _, err := mon.RegisterResource("pulumi:pulumi:StackReference", "other", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "other",
				"foo":  "bar",
			}),
		})
		assert.Error(t, err)
		return err
	})
	p.Options = UpdateOptions{Host: deploytest.NewPluginHost(nil, nil, program, loaders...)}
	p.Run(t, nil)
}

type channelWriter struct {
	channel chan []byte
}

func (cw *channelWriter) Write(d []byte) (int, error) {
	cw.channel <- d
	return len(d), nil
}

// Tests that a failed plugin load correctly shuts down the host.
func TestLoadFailureShutdown(t *testing.T) {
	t.Parallel()

	// Note that the setup here is a bit baroque, and is intended to replicate the CLI architecture that lead to
	// issue #2170. That issue--a panic on a closed channel--was caused by the intersection of several design choices:
	//
	// - The provider registry loads and configures the set of providers necessary for the resources currently in the
	//   checkpoint it is processing at plan creation time. Registry creation fails promptly if a provider plugin
	//   fails to load (e.g. because is binary is missing).
	// - Provider configuration in the CLI's host happens asynchronously. This is meant to allow the engine to remain
	//   responsive while plugins configure.
	// - Providers may call back into the CLI's host for logging. Callbacks are processed as long as the CLI's plugin
	//   context is open.
	// - Log events from the CLI's host are delivered to the CLI's diagnostic streams via channels. The CLI closes
	//   these channels once the engine operation it initiated completes.
	//
	// These choices gave rise to the following situation:
	// 1. The provider registry loads a provider for package A and kicks off its configuration.
	// 2. The provider registry attempts to load a provider for package B. The load fails, and the provider registry
	//   creation call fails promptly.
	// 3. The engine operation requested by the CLI fails promptly because provider registry creation failed.
	// 4. The CLI shuts down its diagnostic channels.
	// 5. The provider for package A calls back in to the host to log a message. The host then attempts to deliver
	//    the message to the CLI's diagnostic channels, causing a panic.
	//
	// The fix was to properly close the plugin host during step (3) s.t. the host was no longer accepting callbacks
	// and would not attempt to send messages to the CLI's diagnostic channels.
	//
	// As such, this test attempts to replicate the CLI architecture by using one provider that configures
	// asynchronously and attempts to call back into the engine and a second provider that fails to load.
	//
	// The engine architecture has changed since this issue was discovered, and the test has been updated to
	// reflect that. Registry creation no longer configures providers up front, so the program below tries to
	// register two providers instead.

	release, done := make(chan bool), make(chan bool)
	sinkWriter := &channelWriter{channel: make(chan []byte)}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoaderWithHost("pkgA", semver.MustParse("1.0.0"),
			func(host plugin.Host) (plugin.Provider, error) {
				return &deploytest.Provider{
					ConfigureF: func(news resource.PropertyMap) error {
						go func() {
							<-release
							host.Log(diag.Info, "", "configuring pkgA provider...", 0)
							close(done)
						}()
						return nil
					},
				}, nil
			}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return nil, errors.New("pkgB load failure")
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource(providers.MakeProviderType("pkgB"), "provB", true)
		assert.NoError(t, err)

		return nil
	})

	op := TestOp(Update)
	sink := diag.DefaultSink(sinkWriter, sinkWriter, diag.FormatOptions{Color: colors.Raw})
	options := UpdateOptions{Host: deploytest.NewPluginHost(sink, sink, program, loaders...)}
	p := &TestPlan{}
	project, target := p.GetProject(), p.GetTarget(t, nil)

	_, res := op.Run(project, target, options, true, nil, nil)
	assertIsErrorOrBailResult(t, res)

	close(sinkWriter.channel)
	close(release)
	<-done
}

func TestSingleResourceIgnoreChanges(t *testing.T) {
	t.Parallel()

	var expectedIgnoreChanges []string

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					assert.Equal(t, expectedIgnoreChanges, ignoreChanges)
					return plugin.DiffResult{}, nil
				},
				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					assert.Equal(t, expectedIgnoreChanges, ignoreChanges)
					return resource.PropertyMap{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	updateProgramWithProps := func(snap *deploy.Snapshot, props resource.PropertyMap, ignoreChanges []string,
		allowedOps []display.StepOp,
	) *deploy.Snapshot {
		expectedIgnoreChanges = ignoreChanges
		program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:        props,
				IgnoreChanges: ignoreChanges,
			})
			assert.NoError(t, err)
			return nil
		})
		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result,
					) result.Result {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Subset(t, allowedOps, []display.StepOp{payload.Metadata.Op})
							}
						}
						return res
					},
				},
			},
		}
		return p.Run(t, snap)
	}

	snap := updateProgramWithProps(nil, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": "foo",
		},
	}), []string{"a", "b.c"}, []display.StepOp{deploy.OpCreate})

	// Ensure that a change to an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 2,
		"b": map[string]interface{}{
			"c": "bar",
		},
	}), []string{"a", "b.c"}, []display.StepOp{deploy.OpSame})

	// Ensure that a change to an un-ignored property results in an OpUpdate
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 3,
		"b": map[string]interface{}{
			"c": "qux",
		},
	}), nil, []display.StepOp{deploy.OpUpdate})

	// Ensure that a removing an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.PropertyMap{}, []string{"a", "b"}, []display.StepOp{deploy.OpSame})

	// Ensure that a removing an un-ignored property results in an OpUpdate
	snap = updateProgramWithProps(snap, resource.PropertyMap{}, nil, []display.StepOp{deploy.OpUpdate})

	// Ensure that adding an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 4,
		"b": map[string]interface{}{
			"c": "zed",
		},
	}), []string{"a", "b"}, []display.StepOp{deploy.OpSame})

	// Ensure that adding an un-ignored property results in an OpUpdate
	_ = updateProgramWithProps(snap, resource.PropertyMap{
		"c": resource.NewNumberProperty(4),
	}, []string{"a", "b"}, []display.StepOp{deploy.OpUpdate})
}

func TestIgnoreChangesInvalidPaths(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := func(monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewObjectProperty(resource.PropertyMap{
					"bar": resource.NewStringProperty("baz"),
				}),
				"qux": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("zed"),
				}),
			},
		})
		assert.NoError(t, err)
		return nil
	}

	runtime := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return program(monitor)
	})
	host := deploytest.NewPluginHost(nil, nil, runtime, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	program = func(monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:        resource.PropertyMap{},
			IgnoreChanges: []string{"foo.bar"},
		})
		assert.Error(t, err)
		return nil
	}

	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)

	program = func(monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:        resource.PropertyMap{},
			IgnoreChanges: []string{"qux[0]"},
		})
		assert.Error(t, err)
		return nil
	}

	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)
}

type DiffFunc = func(urn resource.URN, id resource.ID,
	olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error)

func replaceOnChangesTest(t *testing.T, name string, diffFunc DiffFunc) {
	t.Run(name, func(t *testing.T) {
		t.Parallel()

		loaders := []*deploytest.ProviderLoader{
			deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
				return &deploytest.Provider{
					DiffF: diffFunc,
					UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
						ignoreChanges []string, preview bool,
					) (resource.PropertyMap, resource.Status, error) {
						return news, resource.StatusOK, nil
					},
					CreateF: func(urn resource.URN, inputs resource.PropertyMap, timeout float64,
						preview bool,
					) (resource.ID, resource.PropertyMap, resource.Status, error) {
						return resource.ID("id123"), inputs, resource.StatusOK, nil
					},
				}, nil
			}),
		}

		updateProgramWithProps := func(snap *deploy.Snapshot, props resource.PropertyMap, replaceOnChanges []string,
			allowedOps []display.StepOp,
		) *deploy.Snapshot {
			program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
					Inputs:           props,
					ReplaceOnChanges: replaceOnChanges,
				})
				assert.NoError(t, err)
				return nil
			})
			host := deploytest.NewPluginHost(nil, nil, program, loaders...)
			p := &TestPlan{
				Options: UpdateOptions{Host: host},
				Steps: []TestStep{
					{
						Op: Update,
						Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
							events []Event, res result.Result,
						) result.Result {
							for _, event := range events {
								if event.Type == ResourcePreEvent {
									payload := event.Payload().(ResourcePreEventPayload)
									assert.Subset(t, allowedOps, []display.StepOp{payload.Metadata.Op})
								}
							}
							return res
						},
					},
				},
			}
			return p.Run(t, snap)
		}

		snap := updateProgramWithProps(nil, resource.NewPropertyMapFromMap(map[string]interface{}{
			"a": 1,
			"b": map[string]interface{}{
				"c": "foo",
			},
		}), []string{"a", "b.c"}, []display.StepOp{deploy.OpCreate})

		// Ensure that a change to a replaceOnChange property results in an OpReplace
		snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
			"a": 2,
			"b": map[string]interface{}{
				"c": "foo",
			},
		}), []string{"a"}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

		// Ensure that a change to a nested replaceOnChange property results in an OpReplace
		snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
			"a": 2,
			"b": map[string]interface{}{
				"c": "bar",
			},
		}), []string{"b.c"}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

		// Ensure that a change to any property of a "*" replaceOnChange results in an OpReplace
		snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
			"a": 3,
			"b": map[string]interface{}{
				"c": "baz",
			},
		}), []string{"*"}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

		// Ensure that a change to an non-replaceOnChange property results in an OpUpdate
		snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
			"a": 4,
			"b": map[string]interface{}{
				"c": "qux",
			},
		}), nil, []display.StepOp{deploy.OpUpdate})

		// We ensure that we are listing to the engine diff function only when the provider function
		// is nil. We do this by adding some weirdness to the provider diff function.
		allowed := []display.StepOp{deploy.OpCreateReplacement, deploy.OpReplace, deploy.OpDeleteReplaced}
		if diffFunc != nil {
			allowed = []display.StepOp{deploy.OpSame}
		}
		snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
			"a": 42, // 42 is a special value in the "provider" diff function.
			"b": map[string]interface{}{
				"c": "qux",
			},
		}), []string{"a"}, allowed)

		_ = snap
	})
}

func TestReplaceOnChanges(t *testing.T) {
	t.Parallel()

	// We simulate a provider that has it's own diff function.
	replaceOnChangesTest(t, "provider diff",
		func(urn resource.URN, id resource.ID,
			olds, news resource.PropertyMap, ignoreChanges []string,
		) (plugin.DiffResult, error) {
			// To establish a observable difference between the provider and engine diff function,
			// we treat 42 as an OpSame. We use this to check that the right diff function is being
			// used.
			for k, v := range news {
				if v == resource.NewNumberProperty(42) {
					news[k] = olds[k]
				}
			}
			diff := olds.Diff(news)
			if diff == nil {
				return plugin.DiffResult{Changes: plugin.DiffNone}, nil
			}
			detailedDiff := plugin.NewDetailedDiffFromObjectDiff(diff)
			changedKeys := diff.ChangedKeys()

			return plugin.DiffResult{
				Changes:      plugin.DiffSome,
				ChangedKeys:  changedKeys,
				DetailedDiff: detailedDiff,
			}, nil
		})

	// We simulate a provider that does not have it's own diff function. This tests the engines diff
	// function instead.
	replaceOnChangesTest(t, "engine diff", nil)
}

// Resource is an abstract representation of a resource graph
type Resource struct {
	t                   tokens.Type
	name                string
	children            []Resource
	props               resource.PropertyMap
	aliasURNs           []resource.URN
	aliases             []resource.Alias
	dependencies        []resource.URN
	parent              resource.URN
	deleteBeforeReplace bool
}

func registerResources(t *testing.T, monitor *deploytest.ResourceMonitor, resources []Resource) error {
	for _, r := range resources {
		_, _, _, err := monitor.RegisterResource(r.t, r.name, true, deploytest.ResourceOptions{
			Parent:              r.parent,
			Dependencies:        r.dependencies,
			Inputs:              r.props,
			DeleteBeforeReplace: &r.deleteBeforeReplace,
			AliasURNs:           r.aliasURNs,
			Aliases:             r.aliases,
		})
		if err != nil {
			return err
		}
		err = registerResources(t, monitor, r.children)
		if err != nil {
			return err
		}
	}
	return nil
}

func TestAliases(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// The `forcesReplacement` key forces replacement and all other keys can update in place
				DiffF: func(res resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					replaceKeys := []resource.PropertyKey{}
					old, hasOld := olds["forcesReplacement"]
					new, hasNew := news["forcesReplacement"]
					if hasOld && !hasNew || hasNew && !hasOld || hasOld && hasNew && old.Diff(new) != nil {
						replaceKeys = append(replaceKeys, "forcesReplacement")
					}
					return plugin.DiffResult{ReplaceKeys: replaceKeys}, nil
				},
			}, nil
		}),
	}

	updateProgramWithResource := func(
		snap *deploy.Snapshot, resources []Resource, allowedOps []display.StepOp, expectFailure bool,
	) *deploy.Snapshot {
		program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			err := registerResources(t, monitor, resources)
			return err
		})
		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op:            Update,
					ExpectFailure: expectFailure,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result,
					) result.Result {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Subset(t, allowedOps, []display.StepOp{payload.Metadata.Op})
							}
						}

						for _, entry := range entries {
							if entry.Step.Type() == "pulumi:providers:pkgA" {
								continue
							}
							switch entry.Kind {
							case JournalEntrySuccess:
								assert.Subset(t, allowedOps, []display.StepOp{entry.Step.Op()})
							case JournalEntryFailure:
								assert.Fail(t, "unexpected failure in journal")
							case JournalEntryBegin:
							case JournalEntryOutputs:
							}
						}

						return res
					},
				},
			},
		}
		return p.Run(t, snap)
	}

	snap := updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Ensure that rename produces Same
	snap = updateProgramWithResource(snap, []Resource{{
		t:       "pkgA:index:t1",
		name:    "n2",
		aliases: []resource.Alias{{URN: "urn:pulumi:test::test::pkgA:index:t1::n1"}},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that rename produces Same with multiple aliases
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliases: []resource.Alias{
			{Name: "n2", Type: "pkgA:index:t1", Stack: "test", Project: "test"},
			{Name: "n1", Type: "pkgA:index:t1", Stack: "test", Project: "test"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that rename produces Same with multiple aliases (reversed)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n2"},
			{Name: "n1", Type: "pkgA:index:t1", Stack: "test", Project: "test"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that aliasing back to original name is okay
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n3"},
			{Name: "n2", Type: "pkgA:index:t1", Stack: "test", Project: "test"},
		},
		aliasURNs: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1::n3"},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing the type works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t2",
		name: "n1",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n1"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing the type again works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n1"},
			{URN: "urn:pulumi:test::test::pkgA:index:t2::n1"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that order of aliases doesn't matter
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n1"},
			{URN: "urn:pulumi:test::test::pkgA:othermod:t3::n1"},
			{URN: "urn:pulumi:test::test::pkgA:index:t2::n1"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing everything (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t4",
		name: "n2",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(42),
		},
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:othermod:t3::n1"},
		},
	}}, []display.StepOp{deploy.OpUpdate}, false)

	// Ensure that changing everything again (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t5",
		name: "n3",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(1000),
		},
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t4::n2"},
		},
	}}, []display.StepOp{deploy.OpUpdate}, false)

	// Ensure that changing a forceNew property while also changing type and name leads to replacement not delete+create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t6",
		name: "n4",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1000),
		},
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t5::n3"},
		},
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Ensure that changing a forceNew property and deleteBeforeReplace while also changing type and name leads to
	// replacement not delete+create
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t7",
		name: "n5",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(999),
		},
		deleteBeforeReplace: true,
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t6::n4"},
		},
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Start again - this time with two resources with depends on relationship
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1),
		},
		deleteBeforeReplace: true,
	}, {
		t:            "pkgA:index:t2",
		name:         "n2",
		dependencies: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1::n1"},
	}}, []display.StepOp{deploy.OpCreate}, false)

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n1"},
		},
	}, {
		t:            "pkgA:index:t2-new",
		name:         "n2-new",
		dependencies: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1-new::n1-new"},
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t2::n2"},
		},
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Start again - this time with two resources with parent relationship
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1),
		},
		deleteBeforeReplace: true,
	}, {
		t:      "pkgA:index:t2",
		name:   "n2",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1::n1"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n1"},
		},
	}, {
		t:      "pkgA:index:t2-new",
		name:   "n2-new",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new::n1-new"),
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n2"},
		},
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// ensure failure when different resources use duplicate aliases
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n2",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n1"},
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n3",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n1"},
		},
	}}, []display.StepOp{deploy.OpCreate}, true)

	// ensure different resources can use different aliases
	_ = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n1"},
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n2",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n2"},
		},
	}}, []display.StepOp{deploy.OpCreate}, false)

	// ensure that aliases of parents of parents resolves correctly
	// first create a chain of resources such that we have n1 -> n1-sub -> n1-sub-sub
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}, {
		t:      "pkgA:index:t2",
		name:   "n1-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1::n1"),
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n1-sub"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Now change n1's name and type
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n1"},
		},
	}, {
		t:      "pkgA:index:t2",
		name:   "n1-new-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new::n1-new"),
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n1-sub"},
		},
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-new-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new$pkgA:index:t2::n1-new-sub"),
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2$pkgA:index:t3::n1-sub-sub"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Test catastrophic multiplication out of aliases doesn't crash out of memory
	// first create a chain of resources such that we have n1 -> n1-sub -> n1-sub-sub
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1-v0",
		name: "n1",
	}, {
		t:      "pkgA:index:t2-v0",
		name:   "n1-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-v0::n1"),
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-v0$pkgA:index:t2-v0::n1-sub"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Now change n1's name and type and n2's type, but also add a load of aliases and pre-multiply them out
	// before sending to the engine
	n1Aliases := make([]resource.Alias, 0)
	n2Aliases := make([]resource.Alias, 0)
	n3Aliases := make([]resource.Alias, 0)
	for i := 0; i < 100; i++ {
		n1Aliases = append(n1Aliases, resource.Alias{URN: resource.URN(
			fmt.Sprintf("urn:pulumi:test::test::pkgA:index:t1-v%d::n1", i),
		)})

		for j := 0; j < 10; j++ {
			n2Aliases = append(n2Aliases, resource.Alias{
				URN: resource.URN(fmt.Sprintf("urn:pulumi:test::test::pkgA:index:t1-v%d$pkgA:index:t2-v%d::n1-sub", i, j)),
			})
			n3Aliases = append(n3Aliases, resource.Alias{
				Name:    "n1-sub-sub",
				Type:    fmt.Sprintf("pkgA:index:t1-v%d$pkgA:index:t2-v%d$pkgA:index:t3", i, j),
				Stack:   "test",
				Project: "test",
			})
		}
	}

	snap = updateProgramWithResource(snap, []Resource{{
		t:       "pkgA:index:t1-v100",
		name:    "n1-new",
		aliases: n1Aliases,
	}, {
		t:       "pkgA:index:t2-v10",
		name:    "n1-new-sub",
		parent:  resource.URN("urn:pulumi:test::test::pkgA:index:t1-v100::n1-new"),
		aliases: n2Aliases,
	}, {
		t:       "pkgA:index:t3",
		name:    "n1-new-sub-sub",
		parent:  resource.URN("urn:pulumi:test::test::pkgA:index:t1-v100$pkgA:index:t2-v10::n1-new-sub"),
		aliases: n3Aliases,
	}}, []display.StepOp{deploy.OpSame}, false)

	var err error
	_, err = snap.NormalizeURNReferences()
	assert.Nil(t, err)
}

func TestAliasURNs(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// The `forcesReplacement` key forces replacement and all other keys can update in place
				DiffF: func(res resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					replaceKeys := []resource.PropertyKey{}
					old, hasOld := olds["forcesReplacement"]
					new, hasNew := news["forcesReplacement"]
					if hasOld && !hasNew || hasNew && !hasOld || hasOld && hasNew && old.Diff(new) != nil {
						replaceKeys = append(replaceKeys, "forcesReplacement")
					}
					return plugin.DiffResult{ReplaceKeys: replaceKeys}, nil
				},
			}, nil
		}),
	}

	updateProgramWithResource := func(
		snap *deploy.Snapshot, resources []Resource, allowedOps []display.StepOp, expectFailure bool,
	) *deploy.Snapshot {
		program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			err := registerResources(t, monitor, resources)
			return err
		})
		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op:            Update,
					ExpectFailure: expectFailure,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result,
					) result.Result {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Subset(t, allowedOps, []display.StepOp{payload.Metadata.Op})
							}
						}

						for _, entry := range entries {
							if entry.Step.Type() == "pulumi:providers:pkgA" {
								continue
							}
							switch entry.Kind {
							case JournalEntrySuccess:
								assert.Subset(t, allowedOps, []display.StepOp{entry.Step.Op()})
							case JournalEntryFailure:
								assert.Fail(t, "unexpected failure in journal")
							case JournalEntryBegin:
							case JournalEntryOutputs:
							}
						}

						return res
					},
				},
			},
		}
		return p.Run(t, snap)
	}

	snap := updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Ensure that rename produces Same
	snap = updateProgramWithResource(snap, []Resource{{
		t:         "pkgA:index:t1",
		name:      "n2",
		aliasURNs: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1::n1"},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that rename produces Same with multiple aliases
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:index:t1::n2",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that rename produces Same with multiple aliases (reversed)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n2",
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that aliasing back to original name is okay
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n3",
			"urn:pulumi:test::test::pkgA:index:t1::n2",
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing the type works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t2",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing the type again works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:index:t2::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that order of aliases doesn't matter
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:othermod:t3::n1",
			"urn:pulumi:test::test::pkgA:index:t2::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing everything (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t4",
		name: "n2",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(42),
		},
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:othermod:t3::n1",
		},
	}}, []display.StepOp{deploy.OpUpdate}, false)

	// Ensure that changing everything again (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t5",
		name: "n3",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(1000),
		},
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t4::n2",
		},
	}}, []display.StepOp{deploy.OpUpdate}, false)

	// Ensure that changing a forceNew property while also changing type and name leads to replacement not delete+create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t6",
		name: "n4",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1000),
		},
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t5::n3",
		},
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Ensure that changing a forceNew property and deleteBeforeReplace while also changing type and name leads to
	// replacement not delete+create
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t7",
		name: "n5",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(999),
		},
		deleteBeforeReplace: true,
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t6::n4",
		},
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Start again - this time with two resources with depends on relationship
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1),
		},
		deleteBeforeReplace: true,
	}, {
		t:            "pkgA:index:t2",
		name:         "n2",
		dependencies: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1::n1"},
	}}, []display.StepOp{deploy.OpCreate}, false)

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:            "pkgA:index:t2-new",
		name:         "n2-new",
		dependencies: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1-new::n1-new"},
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t2::n2",
		},
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Start again - this time with two resources with parent relationship
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1),
		},
		deleteBeforeReplace: true,
	}, {
		t:      "pkgA:index:t2",
		name:   "n2",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1::n1"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:      "pkgA:index:t2-new",
		name:   "n2-new",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new::n1-new"),
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n2",
		},
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// ensure failure when different resources use duplicate aliases
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n2",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n3",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []display.StepOp{deploy.OpCreate}, true)

	// ensure different resources can use different aliases
	_ = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n2",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n2",
		},
	}}, []display.StepOp{deploy.OpCreate}, false)

	// ensure that aliases of parents of parents resolves correctly
	// first create a chain of resources such that we have n1 -> n1-sub -> n1-sub-sub
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}, {
		t:      "pkgA:index:t2",
		name:   "n1-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1::n1"),
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n1-sub"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Now change n1's name and type
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:      "pkgA:index:t2",
		name:   "n1-new-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new::n1-new"),
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n1-sub",
		},
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-new-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new$pkgA:index:t2::n1-new-sub"),
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2$pkgA:index:t3::n1-sub-sub",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Test catastrophic multiplication out of aliases doesn't crash out of memory
	// first create a chain of resources such that we have n1 -> n1-sub -> n1-sub-sub
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1-v0",
		name: "n1",
	}, {
		t:      "pkgA:index:t2-v0",
		name:   "n1-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-v0::n1"),
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-v0$pkgA:index:t2-v0::n1-sub"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Now change n1's name and type and n2's type, but also add a load of aliases and pre-multiply them out
	// before sending to the engine
	n1Aliases := make([]resource.URN, 0)
	n2Aliases := make([]resource.URN, 0)
	n3Aliases := make([]resource.URN, 0)
	for i := 0; i < 100; i++ {
		n1Aliases = append(n1Aliases, resource.URN(
			fmt.Sprintf("urn:pulumi:test::test::pkgA:index:t1-v%d::n1", i)))

		for j := 0; j < 10; j++ {
			n2Aliases = append(n2Aliases, resource.URN(
				fmt.Sprintf("urn:pulumi:test::test::pkgA:index:t1-v%d$pkgA:index:t2-v%d::n1-sub", i, j)))

			n3Aliases = append(n3Aliases, resource.URN(
				fmt.Sprintf("urn:pulumi:test::test::pkgA:index:t1-v%d$pkgA:index:t2-v%d$pkgA:index:t3::n1-sub-sub", i, j)))
		}
	}

	snap = updateProgramWithResource(snap, []Resource{{
		t:         "pkgA:index:t1-v100",
		name:      "n1-new",
		aliasURNs: n1Aliases,
	}, {
		t:         "pkgA:index:t2-v10",
		name:      "n1-new-sub",
		parent:    resource.URN("urn:pulumi:test::test::pkgA:index:t1-v100::n1-new"),
		aliasURNs: n2Aliases,
	}, {
		t:         "pkgA:index:t3",
		name:      "n1-new-sub-sub",
		parent:    resource.URN("urn:pulumi:test::test::pkgA:index:t1-v100$pkgA:index:t2-v10::n1-new-sub"),
		aliasURNs: n3Aliases,
	}}, []display.StepOp{deploy.OpSame}, false)

	var err error
	_, err = snap.NormalizeURNReferences()
	assert.Nil(t, err)
}

func TestPersistentDiff(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{Changes: plugin.DiffSome}, nil
				},
			}, nil
		}),
	}

	inputs := resource.PropertyMap{}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update.
	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// First, make no change to the inputs and run a preview. We should see an update to the resource due to
	// provider diffing.
	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result,
		) result.Result {
			found := false
			for _, e := range events {
				if e.Type == ResourcePreEvent {
					p := e.Payload().(ResourcePreEventPayload).Metadata
					if p.URN == resURN {
						assert.Equal(t, deploy.OpUpdate, p.Op)
						found = true
					}
				}
			}
			assert.True(t, found)
			return res
		})
	assert.Nil(t, res)

	// Next, enable legacy diff behavior. We should see no changes to the resource.
	p.Options.UseLegacyDiff = true
	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result,
		) result.Result {
			found := false
			for _, e := range events {
				if e.Type == ResourcePreEvent {
					p := e.Payload().(ResourcePreEventPayload).Metadata
					if p.URN == resURN {
						assert.Equal(t, deploy.OpSame, p.Op)
						found = true
					}
				}
			}
			assert.True(t, found)
			return res
		})
	assert.Nil(t, res)
}

func TestDetailedDiffReplace(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
						DetailedDiff: map[string]plugin.PropertyDiff{
							"prop": {Kind: plugin.DiffAddReplace},
						},
					}, nil
				},
			}, nil
		}),
	}

	inputs := resource.PropertyMap{}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update.
	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// First, make no change to the inputs and run a preview. We should see an update to the resource due to
	// provider diffing.
	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result,
		) result.Result {
			found := false
			for _, e := range events {
				if e.Type == ResourcePreEvent {
					p := e.Payload().(ResourcePreEventPayload).Metadata
					if p.URN == resURN && p.Op == deploy.OpReplace {
						found = true
					}
				}
			}
			assert.True(t, found)
			return res
		})
	assert.Nil(t, res)
}

func TestCustomTimeouts(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			CustomTimeouts: &resource.CustomTimeouts{
				Create: 60, Delete: 60, Update: 240,
			},
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default")
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.NotNil(t, snap.Resources[1].CustomTimeouts)
	assert.Equal(t, snap.Resources[1].CustomTimeouts.Create, float64(60))
	assert.Equal(t, snap.Resources[1].CustomTimeouts.Update, float64(240))
	assert.Equal(t, snap.Resources[1].CustomTimeouts.Delete, float64(60))
}

func TestProviderDiffMissingOldOutputs(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					// Always require replacement if any diff exists.
					if !olds.DeepEquals(news) {
						keys := []resource.PropertyKey{}
						for k := range news {
							keys = append(keys, k)
						}
						return plugin.DiffResult{Changes: plugin.DiffSome, ReplaceKeys: keys}, nil
					}
					return plugin.DiffResult{Changes: plugin.DiffNone}, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Config: config.Map{
			config.MustMakeKey("pkgA", "foo"): config.NewValue("bar"),
		},
	}

	// Build a basic lifecycle.
	steps := MakeBasicLifecycleSteps(t, 2)

	// Run the lifecycle through its initial update and refresh.
	p.Steps = steps[:2]
	snap := p.Run(t, nil)

	// Delete the old provider outputs (if any) from the checkpoint, then run the no-op update.
	providerURN := p.NewProviderURN("pkgA", "default", "")
	for _, r := range snap.Resources {
		if r.URN == providerURN {
			r.Outputs = nil
		}
	}

	p.Steps = steps[2:3]
	snap = p.Run(t, snap)

	// Change the config, delete the old provider outputs,  and run an update. We expect everything to require
	// replacement.
	p.Config[config.MustMakeKey("pkgA", "foo")] = config.NewValue("baz")
	for _, r := range snap.Resources {
		if r.URN == providerURN {
			r.Outputs = nil
		}
	}
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result,
		) result.Result {
			resURN := p.NewURN("pkgA:m:typA", "resA", "")

			// Look for replace steps on the provider and the resource.
			replacedProvider, replacedResource := false, false
			for _, entry := range entries {
				if entry.Kind != JournalEntrySuccess || entry.Step.Op() != deploy.OpDeleteReplaced {
					continue
				}

				switch urn := entry.Step.URN(); urn {
				case providerURN:
					replacedProvider = true
				case resURN:
					replacedResource = true
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			assert.True(t, replacedProvider)
			assert.True(t, replacedResource)

			return res
		},
	}}
	p.Run(t, snap)
}

func TestMissingRead(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ resource.URN, _ resource.ID, _, _ resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	// Our program reads a resource and exits.
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "resA-some-id", "", resource.PropertyMap{}, "", "")
		assert.Error(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   []TestStep{{Op: Update, ExpectFailure: true}},
	}
	p.Run(t, nil)
}

func TestProviderPreview(t *testing.T) {
	t.Parallel()

	sawPreview := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					if preview {
						sawPreview = true
					}

					assert.Equal(t, preview, news.ContainsUnknowns())
					return "created-id", news, resource.StatusOK, nil
				},
				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					if preview {
						sawPreview = true
					}

					assert.Equal(t, preview, news.ContainsUnknowns())
					return news, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	preview := true
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		computed := interface{}(resource.Computed{Element: resource.NewStringProperty("")})
		if !preview {
			computed = "alpha"
		}

		ins := resource.NewPropertyMapFromMap(map[string]interface{}{
			"foo": "bar",
			"baz": map[string]interface{}{
				"a": 42,
				"b": computed,
			},
			"qux": []interface{}{
				computed,
				24,
			},
			"zed": computed,
		})

		_, _, state, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		assert.True(t, state.DeepEquals(ins))

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run a preview. The inputs should be propagated to the outputs by the provider during the create.
	preview, sawPreview = true, false
	_, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.True(t, sawPreview)

	// Run an update.
	preview, sawPreview = false, false
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.False(t, sawPreview)

	// Run another preview. The inputs should be propagated to the outputs during the update.
	preview, sawPreview = true, false
	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.True(t, sawPreview)
}

func TestProviderPreviewGrpc(t *testing.T) {
	t.Parallel()

	sawPreview := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					if preview {
						sawPreview = true
					}

					assert.Equal(t, preview, news.ContainsUnknowns())
					return "created-id", news, resource.StatusOK, nil
				},
				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					if preview {
						sawPreview = true
					}

					assert.Equal(t, preview, news.ContainsUnknowns())
					return news, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	preview := true
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		computed := interface{}(resource.Computed{Element: resource.NewStringProperty("")})
		if !preview {
			computed = "alpha"
		}

		ins := resource.NewPropertyMapFromMap(map[string]interface{}{
			"foo": "bar",
			"baz": map[string]interface{}{
				"a": 42,
				"b": computed,
			},
			"qux": []interface{}{
				computed,
				24,
			},
			"zed": computed,
		})

		_, _, state, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		assert.True(t, state.DeepEquals(ins))

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run a preview. The inputs should be propagated to the outputs by the provider during the create.
	preview, sawPreview = true, false
	_, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.True(t, sawPreview)

	// Run an update.
	preview, sawPreview = false, false
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.False(t, sawPreview)

	// Run another preview. The inputs should be propagated to the outputs during the update.
	preview, sawPreview = true, false
	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.True(t, sawPreview)
}

func TestProviderPreviewUnknowns(t *testing.T) {
	t.Parallel()

	sawPreview := false
	loaders := []*deploytest.ProviderLoader{
		// NOTE: it is important that this test uses a gRPC-wraped provider. The code that handles previews for unconfigured
		// providers is specific to the gRPC layer.
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					if preview {
						sawPreview = true
					}

					return "created-id", news, resource.StatusOK, nil
				},
				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					if preview {
						sawPreview = true
					}

					return news, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	preview := true
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		computed := interface{}(resource.Computed{Element: resource.NewStringProperty("")})
		if !preview {
			computed = "alpha"
		}

		provURN, provID, _, err := monitor.RegisterResource("pulumi:providers:pkgA", "provA", true,
			deploytest.ResourceOptions{
				Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{"foo": computed}),
			})
		require.NoError(t, err)

		ins := resource.NewPropertyMapFromMap(map[string]interface{}{
			"foo": "bar",
			"baz": map[string]interface{}{
				"a": 42,
			},
			"qux": []interface{}{
				24,
			},
		})

		_, _, state, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:   ins,
			Provider: fmt.Sprintf("%v::%v", provURN, provID),
		})
		require.NoError(t, err)

		if preview {
			assert.True(t, state.DeepEquals(resource.PropertyMap{}))
		} else {
			assert.True(t, state.DeepEquals(ins))
		}

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run a preview. The inputs should not be propagated to the outputs by the provider during the create because the
	// provider has unknown inputs.
	preview, sawPreview = true, false
	_, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	require.Nil(t, res)
	assert.False(t, sawPreview)

	// Run an update.
	preview, sawPreview = false, false
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	require.Nil(t, res)
	assert.False(t, sawPreview)

	// Run another preview. The inputs should not be propagated to the outputs during the update because the provider
	// has unknown inputs.
	preview, sawPreview = true, false
	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, preview, p.BackendClient, nil)
	require.Nil(t, res)
	assert.False(t, sawPreview)
}

func TestSingleComponentDefaultProviderLifecycle(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(monitor *deploytest.ResourceMonitor,
				typ, name string, parent resource.URN, inputs resource.PropertyMap,
				options plugin.ConstructOptions,
			) (plugin.ConstructResult, error) {
				urn, _, _, err := monitor.RegisterResource(tokens.Type(typ), name, false, deploytest.ResourceOptions{
					Parent:  parent,
					Aliases: options.Aliases,
					Protect: options.Protect,
				})
				assert.NoError(t, err)

				_, _, _, err = monitor.RegisterResource("pkgA:m:typB", "resA", true, deploytest.ResourceOptions{
					Parent: urn,
				})
				assert.NoError(t, err)

				outs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
				err = monitor.RegisterResourceOutputs(urn, outs)
				assert.NoError(t, err)

				return plugin.ConstructResult{
					URN:     urn,
					Outputs: outs,
				}, nil
			}

			return &deploytest.Provider{
				ConstructF: construct,
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, state, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}, state)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 3),
	}
	p.Run(t, nil)
}

type updateContext struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	*deploytest.ResourceMonitor

	resmon       chan *deploytest.ResourceMonitor
	programErr   chan error
	snap         chan *deploy.Snapshot
	updateResult chan result.Result
}

func startUpdate(t *testing.T, host plugin.Host) (*updateContext, error) {
	ctx := &updateContext{
		resmon:       make(chan *deploytest.ResourceMonitor),
		programErr:   make(chan error),
		snap:         make(chan *deploy.Snapshot),
		updateResult: make(chan result.Result),
	}

	stop := make(chan bool)
	port, _, err := rpcutil.Serve(0, stop, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, ctx)
			return nil
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Runtime: "client",
		RuntimeOptions: map[string]interface{}{
			"address": fmt.Sprintf("127.0.0.1:%d", port),
		},
	}

	go func() {
		snap, res := TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
		ctx.snap <- snap
		close(ctx.snap)
		ctx.updateResult <- res
		close(ctx.updateResult)
		stop <- true
	}()

	ctx.ResourceMonitor = <-ctx.resmon
	return ctx, nil
}

func (ctx *updateContext) Finish(err error) (*deploy.Snapshot, result.Result) {
	ctx.programErr <- err
	close(ctx.programErr)

	return <-ctx.snap, <-ctx.updateResult
}

func (ctx *updateContext) GetRequiredPlugins(_ context.Context,
	req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (ctx *updateContext) Run(_ context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	// Connect to the resource monitor and create an appropriate client.
	conn, err := grpc.Dial(
		req.MonitorAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not connect to resource monitor: %w", err)
	}
	defer contract.IgnoreClose(conn)

	// Fire up a resource monitor client
	ctx.resmon <- deploytest.NewResourceMonitor(pulumirpc.NewResourceMonitorClient(conn))
	close(ctx.resmon)

	// Wait for the program to terminate.
	if err := <-ctx.programErr; err != nil {
		return &pulumirpc.RunResponse{Error: err.Error()}, nil
	}
	return &pulumirpc.RunResponse{}, nil
}

func (ctx *updateContext) GetPluginInfo(_ context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: "1.0.0",
	}, nil
}

func (ctx *updateContext) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest,
	server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

func TestLanguageClient(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	update, err := startUpdate(t, deploytest.NewPluginHost(nil, nil, nil, loaders...))
	if err != nil {
		t.Fatalf("failed to start update: %v", err)
	}

	// Register resources, etc.
	_, _, _, err = update.RegisterResource("pkgA:m:typA", "resA", true)
	assert.NoError(t, err)

	snap, res := update.Finish(nil)
	assert.Nil(t, res)
	assert.Len(t, snap.Resources, 2)
}

func TestSingleComponentGetResourceDefaultProviderLifecycle(t *testing.T) {
	t.Parallel()

	var urnB resource.URN
	var idB resource.ID

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(monitor *deploytest.ResourceMonitor, typ, name string, parent resource.URN,
				inputs resource.PropertyMap, options plugin.ConstructOptions,
			) (plugin.ConstructResult, error) {
				urn, _, _, err := monitor.RegisterResource(tokens.Type(typ), name, false, deploytest.ResourceOptions{
					Parent:       parent,
					Protect:      options.Protect,
					Aliases:      options.Aliases,
					Dependencies: options.Dependencies,
				})
				assert.NoError(t, err)

				urnB, idB, _, err = monitor.RegisterResource("pkgA:m:typB", "resB", true, deploytest.ResourceOptions{
					Parent: urn,
					Inputs: resource.PropertyMap{
						"bar": resource.NewStringProperty("baz"),
					},
				})
				assert.NoError(t, err)

				return plugin.ConstructResult{
					URN: urn,
					Outputs: resource.PropertyMap{
						"foo": resource.NewStringProperty("bar"),
						"res": resource.MakeCustomResourceReference(urnB, idB, ""),
					},
				}, nil
			}

			return &deploytest.Provider{
				CreateF: func(urn resource.URN, inputs resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", inputs, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
				ConstructF: construct,
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, state, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
			"res": resource.MakeCustomResourceReference(urnB, idB, ""),
		}, state)

		result, _, err := monitor.Invoke("pulumi:pulumi:getResource", resource.PropertyMap{
			"urn": resource.NewStringProperty(string(urnB)),
		}, "", "")
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"urn": resource.NewStringProperty(string(urnB)),
			"id":  resource.NewStringProperty(string(idB)),
			"state": resource.NewObjectProperty(resource.PropertyMap{
				"bar": resource.NewStringProperty("baz"),
			}),
		}, result)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

func TestConfigSecrets(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	crypter := config.NewSymmetricCrypter(make([]byte, 32))
	secret, err := crypter.EncryptValue(context.Background(), "hunter2")
	assert.NoError(t, err)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 2),
		Config: config.Map{
			config.MustMakeKey("pkgA", "secret"): config.NewSecureValue(secret),
		},
		Decrypter: crypter,
	}

	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	if !assert.Len(t, snap.Resources, 2) {
		return
	}

	provider := snap.Resources[0]
	assert.True(t, provider.Inputs["secret"].IsSecret())
	assert.True(t, provider.Outputs["secret"].IsSecret())
}

func TestComponentOutputs(t *testing.T) {
	t.Parallel()

	// A component's outputs should never be returned by `RegisterResource`, even if (especially if) there are
	// outputs from a prior deployment and the component's inputs have not changed.
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		urn, _, state, err := monitor.RegisterResource("component", "resA", false)
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{}, state)

		err = monitor.RegisterResourceOutputs(urn, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 1),
	}
	p.Run(t, nil)
}

// Test calling a method.
func TestSingleComponentMethodDefaultProviderLifecycle(t *testing.T) {
	t.Parallel()

	var urn resource.URN

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(monitor *deploytest.ResourceMonitor,
				typ, name string, parent resource.URN, inputs resource.PropertyMap,
				options plugin.ConstructOptions,
			) (plugin.ConstructResult, error) {
				var err error
				urn, _, _, err = monitor.RegisterResource(tokens.Type(typ), name, false, deploytest.ResourceOptions{
					Parent:  parent,
					Aliases: options.Aliases,
					Protect: options.Protect,
				})
				assert.NoError(t, err)

				_, _, _, err = monitor.RegisterResource("pkgA:m:typB", "resA", true, deploytest.ResourceOptions{
					Parent: urn,
				})
				assert.NoError(t, err)

				outs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
				err = monitor.RegisterResourceOutputs(urn, outs)
				assert.NoError(t, err)

				return plugin.ConstructResult{
					URN:     urn,
					Outputs: outs,
				}, nil
			}

			call := func(monitor *deploytest.ResourceMonitor, tok tokens.ModuleMember, args resource.PropertyMap,
				info plugin.CallInfo, options plugin.CallOptions,
			) (plugin.CallResult, error) {
				assert.Equal(t, resource.PropertyMap{
					"name": resource.NewStringProperty("Alice"),
				}, args)
				name := args["name"].StringValue()

				result, _, err := monitor.Invoke("pulumi:pulumi:getResource", resource.PropertyMap{
					"urn": resource.NewStringProperty(string(urn)),
				}, "", "")
				assert.NoError(t, err)
				state := result["state"]
				foo := state.ObjectValue()["foo"].StringValue()

				message := fmt.Sprintf("%s, %s!", name, foo)
				return plugin.CallResult{
					Return: resource.PropertyMap{
						"message": resource.NewStringProperty(message),
					},
				}, nil
			}

			return &deploytest.Provider{
				ConstructF: construct,
				CallF:      call,
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, state, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}, state)

		outs, _, _, err := monitor.Call("pkgA:m:typA/methodA", resource.PropertyMap{
			"name": resource.NewStringProperty("Alice"),
		}, "", "")
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"message": resource.NewStringProperty("Alice, bar!"),
		}, outs)

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// Test creating a resource from a method.
func TestSingleComponentMethodResourceDefaultProviderLifecycle(t *testing.T) {
	t.Parallel()

	var urn resource.URN

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(monitor *deploytest.ResourceMonitor,
				typ, name string, parent resource.URN, inputs resource.PropertyMap,
				options plugin.ConstructOptions,
			) (plugin.ConstructResult, error) {
				var err error
				urn, _, _, err = monitor.RegisterResource(tokens.Type(typ), name, false, deploytest.ResourceOptions{
					Parent:  parent,
					Aliases: options.Aliases,
					Protect: options.Protect,
				})
				assert.NoError(t, err)

				_, _, _, err = monitor.RegisterResource("pkgA:m:typB", "resA", true, deploytest.ResourceOptions{
					Parent: urn,
				})
				assert.NoError(t, err)

				outs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
				err = monitor.RegisterResourceOutputs(urn, outs)
				assert.NoError(t, err)

				return plugin.ConstructResult{
					URN:     urn,
					Outputs: outs,
				}, nil
			}

			call := func(monitor *deploytest.ResourceMonitor, tok tokens.ModuleMember, args resource.PropertyMap,
				info plugin.CallInfo, options plugin.CallOptions,
			) (plugin.CallResult, error) {
				_, _, _, err := monitor.RegisterResource("pkgA:m:typC", "resA", true, deploytest.ResourceOptions{
					Parent: urn,
				})
				assert.NoError(t, err)

				return plugin.CallResult{}, nil
			}

			return &deploytest.Provider{
				ConstructF: construct,
				CallF:      call,
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, state, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}, state)

		_, _, _, err = monitor.Call("pkgA:m:typA/methodA", resource.PropertyMap{}, "", "")
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// This tests a scenario involving two remote components with interdependencies that are only represented in the
// user program.
func TestComponentDeleteDependencies(t *testing.T) {
	t.Parallel()

	var (
		firstURN  resource.URN
		nestedURN resource.URN
		sgURN     resource.URN
		secondURN resource.URN
		ruleURN   resource.URN

		err error
	)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(monitor *deploytest.ResourceMonitor, typ, name string, parent resource.URN,
					inputs resource.PropertyMap, options plugin.ConstructOptions,
				) (plugin.ConstructResult, error) {
					switch typ {
					case "pkgB:m:first":
						firstURN, _, _, err = monitor.RegisterResource("pkgB:m:first", name, false)
						require.NoError(t, err)

						nestedURN, _, _, err = monitor.RegisterResource("nested", "nested", false,
							deploytest.ResourceOptions{
								Parent: firstURN,
							})
						require.NoError(t, err)

						sgURN, _, _, err = monitor.RegisterResource("pkgA:m:sg", "sg", true, deploytest.ResourceOptions{
							Parent: nestedURN,
						})
						require.NoError(t, err)

						err = monitor.RegisterResourceOutputs(nestedURN, resource.PropertyMap{})
						require.NoError(t, err)

						err = monitor.RegisterResourceOutputs(firstURN, resource.PropertyMap{})
						require.NoError(t, err)

						return plugin.ConstructResult{URN: firstURN}, nil
					case "pkgB:m:second":
						secondURN, _, _, err = monitor.RegisterResource("pkgB:m:second", name, false,
							deploytest.ResourceOptions{
								Dependencies: options.Dependencies,
							})
						require.NoError(t, err)

						ruleURN, _, _, err = monitor.RegisterResource("pkgA:m:rule", "rule", true,
							deploytest.ResourceOptions{
								Parent:       secondURN,
								Dependencies: options.PropertyDependencies["sgID"],
							})
						require.NoError(t, err)

						err = monitor.RegisterResourceOutputs(secondURN, resource.PropertyMap{})
						require.NoError(t, err)

						return plugin.ConstructResult{URN: secondURN}, nil
					default:
						return plugin.ConstructResult{}, fmt.Errorf("unexpected type %v", typ)
					}
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err = monitor.RegisterResource("pkgB:m:first", "first", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgB:m:second", "second", false, deploytest.ResourceOptions{
			Remote: true,
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"sgID": {sgURN},
			},
			Dependencies: []resource.URN{firstURN},
		})
		require.NoError(t, err)

		return nil
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{Options: UpdateOptions{Host: host}}

	p.Steps = []TestStep{
		{
			Op:          Update,
			SkipPreview: true,
		},
		{
			Op:          Destroy,
			SkipPreview: true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, res result.Result,
			) result.Result {
				assert.Nil(t, res)

				firstIndex, nestedIndex, sgIndex, secondIndex, ruleIndex := -1, -1, -1, -1, -1

				for i, entry := range entries {
					switch urn := entry.Step.URN(); urn {
					case firstURN:
						firstIndex = i
					case nestedURN:
						nestedIndex = i
					case sgURN:
						sgIndex = i
					case secondURN:
						secondIndex = i
					case ruleURN:
						ruleIndex = i
					}
				}

				assert.Less(t, ruleIndex, sgIndex)
				assert.Less(t, ruleIndex, secondIndex)
				assert.Less(t, secondIndex, firstIndex)
				assert.Less(t, secondIndex, sgIndex)
				assert.Less(t, sgIndex, nestedIndex)
				assert.Less(t, nestedIndex, firstIndex)

				return res
			},
		},
	}
	p.Run(t, nil)
}

func TestProtect(t *testing.T) {
	t.Parallel()

	idCounter := 0
	deleteCounter := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(
					urn resource.URN,
					id resource.ID,
					olds, news resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if !olds["foo"].DeepEquals(news["foo"]) {
						// If foo changes do a replace, we use this to check we don't delete on replace
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", idCounter))
					idCounter = idCounter + 1
					return resourceID, news, resource.StatusOK, nil
				},
				DeleteF: func(urn resource.URN, id resource.ID, olds resource.PropertyMap,
					timeout float64,
				) (resource.Status, error) {
					deleteCounter = deleteCounter + 1
					return resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	shouldProtect := true
	createResource := true

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:  ins,
				Protect: shouldProtect,
			})
			assert.NoError(t, err)
		}

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())
	assert.Equal(t, 0, deleteCounter)

	expectedUrn := snap.Resources[1].URN
	expectedMessage := ""

	// Both updates below should give a diagnostic event
	validate := func(project workspace.Project,
		target deploy.Target, entries JournalEntries,
		events []Event, res result.Result,
	) result.Result {
		for _, event := range events {
			if event.Type == DiagEvent {
				payload := event.Payload().(DiagEventPayload)
				assert.Equal(t, expectedUrn, payload.URN)
				assert.Equal(t, expectedMessage, payload.Message)
				break
			}
		}
		return res
	}

	// Run a new update which will cause a replace, we should get an error
	expectedMessage = "<{%reset%}>unable to replace resource \"urn:pulumi:test::test::pkgA:m:typA::resA\"\n" +
		"as it is currently marked for protection. To unprotect the resource, remove the `protect` flag from " +
		"the resource in your Pulumi program and run `pulumi up`<{%reset%}>\n"
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate)
	assert.NotNil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())
	assert.Equal(t, 0, deleteCounter)

	// Run a new update which will cause a delete, we still shouldn't see a provider delete
	expectedMessage = "<{%reset%}>resource \"urn:pulumi:test::test::pkgA:m:typA::resA\" cannot be deleted\n" +
		"because it is protected. To unprotect the resource, either remove the `protect` flag " +
		"from the resource in your Pulumi program and run `pulumi up`, or use the command:\n" +
		"`pulumi state unprotect 'urn:pulumi:test::test::pkgA:m:typA::resA'`<{%reset%}>\n"
	createResource = false
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate)
	assert.NotNil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())
	assert.Equal(t, true, snap.Resources[1].Protect)
	assert.Equal(t, 0, deleteCounter)

	// Run a new update to remove the protect and replace in the same update, this should delete the old one
	// and create the new one
	createResource = true
	shouldProtect = false
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-1", snap.Resources[1].ID.String())
	assert.Equal(t, false, snap.Resources[1].Protect)
	assert.Equal(t, 1, deleteCounter)

	// Run a new update to add the protect flag, nothing else should change
	shouldProtect = true
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-1", snap.Resources[1].ID.String())
	assert.Equal(t, true, snap.Resources[1].Protect)
	assert.Equal(t, 1, deleteCounter)

	// Edit the snapshot to remove the protect flag and try and replace
	snap.Resources[1].Protect = false
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "daz",
	})
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-2", snap.Resources[1].ID.String())
	assert.Equal(t, 2, deleteCounter)
}

func TestRetainOnDelete(t *testing.T) {
	t.Parallel()

	idCounter := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(
					urn resource.URN,
					id resource.ID,
					olds, news resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if !olds["foo"].DeepEquals(news["foo"]) {
						// If foo changes do a replace, we use this to check we don't delete on replace
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", idCounter))
					idCounter = idCounter + 1
					return resourceID, news, resource.StatusOK, nil
				},
				DeleteF: func(urn resource.URN, id resource.ID, olds resource.PropertyMap,
					timeout float64,
				) (resource.Status, error) {
					assert.Fail(t, "Delete was called")
					return resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	createResource := true

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:         ins,
				RetainOnDelete: true,
			})
			assert.NoError(t, err)
		}

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())

	// Run a new update which will cause a replace, we shouldn't see a provider delete but should get a new id
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-1", snap.Resources[1].ID.String())

	// Run a new update which will cause a delete, we still shouldn't see a provider delete
	createResource = false
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 0)
}

func TestDeletedWith(t *testing.T) {
	t.Parallel()

	idCounter := 0

	topURN := resource.URN("")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(
					urn resource.URN,
					id resource.ID,
					olds, news resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if !olds["foo"].DeepEquals(news["foo"]) {
						// If foo changes do a replace, we use this to check we don't delete on replace
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", idCounter))
					idCounter = idCounter + 1
					return resourceID, news, resource.StatusOK, nil
				},
				DeleteF: func(urn resource.URN, id resource.ID, olds resource.PropertyMap,
					timeout float64,
				) (resource.Status, error) {
					if urn != topURN {
						// Only topURN (aURN) should be actually deleted
						assert.Fail(t, "Delete was called")
					}
					return resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	createResource := true

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			aURN, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: ins,
			})
			assert.NoError(t, err)
			topURN = aURN

			bURN, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs:      ins,
				DeletedWith: aURN,
			})
			assert.NoError(t, err)

			_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
				Inputs:      ins,
				DeletedWith: bURN,
			})
			assert.NoError(t, err)
		}

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())
	assert.Equal(t, "created-id-1", snap.Resources[2].ID.String())
	assert.Equal(t, "created-id-2", snap.Resources[3].ID.String())

	// Run a new update which will cause a replace, we should only see a provider delete for aURN but should
	// get a new id for everything
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, "created-id-3", snap.Resources[1].ID.String())
	assert.Equal(t, "created-id-4", snap.Resources[2].ID.String())
	assert.Equal(t, "created-id-5", snap.Resources[3].ID.String())

	// Run a new update which will cause a delete, we still shouldn't see a provider delete for anything but aURN
	createResource = false
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 0)
}

func TestDeletedWithCircularDependency(t *testing.T) {
	// This test should be removed if DeletedWith circular dependency is taken care of.
	// At the mean time, if there is a circular dependency - none shall be deleted.
	t.Parallel()

	idCounter := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(
					urn resource.URN,
					id resource.ID,
					olds, news resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{}, nil
				},
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", idCounter))
					idCounter = idCounter + 1
					return resourceID, news, resource.StatusOK, nil
				},
				DeleteF: func(urn resource.URN, id resource.ID, olds resource.PropertyMap,
					timeout float64,
				) (resource.Status, error) {
					assert.Fail(t, "Delete was called")

					return resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	createResource := true
	cURN := resource.URN("")

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			aURN, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:      ins,
				DeletedWith: cURN,
			})
			assert.NoError(t, err)

			bURN, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs:      ins,
				DeletedWith: aURN,
			})
			assert.NoError(t, err)

			cURN, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
				Inputs:      ins,
				DeletedWith: bURN,
			})
			assert.NoError(t, err)
		}

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())
	assert.Equal(t, "created-id-1", snap.Resources[2].ID.String())
	assert.Equal(t, "created-id-2", snap.Resources[3].ID.String())

	// Run again to update DeleteWith for resA
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())
	assert.Equal(t, "created-id-1", snap.Resources[2].ID.String())
	assert.Equal(t, "created-id-2", snap.Resources[3].ID.String())

	// Run a new update which will cause a delete, we still shouldn't see a provider delete
	createResource = false
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 0)
}

func TestInvalidGetIDReportsUserError(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "", "", resource.PropertyMap{}, "", "")
		assert.Error(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>Expected an ID for urn:pulumi:test::test::pkgA:m:typA::resA<{%reset%}>"))

	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 1)
}

func TestEventSecrets(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					diff := olds.Diff(news)
					if diff == nil {
						return plugin.DiffResult{Changes: plugin.DiffNone}, nil
					}
					detailedDiff := plugin.NewDetailedDiffFromObjectDiff(diff)
					changedKeys := diff.ChangedKeys()

					return plugin.DiffResult{
						Changes:      plugin.DiffSome,
						ChangedKeys:  changedKeys,
						DetailedDiff: detailedDiff,
					}, nil
				},

				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					return news, resource.StatusOK, nil
				},
				CreateF: func(urn resource.URN, inputs resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return resource.ID("id123"), inputs, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	var inputs resource.PropertyMap
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps: []TestStep{{
			Op:          Update,
			SkipPreview: true,
		}},
	}

	inputs = resource.PropertyMap{
		"webhooks": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewObjectProperty(resource.PropertyMap{
				"clientConfig": resource.NewObjectProperty(resource.PropertyMap{
					"service": resource.NewStringProperty("foo"),
				}),
			}),
		})),
	}
	snap := p.Run(t, nil)

	inputs = resource.PropertyMap{
		"webhooks": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewObjectProperty(resource.PropertyMap{
				"clientConfig": resource.NewObjectProperty(resource.PropertyMap{
					"service": resource.NewStringProperty("bar"),
				}),
			}),
		})),
	}
	p.Steps[0].Validate = func(project workspace.Project, target deploy.Target, entries JournalEntries,
		evts []Event, res result.Result,
	) result.Result {
		for _, e := range evts {
			var step StepEventMetadata
			switch e.Type {
			case ResourcePreEvent:
				step = e.Payload().(ResourcePreEventPayload).Metadata
			case ResourceOutputsEvent:
				step = e.Payload().(ResourceOutputsEventPayload).Metadata
			default:
				continue
			}
			if step.URN.Name() != "resA" {
				continue
			}

			assert.True(t, step.Old.Inputs["webhooks"].IsSecret())
			assert.True(t, step.Old.Outputs["webhooks"].IsSecret())
			assert.True(t, step.New.Inputs["webhooks"].IsSecret())
		}
		return res
	}
	p.Run(t, snap)
}

func TestAdditionalSecretOutputs(t *testing.T) {
	t.Parallel()

	t.Skip("AdditionalSecretOutputs warning is currently disabled")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, inputs resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return resource.ID("id123"), inputs, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	var inputs resource.PropertyMap
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:                  inputs,
			AdditionalSecretOutputs: []resource.PropertyKey{"a", "b"},
		})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	inputs = resource.PropertyMap{
		"a": resource.NewStringProperty("testA"),
		// b is missing
		"c": resource.MakeSecret(resource.NewStringProperty("testC")),
	}

	// Run an update to create the resource and check we warn about b
	validate := func(
		project workspace.Project, target deploy.Target,
		entries JournalEntries, events []Event,
		res result.Result,
	) result.Result {
		if res != nil {
			return res
		}

		for i := range events {
			if events[i].Type == "diag" {
				payload := events[i].Payload().(engine.DiagEventPayload)
				if payload.Severity == "warning" &&
					payload.URN == "urn:pulumi:test::test::pkgA:m:typA::resA" &&
					payload.Message == "<{%reset%}>Could not find property 'b' listed in additional secret outputs.<{%reset%}>\n" {
					// Found the message we expected
					return nil
				}
			}
		}
		return result.Error("Expected a diagnostic message, got none")
	}
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	assert.Nil(t, res)

	// Should have the provider and resA
	assert.Len(t, snap.Resources, 2)
	resA := snap.Resources[1]
	assert.Equal(t, []resource.PropertyKey{"a", "b"}, resA.AdditionalSecretOutputs)
	assert.True(t, resA.Outputs["a"].IsSecret())
	assert.True(t, resA.Outputs["c"].IsSecret())
}

func TestDefaultParents(t *testing.T) {
	t.Parallel()
	t.Skipf("Default parents disabled due to https://github.com/pulumi/pulumi/issues/10950")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource(
			resource.RootStackType,
			info.Project+"-"+info.Stack,
			false,
			deploytest.ResourceOptions{})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource(
			"pkgA:m:typA",
			"resA",
			true,
			deploytest.ResourceOptions{})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)

	// Assert that resource 0 is the stack
	assert.Equal(t, resource.RootStackType, snap.Resources[0].Type)
	// Assert that the other 2 resources have the stack as a parent
	assert.Equal(t, snap.Resources[0].URN, snap.Resources[1].Parent)
	assert.Equal(t, snap.Resources[0].URN, snap.Resources[2].Parent)
}

func TestPendingDeleteOrder(t *testing.T) {
	// Test for https://github.com/pulumi/pulumi/issues/2948 Ensure that if we have resources A and B, and we
	// go to replace A but then fail to replace B that we correctly handle everything in the same order when
	// we retry the update.
	//
	// That is normally for this operation we would do the following:
	// 1. Create new A
	// 2. Create new B
	// 3. Delete old B
	// 4. Delete old A
	// So if step 2 fails to create the new B we want to see:
	// 1. Create new A
	// 2. Create new B (fail)
	// 1. Create new B
	// 2. Delete old B
	// 3. Delete old A
	// Currently (and what #2948 tracks) is that the engine does the following:
	// 1. Create new A
	// 2. Create new B (fail)
	// 3. Delete old A
	// 1. Create new B
	// 2. Delete old B
	// That delete A fails because the delete B needs to happen first.

	t.Parallel()

	cloudState := map[resource.ID]resource.PropertyMap{}

	failCreationOfTypB := false

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					if strings.Contains(string(urn), "typB") && failCreationOfTypB {
						return "", nil, resource.StatusOK, fmt.Errorf("Could not create typB")
					}

					id := resource.ID(fmt.Sprintf("%d", len(cloudState)))
					if !preview {
						cloudState[id] = news
					}
					return id, news, resource.StatusOK, nil
				},
				DeleteF: func(urn resource.URN,
					id resource.ID, olds resource.PropertyMap, timeout float64,
				) (resource.Status, error) {
					// Fail if anything in cloud state still points to us
					for other, res := range cloudState {
						for _, v := range res {
							if v.IsString() && v.StringValue() == string(id) {
								return resource.StatusOK, fmt.Errorf("Can not delete %s used by %s", id, other)
							}
						}
					}

					delete(cloudState, id)
					return resource.StatusOK, nil
				},
				DiffF: func(urn resource.URN,
					id resource.ID, olds, news resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if strings.Contains(string(urn), "typA") {
						if !olds["foo"].DeepEquals(news["foo"]) {
							return plugin.DiffResult{
								Changes:     plugin.DiffSome,
								ReplaceKeys: []resource.PropertyKey{"foo"},
								DetailedDiff: map[string]plugin.PropertyDiff{
									"foo": {
										Kind:      plugin.DiffUpdateReplace,
										InputDiff: true,
									},
								},
								DeleteBeforeReplace: false,
							}, nil
						}
					}
					if strings.Contains(string(urn), "typB") {
						if !olds["parent"].DeepEquals(news["parent"]) {
							return plugin.DiffResult{
								Changes:     plugin.DiffSome,
								ReplaceKeys: []resource.PropertyKey{"parent"},
								DetailedDiff: map[string]plugin.PropertyDiff{
									"parent": {
										Kind:      plugin.DiffUpdateReplace,
										InputDiff: true,
									},
								},
								DeleteBeforeReplace: false,
							}, nil
						}
					}

					return plugin.DiffResult{}, nil
				},
				UpdateF: func(urn resource.URN,
					id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					assert.Fail(t, "Didn't expect update to be called")
					return nil, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		urnA, idA, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typB", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"parent": idA,
			}),
			Dependencies: []resource.URN{urnA},
		})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run an update to create the resources
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)

	// Trigger a replacement of A but fail to create B
	failCreationOfTypB = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	// Assert that this fails, we should have two copies of A now, one new one and one old one pending delete
	assert.NotNil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, snap.Resources[1].Type, tokens.Type("pkgA:m:typA"))
	assert.False(t, snap.Resources[1].Delete)
	assert.Equal(t, snap.Resources[2].Type, tokens.Type("pkgA:m:typA"))
	assert.True(t, snap.Resources[2].Delete)

	// Now allow B to create and try again
	failCreationOfTypB = false
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
}

func TestDuplicatesDueToAliases(t *testing.T) {
	t.Parallel()

	// This is a test for https://github.com/pulumi/pulumi/issues/11173
	// to check that we don't allow resource aliases to refer to other resources.
	// That is if you have A, then try and add B saying it's alias is A we should error that's a duplicate.
	// We need to be careful that we handle this regardless of the order we send the RegisterResource requests for A and B.

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	mode := 0
	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		switch mode {
		case 0:
			// Default case, just make resA
			_, _, _, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

		case 1:
			// First test case, try and create a new B that aliases to A. First make the A like normal...
			_, _, _, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

			// ... then make B with an alias, it should error
			_, _, _, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{
					Aliases: []resource.Alias{{Name: "resA"}},
				})
			assert.Error(t, err)

		case 2:
			// Second test case, try and create a new B that aliases to A. First make the B with an alias...
			_, _, _, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{
					Aliases: []resource.Alias{{Name: "resA"}},
				})
			assert.NoError(t, err)

			// ... then try to make the A like normal. It should error that it's already been aliased away
			_, _, _, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.Error(t, err)
		}
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run an update to create the starting A resource
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)

	// Set mode to try and create A then a B that aliases to it, this should fail
	mode = 1
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)

	// Set mode to try and create B first then a A, this should fail
	mode = 2
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	// Because we made the B first that's what should end up in the state file
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resB"), snap.Resources[1].URN)
}

func TestPendingDeleteReplacement(t *testing.T) {
	// Test for https://github.com/pulumi/pulumi/issues/11391, check that if we
	// try to replace a resource via delete before replace, but fail to delete
	// it, then rerun that we don't error.

	t.Parallel()

	cloudID := 0
	cloudState := map[resource.ID]resource.PropertyMap{}

	failDeletionOfTypB := true

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					id := resource.ID("")
					if !preview {
						id = resource.ID(fmt.Sprintf("%d", cloudID))
						cloudID = cloudID + 1
						cloudState[id] = news
					}
					return id, news, resource.StatusOK, nil
				},
				DeleteF: func(urn resource.URN,
					id resource.ID, olds resource.PropertyMap, timeout float64,
				) (resource.Status, error) {
					// Fail if anything in cloud state still points to us
					for _, res := range cloudState {
						for _, v := range res {
							if v.IsString() && v.StringValue() == string(id) {
								return resource.StatusOK, fmt.Errorf("Can not delete %s", id)
							}
						}
					}

					if strings.Contains(string(urn), "typB") && failDeletionOfTypB {
						return resource.StatusOK, fmt.Errorf("Could not delete typB")
					}

					delete(cloudState, id)
					return resource.StatusOK, nil
				},
				DiffF: func(urn resource.URN,
					id resource.ID, olds, news resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if strings.Contains(string(urn), "typA") {
						if !olds["foo"].DeepEquals(news["foo"]) {
							return plugin.DiffResult{
								Changes:     plugin.DiffSome,
								ReplaceKeys: []resource.PropertyKey{"foo"},
								DetailedDiff: map[string]plugin.PropertyDiff{
									"foo": {
										Kind:      plugin.DiffUpdateReplace,
										InputDiff: true,
									},
								},
								DeleteBeforeReplace: true,
							}, nil
						}
					}
					if strings.Contains(string(urn), "typB") {
						if !olds["parent"].DeepEquals(news["parent"]) {
							return plugin.DiffResult{
								Changes:     plugin.DiffSome,
								ReplaceKeys: []resource.PropertyKey{"parent"},
								DetailedDiff: map[string]plugin.PropertyDiff{
									"parent": {
										Kind:      plugin.DiffUpdateReplace,
										InputDiff: true,
									},
								},
								DeleteBeforeReplace: false,
							}, nil
						}
						if !olds["frob"].DeepEquals(news["frob"]) {
							return plugin.DiffResult{
								Changes:     plugin.DiffSome,
								ReplaceKeys: []resource.PropertyKey{"frob"},
								DetailedDiff: map[string]plugin.PropertyDiff{
									"frob": {
										Kind:      plugin.DiffUpdateReplace,
										InputDiff: true,
									},
								},
								DeleteBeforeReplace: false,
							}, nil
						}
					}

					return plugin.DiffResult{}, nil
				},
				UpdateF: func(urn resource.URN,
					id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					assert.Fail(t, "Didn't expect update to be called")
					return nil, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	insA := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	inB := "active"
	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		urnA, idA, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: insA,
		})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typB", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"parent": idA,
				"frob":   inB,
			}),
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"parent": {urnA},
			},
			Dependencies: []resource.URN{urnA},
		})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run an update to create the resources
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)

	// Trigger a replacement of B but fail to delete it
	inB = "inactive"
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	// Assert that this fails, we should have two B's one marked to delete
	assert.NotNil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, snap.Resources[1].Type, tokens.Type("pkgA:m:typA"))
	assert.False(t, snap.Resources[1].Delete)
	assert.Equal(t, snap.Resources[2].Type, tokens.Type("pkgA:m:typB"))
	assert.False(t, snap.Resources[2].Delete)
	assert.Equal(t, snap.Resources[3].Type, tokens.Type("pkgA:m:typB"))
	assert.True(t, snap.Resources[3].Delete)

	// Now trigger a replacment of A, which will also trigger B to replace
	insA = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	failDeletionOfTypB = false
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	// Assert this is ok, we should have just one A and B
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, snap.Resources[1].Type, tokens.Type("pkgA:m:typA"))
	assert.False(t, snap.Resources[1].Delete)
	assert.Equal(t, snap.Resources[2].Type, tokens.Type("pkgA:m:typB"))
	assert.False(t, snap.Resources[2].Delete)
}

func TestTimestampTracking(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{Changes: plugin.DiffSome}, nil
				},
				UpdateF: func(_ resource.URN, _ resource.ID, _, _ resource.PropertyMap, _ float64,
					_ []string, _ bool,
				) (resource.PropertyMap, resource.Status, error) {
					outputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"foo": "bar",
					})
					return outputs, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource(
			resource.RootStackType,
			info.Project+"-"+info.Stack,
			false,
			deploytest.ResourceOptions{})
		require.NoError(t, err)

		_, _, _, err = monitor.RegisterResource(
			"pkgA:m:typA",
			"resA",
			true,
			deploytest.ResourceOptions{})
		require.NoError(t, err)

		return nil
	})

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)

	// Run an update to create the resource -- created and updated should be set and equal.
	p.Steps = []TestStep{{Op: Update, SkipPreview: true}}
	snap := p.Run(t, nil)
	require.NotEmpty(t, snap.Resources)

	creationTimes := make(map[resource.URN]time.Time, len(snap.Resources))
	for _, resource := range snap.Resources {
		assert.NotNil(t, resource.Created, "missing created time: %v", resource.URN)
		assert.NotNil(t, resource.Modified, "missing modified time: %v", resource.URN)
		tz, _ := resource.Created.Zone()
		assert.Equal(t, "UTC", tz, "time zone is not UTC: %v", resource.URN)
		assert.Equal(t, resource.Created, resource.Modified,
			"created time != modified time: %v", resource.URN)

		creationTimes[resource.URN] = *resource.Created
	}

	// Run a refresh -- created and updated should be unchanged.
	p.Steps = []TestStep{{Op: Refresh, SkipPreview: true}}
	snap = p.Run(t, snap)
	require.NotEmpty(t, snap.Resources)
	for _, resource := range snap.Resources {
		assert.NotNil(t, resource.Created, "missing created time: %v", resource.URN)
		assert.NotNil(t, resource.Modified, "missing modified time: %v", resource.URN)
		assert.Equal(t, *resource.Created, creationTimes[resource.URN],
			"created time changed: %v", resource.URN)
		assert.Equal(t, resource.Created, resource.Modified,
			"modified time changed: %v", resource.URN)
	}

	// Run another update -- updated should be greater than created for resA,
	// everything else should be untouched.
	p.Steps = []TestStep{{Op: Update, SkipPreview: true}}
	snap = p.Run(t, snap)
	require.NotEmpty(t, snap.Resources)
	for _, resource := range snap.Resources {
		assert.NotNil(t, resource.Created, resource.URN, "missing created time: %v", resource.URN)
		assert.NotNil(t, resource.Modified, resource.URN, "missing modified time: %v", resource.URN)
		assert.Equal(t, creationTimes[resource.URN], *resource.Created,
			"created time changed: %v", resource.URN)

		switch resource.Type {
		case "pkgA:m:typA":
			tz, _ := resource.Modified.Zone()
			assert.Equal(t, "UTC", tz, "time zone is not UTC: %v", resource.URN)
			assert.NotEqual(t, creationTimes[resource.URN], *resource.Modified,
				"modified time did not update: %v", resource.URN)
			assert.Greater(t, *resource.Modified, *resource.Created,
				"modified time is too old: %v", resource.URN)
		case "pulumi:providers:pkgA", "pulumi:pulumi:Stack":
			tz, _ := resource.Modified.Zone()
			assert.Equal(t, "UTC", tz, "time zone is not UTC: %v", resource.URN)
			assert.NotNil(t, *resource.Created, "missing created time: %v", resource.URN)
			assert.NotNil(t, *resource.Modified, "missing modified time: %v", resource.URN)
		default:
			require.FailNow(t, "unrecognized resource type", resource.Type)
		}
	}
}

func TestComponentToCustomUpdate(t *testing.T) {
	// Test for https://github.com/pulumi/pulumi/issues/12550, check that if we change a component resource
	// into a custom resource the engine handles that best it can. This depends on the provider being able to
	// cope with the component state being passed as custom state.

	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					id := resource.ID("")
					if !preview {
						id = resource.ID("1")
					}
					return id, news, resource.StatusOK, nil
				},
				DeleteF: func(urn resource.URN,
					id resource.ID, olds resource.PropertyMap, timeout float64,
				) (resource.Status, error) {
					return resource.StatusOK, nil
				},
				DiffF: func(urn resource.URN,
					id resource.ID, olds, news resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{}, nil
				},
				UpdateF: func(urn resource.URN,
					id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					return news, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	insA := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	createA := func(monitor *deploytest.ResourceMonitor) {
		_, _, _, err := monitor.RegisterResource("prog::myType", "resA", false, deploytest.ResourceOptions{
			Inputs: insA,
		})
		assert.NoError(t, err)
	}

	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		createA(monitor)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()

	// Run an update to create the resources
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 1)
	assert.Equal(t, tokens.Type("prog::myType"), snap.Resources[0].Type)
	assert.False(t, snap.Resources[0].Custom)

	// Now update A from a component to custom with an alias
	createA = func(monitor *deploytest.ResourceMonitor) {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: insA,
			Aliases: []resource.Alias{
				{
					Type: "prog::myType",
				},
			},
		})
		assert.NoError(t, err)
	}
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	// Assert that A is now a custom
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	// Now two because we'll have a provider now
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, tokens.Type("pkgA:m:typA"), snap.Resources[1].Type)
	assert.True(t, snap.Resources[1].Custom)

	// Now update A back to a component (with an alias)
	createA = func(monitor *deploytest.ResourceMonitor) {
		_, _, _, err := monitor.RegisterResource("prog::myType", "resA", false, deploytest.ResourceOptions{
			Inputs: insA,
			Aliases: []resource.Alias{
				{
					Type: "pkgA:m:typA",
				},
			},
		})
		assert.NoError(t, err)
	}
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	// Assert that A is now a custom
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	// Back to one because the provider should have been cleaned up as well
	assert.Len(t, snap.Resources, 1)
	assert.Equal(t, tokens.Type("prog::myType"), snap.Resources[0].Type)
	assert.False(t, snap.Resources[0].Custom)
}
