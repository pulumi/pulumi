// Copyright 2016-2025, Pulumi Corporation.
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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blang/semver"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
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

func AssertSameStepsUnordered(t *testing.T, expected []StepSummary, actual []deploy.Step) {
	require.Equal(t, len(expected), len(actual))
	for _, exp := range expected {
		found := false
		for _, act := range actual {
			if exp.Op == act.Op() && exp.URN == act.URN() {
				found = true
				break
			}
		}
		require.True(t, found, "Expected step %v not found in actual steps.  Actual steps: %v", exp, actual)
	}
}

func ExpectDiagMessage(t *testing.T, messagePattern string) lt.ValidateFunc {
	validate := func(
		project workspace.Project, target deploy.Target,
		entries JournalEntries, events []Event,
		err error,
	) error {
		assert.Error(t, err)

		for i := range events {
			if events[i].Type == "diag" {
				payload := events[i].Payload().(engine.DiagEventPayload)
				match, err := regexp.MatchString(messagePattern, payload.Message)
				if err != nil {
					return err
				}
				if match {
					return nil
				}
				return fmt.Errorf("Unexpected diag message: %s", payload.Message)
			}
		}
		return errors.New("Expected a diagnostic message, got none")
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
	if (runtime.GOOS == "windows" || runtime.GOOS == "darwin") && os.Getenv("PULUMI_FORCE_RUN_TESTS") == "" {
		// These tests are skipped as part of enabling running unit tests on windows and MacOS in
		// https://github.com/pulumi/pulumi/pull/19653. These tests currently fail on Windows, and
		// re-enabling them is left as future work.
		// TODO[pulumi/pulumi#19675]: Re-enable tests on windows and MacOS once they are fixed.
		fmt.Println("Skip tests on windows and MacOS until they are fixed")
		os.Exit(0)
	}

	flag.Parse()

	if *grpcDefault {
		deploytest.UseGrpcPluginsByDefault = true
	}

	os.Exit(m.Run())
}

func TestEmptyProgramLifecycle(t *testing.T) {
	t.Parallel()

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps:   lt.MakeBasicLifecycleSteps(t, 0),
	}
	p.Run(t, nil)
}

func TestSingleResourceDiffUnavailable(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error) {
					return plugin.DiffResult{}, plugin.DiffUnavailable("diff unavailable")
				},
			}, nil
		}),
	}

	inputs := resource.PropertyMap{}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update.
	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	// Now run a preview. Expect a warning because the diff is unavailable.
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, err error,
		) error {
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
			return err
		})
	assert.NoError(t, err)
}

// Test that ensures that we log diagnostics for resources that receive an error from Check. (Note that this
// is distinct from receiving non-error failures from Check.)
func TestCheckFailureRecord(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(
					_ context.Context,
					req plugin.CheckRequest,
				) (plugin.CheckResponse, error) {
					return plugin.CheckResponse{}, errors.New("oh no, check had an error")
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)
		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps: []lt.TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, err error,
			) error {
				sawFailure := false
				for _, evt := range evts {
					if evt.Type == DiagEvent {
						e := evt.Payload().(DiagEventPayload)
						msg := colors.Never.Colorize(e.Message)
						sawFailure = msg == "oh no, check had an error\n" && e.Severity == diag.Error
					}
				}

				assert.True(t, sawFailure)
				return err
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
				CheckF: func(
					_ context.Context,
					req plugin.CheckRequest,
				) (plugin.CheckResponse, error) {
					return plugin.CheckResponse{
						Failures: []plugin.CheckFailure{{
							Property: "someprop",
							Reason:   "field is not valid",
						}},
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)
		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps: []lt.TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, err error,
			) error {
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
				return err
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
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		// Exiting immediately with an error simulates a language exiting immediately with a non-zero exit code.
		return errors.New(errorText)
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps: []lt.TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, err error,
			) error {
				assert.Error(t, err)
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
				return err
			},
		}},
	}

	p.Run(t, nil)
}

type brokenDecrypter struct {
	ErrorMessage string
}

func (b brokenDecrypter) DecryptValue(_ context.Context, _ string) (string, error) {
	return "", errors.New(b.ErrorMessage)
}

func (b brokenDecrypter) BatchDecrypt(_ context.Context, _ []string) ([]string, error) {
	return nil, errors.New(b.ErrorMessage)
}

// Tests that the engine presents a reasonable error message when a decrypter fails to decrypt a config value.
func TestBrokenDecrypter(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	key := config.MustMakeKey("foo", "bar")
	msg := "decryption failed"
	configMap := make(config.Map)
	configMap[key] = config.NewSecureValue("hunter2")
	p := &lt.TestPlan{
		Options:   lt.TestUpdateOptions{T: t, HostF: hostF},
		Decrypter: brokenDecrypter{ErrorMessage: msg},
		Config:    configMap,
		Steps: []lt.TestStep{{
			Op:            Update,
			ExpectFailure: true,
			SkipPreview:   true,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, err error,
			) error {
				assert.Error(t, err)
				decryptErr := err.(DecryptError)
				assert.Equal(t, key, decryptErr.Key)
				assert.ErrorContains(t, decryptErr.Err, msg)
				return err
			},
		}},
	}

	p.Run(t, nil)
}

func TestConfigPropertyMapMatches(t *testing.T) {
	t.Parallel()

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// Check that the config property map matches what we expect.
		assert.Equal(t, 8, len(info.Config))
		assert.Equal(t, 8, len(info.ConfigPropertyMap))

		assert.Equal(t, "hunter2", info.Config[config.MustMakeKey("pkgA", "secret")])
		assert.True(t, info.ConfigPropertyMap["pkgA:secret"].IsSecret())
		assert.Equal(t, "hunter2", info.ConfigPropertyMap["pkgA:secret"].SecretValue().Element.StringValue())

		assert.Equal(t, "all I see is ******", info.Config[config.MustMakeKey("pkgA", "plain")])
		assert.False(t, info.ConfigPropertyMap["pkgA:plain"].IsSecret())
		assert.Equal(t, "all I see is ******", info.ConfigPropertyMap["pkgA:plain"].StringValue())

		assert.Equal(t, "1234", info.Config[config.MustMakeKey("pkgA", "int")])
		assert.Equal(t, 1234.0, info.ConfigPropertyMap["pkgA:int"].NumberValue())

		assert.Equal(t, "12.34", info.Config[config.MustMakeKey("pkgA", "float")])
		// This is a string because adjustObjectValue only parses integers, not floats.
		assert.Equal(t, "12.34", info.ConfigPropertyMap["pkgA:float"].StringValue())

		assert.Equal(t, "012345", info.Config[config.MustMakeKey("pkgA", "string")])
		assert.Equal(t, "012345", info.ConfigPropertyMap["pkgA:string"].StringValue())

		assert.Equal(t, "true", info.Config[config.MustMakeKey("pkgA", "bool")])
		assert.Equal(t, true, info.ConfigPropertyMap["pkgA:bool"].BoolValue())

		assert.Equal(t, "[1,2,3]", info.Config[config.MustMakeKey("pkgA", "array")])
		assert.Equal(t, 1.0, info.ConfigPropertyMap["pkgA:array"].ArrayValue()[0].NumberValue())
		assert.Equal(t, 2.0, info.ConfigPropertyMap["pkgA:array"].ArrayValue()[1].NumberValue())
		assert.Equal(t, 3.0, info.ConfigPropertyMap["pkgA:array"].ArrayValue()[2].NumberValue())

		assert.Equal(t, `{"bar":"02","foo":1}`, info.Config[config.MustMakeKey("pkgA", "map")])
		assert.Equal(t, 1.0, info.ConfigPropertyMap["pkgA:map"].ObjectValue()["foo"].NumberValue())
		assert.Equal(t, "02", info.ConfigPropertyMap["pkgA:map"].ObjectValue()["bar"].StringValue())
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF)

	crypter := config.NewSymmetricCrypter(make([]byte, 32))
	secret, err := crypter.EncryptValue(context.Background(), "hunter2")
	assert.NoError(t, err)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps:   lt.MakeBasicLifecycleSteps(t, 0),
		Config: config.Map{
			config.MustMakeKey("pkgA", "secret"): config.NewSecureValue(secret),
			config.MustMakeKey("pkgA", "plain"):  config.NewValue("all I see is ******"),
			config.MustMakeKey("pkgA", "int"):    config.NewValue("1234"),
			config.MustMakeKey("pkgA", "float"):  config.NewValue("12.34"),
			config.MustMakeKey("pkgA", "string"): config.NewValue("012345"),
			config.MustMakeKey("pkgA", "bool"):   config.NewValue("true"),
			config.MustMakeKey("pkgA", "array"):  config.NewObjectValue("[1, 2, 3]"),
			config.MustMakeKey("pkgA", "map"):    config.NewObjectValue(`{"foo": 1, "bar": "02"}`),
		},
		Decrypter: crypter,
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, err := mon.RegisterResource("very:bad", "resA", true)
		assert.Error(t, err)
		rpcerr, ok := rpcerror.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, rpcerr.Code())
		assert.Contains(t, rpcerr.Message(), "Type 'very:bad' is not a valid type token")

		_, _, err = mon.ReadResource("very:bad", "someResource", "someId", "", resource.PropertyMap{}, "", "", "", "")
		assert.Error(t, err)
		rpcerr, ok = rpcerror.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, rpcerr.Code())
		assert.Contains(t, rpcerr.Message(), "Type 'very:bad' is not a valid type token")

		// Component resources may have any format type.
		_, noErr := mon.RegisterResource("a:component", "resB", false)
		assert.NoError(t, noErr)

		_, noErr = mon.RegisterResource("singlename", "resC", false)
		assert.NoError(t, noErr)

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps: []lt.TestStep{{
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
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					// Inform the waiter that we've entered a provider op and wait for cancellation.
					ops.Done()
					<-provCtx.Done()

					return plugin.CreateResponse{
						ID:         resource.ID(req.URN.Name()),
						Properties: resource.PropertyMap{},
						Status:     resource.StatusOK,
					}, nil
				},
				CancelF: func() error {
					provCancel()
					return nil
				},
			}, nil
		}),
	}

	done := make(chan bool)
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		errors := make([]error, resourceCount)
		var resources sync.WaitGroup
		resources.Add(resourceCount)
		for i := 0; i < resourceCount; i++ {
			go func(idx int) {
				_, errors[idx] = monitor.RegisterResource("pkgA:m:typA", fmt.Sprintf("res%d", idx), true)
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

	p := &lt.TestPlan{}
	op := lt.TestOp(Update)
	options := lt.TestUpdateOptions{
		T: t,

		HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...),
		UpdateOptions: UpdateOptions{
			Parallel: resourceCount,
		},
	}
	project, target := p.GetProject(), p.GetTarget(t, nil)

	_, err := op.RunWithContext(ctx, project, target, options, false, nil, nil)
	assert.Error(t, err)

	// Wait for the program to finish.
	<-done
}

// Tests that a preview works for a stack with pending operations.
func TestPreviewWithPendingOperations(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}

	const resType = "pkgA:m:typA"
	urnA := p.NewURN(resType, "resA", "")

	newResource := func(urn resource.URN, id resource.ID, del bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       del,
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})

	op := lt.TestOp(Update)

	options := lt.TestUpdateOptions{T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...)}
	project, target := p.GetProject(), p.GetTarget(t, old)

	// A preview should succeed despite the pending operations.
	_, err := op.Run(project, target, options, true, nil, nil)
	assert.NoError(t, err)
}

// Tests that a refresh works for a stack with pending operations.
func TestRefreshWithPendingOperations(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}

	const resType = "pkgA:m:typA"
	urnA := p.NewURN(resType, "resA", "")

	newResource := func(urn resource.URN, id resource.ID, del bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       del,
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})

	op := lt.TestOp(Update)
	options := lt.TestUpdateOptions{T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...)}
	project, target := p.GetProject(), p.GetTarget(t, old)

	// With a refresh, the update should succeed.
	withRefresh := options
	withRefresh.Refresh = true
	new, err := op.RunStep(project, target, withRefresh, false, nil, nil, "0")
	assert.NoError(t, err)
	assert.Len(t, new.PendingOperations, 0)

	// Similarly, the update should succeed if performed after a separate refresh.
	new, err = lt.TestOp(Refresh).RunStep(project, target, options, false, nil, nil, "1")
	assert.NoError(t, err)
	assert.Len(t, new.PendingOperations, 0)

	_, err = op.RunStep(project, p.GetTarget(t, new), options, false, nil, nil, "2")
	assert.NoError(t, err)
}

// Test to make sure that if we pulumi refresh
// while having pending CREATE operations,
// that these are preserved after the refresh.
func TestRefreshPreservesPendingCreateOperations(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}

	const resType = "pkgA:m:typA"
	urnA := p.NewURN(resType, "resA", "")
	urnB := p.NewURN(resType, "resB", "")

	newResource := func(urn resource.URN, id resource.ID, del bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       del,
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})

	op := lt.TestOp(Update)
	options := lt.TestUpdateOptions{T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...)}
	project, target := p.GetProject(), p.GetTarget(t, old)

	// With a refresh, the update should succeed.
	withRefresh := options
	withRefresh.Refresh = true
	new, err := op.Run(project, target, withRefresh, false, nil, nil)
	assert.NoError(t, err)
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

	p := &lt.TestPlan{}

	const resType = "pkgA:m:typA"
	urnA := p.NewURN(resType, "resA", "")
	urnB := p.NewURN(resType, "resB", "")

	newResource := func(urn resource.URN, id resource.ID, del bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       del,
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})

	op := lt.TestOp(Update)
	options := lt.TestUpdateOptions{T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...)}
	project, target := p.GetProject(), p.GetTarget(t, old)

	// The update should succeed but give a warning
	initialPartOfMessage := "Attempting to deploy or update resources with 1 pending operations from previous deployment."
	validate := func(
		project workspace.Project, target deploy.Target,
		entries JournalEntries, events []Event,
		err error,
	) error {
		for i := range events {
			if events[i].Type == "diag" {
				payload := events[i].Payload().(engine.DiagEventPayload)

				if payload.Severity == "warning" && strings.Contains(payload.Message, initialPartOfMessage) {
					return nil
				}
				return fmt.Errorf("Unexpected warning diag message: %s", payload.Message)
			}
		}
		return errors.New("Expected a diagnostic message, got none")
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
				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error) {
					return plugin.DiffResult{Changes: plugin.DiffSome}, nil
				},

				UpdateF: func(context.Context, plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					outputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"output_prop": 42,
					})

					return plugin.UpdateResponse{
						Properties: outputs,
						Status:     resource.StatusPartialFailure,
					}, errors.New("update failed to apply")
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, err := mon.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"input_prop": "new inputs",
			}),
		})
		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")
	p.Steps = []lt.TestStep{{
		Op:            Update,
		ExpectFailure: true,
		SkipPreview:   true,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.Error(t, err)
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case resURN:
					assert.Equal(t, deploy.OpUpdate, entry.Step.Op())
					//nolint:exhaustive // default case signifies testing failure
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

			return err
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
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, state, err := mon.ReadResource("pulumi:pulumi:StackReference", "other", "other", "",
			resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "other",
			}), "", "", "", "")
		assert.NoError(t, err)
		if !info.DryRun {
			assert.Equal(t, "bar", state["outputs"].ObjectValue()["foo"].StringValue())
		}
		return nil
	})
	p := &lt.TestPlan{
		BackendClient: &deploytest.BackendClient{
			GetStackOutputsF: func(ctx context.Context, name string, _ func(error) error) (resource.PropertyMap, error) {
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
		Options: lt.TestUpdateOptions{T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...)},
		Steps:   lt.MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)

	// Test that changes to `name` cause replacement.
	resURN := p.NewURN("pulumi:pulumi:StackReference", "other", "")
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:     resURN.Type(),
				URN:      resURN,
				Custom:   true,
				ID:       "1",
				External: true,
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
	p.Steps = []lt.TestStep{{
		Op:          Update,
		SkipPreview: true,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)
			for _, entry := range entries {
				switch urn := entry.Step.URN(); urn {
				case resURN:
					switch entry.Step.Op() {
					case deploy.OpRead:
						// OK
					default:
						t.Fatalf("unexpected journal operation: %v", entry.Step.Op())
					}
				}
			}

			return err
		},
	}}
	p.Run(t, old)

	// Test that unknown stacks are handled appropriately.
	programF = deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, err := mon.ReadResource("pulumi:pulumi:StackReference", "other", "other", "",
			resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "rehto",
			}), "", "", "", "")
		assert.Error(t, err)
		return err
	})
	p.Options = lt.TestUpdateOptions{T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...)}
	p.Steps = []lt.TestStep{{
		Op:            Update,
		ExpectFailure: true,
		SkipPreview:   true,
	}}
	p.Run(t, nil)

	// Test that unknown properties cause errors.
	programF = deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, _, err := mon.ReadResource("pulumi:pulumi:StackReference", "other", "other", "",
			resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "other",
				"foo":  "bar",
			}), "", "", "", "")
		assert.Error(t, err)
		return err
	})
	p.Options = lt.TestUpdateOptions{T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...)}
	p.Run(t, nil)
}

// Tests that registering (rather than reading) a StackReference resource works as intended, but warns the user that
// it's deprecated.
func TestStackReferenceRegister(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{}

	// Test that the normal lifecycle works correctly.
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		resp, err := mon.RegisterResource("pulumi:pulumi:StackReference", "other", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "other",
			}),
		})
		assert.NoError(t, err)
		if !info.DryRun {
			assert.Equal(t, "bar", resp.Outputs["outputs"].ObjectValue()["foo"].StringValue())
		}
		return nil
	})

	steps := lt.MakeBasicLifecycleSteps(t, 2)
	// Add an extra validate stage to each step to check we get the diagnostic that this use of stack reference is
	// obsolete if a stack resource was registered.
	for i := range steps {
		v := steps[i].Validate
		steps[i].Validate = func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			// Check if we registered a stack reference resource (i.e. same/update/create). Ideally we'd warn on refresh
			// as well but that's just a Read so it's hard to tell in the built-in provider if that's a Read for a
			// ReadResource or a Read for a refresh, so we don't worry about that case.
			registered := false
			for _, entry := range entries {
				if entry.Step.URN().Type() == "pulumi:pulumi:StackReference" &&
					(entry.Step.Op() == deploy.OpCreate ||
						entry.Step.Op() == deploy.OpUpdate ||
						entry.Step.Op() == deploy.OpSame) {
					registered = true
				}
			}

			if registered {
				found := false
				for _, evt := range evts {
					if evt.Type == DiagEvent {
						payload := evt.Payload().(DiagEventPayload)

						ok := payload.Severity == "warning" &&
							payload.URN.Type() == "pulumi:pulumi:StackReference" &&
							strings.Contains(
								payload.Message,
								"The \"pulumi:pulumi:StackReference\" resource type is deprecated.")
						found = found || ok
					}
				}
				assert.True(t, found, "diagnostic warning not found in: %+v", evts)
			}

			return v(project, target, entries, evts, err)
		}
	}

	p := &lt.TestPlan{
		BackendClient: &deploytest.BackendClient{
			GetStackOutputsF: func(ctx context.Context, name string, _ func(error) error) (resource.PropertyMap, error) {
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
		Options: lt.TestUpdateOptions{T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...)},
		Steps:   steps,
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
	p.Steps = []lt.TestStep{{
		Op:          Update,
		SkipPreview: true,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)
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

			return err
		},
	}}
	p.Run(t, old)

	// Test that unknown stacks are handled appropriately.
	programF = deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, err := mon.RegisterResource("pulumi:pulumi:StackReference", "other", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "rehto",
			}),
		})
		assert.Error(t, err)
		return err
	})
	p.Options = lt.TestUpdateOptions{T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...)}
	p.Steps = []lt.TestStep{{
		Op:            Update,
		ExpectFailure: true,
		SkipPreview:   true,
	}}
	p.Run(t, nil)

	// Test that unknown properties cause errors.
	programF = deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, mon *deploytest.ResourceMonitor) error {
		_, err := mon.RegisterResource("pulumi:pulumi:StackReference", "other", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"name": "other",
				"foo":  "bar",
			}),
		})
		assert.Error(t, err)
		return err
	})
	p.Options = lt.TestUpdateOptions{T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...)}
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
					ConfigureF: func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
						go func() {
							<-release
							host.Log(diag.Info, "", "configuring pkgA provider...", 0)
							close(done)
						}()
						return plugin.ConfigureResponse{}, nil
					},
				}, nil
			}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return nil, errors.New("pkgB load failure")
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource(providers.MakeProviderType("pkgB"), "provB", true)
		assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")

		return nil
	})

	op := lt.TestOp(Update)
	sink := diag.DefaultSink(sinkWriter, sinkWriter, diag.FormatOptions{Color: colors.Raw})
	options := lt.TestUpdateOptions{T: t, HostF: deploytest.NewPluginHostF(sink, sink, programF, loaders...)}
	p := &lt.TestPlan{}
	project, target := p.GetProject(), p.GetTarget(t, nil)

	_, err := op.Run(project, target, options, true, nil, nil)
	assert.Error(t, err)

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
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					assert.Equal(t, expectedIgnoreChanges, req.IgnoreChanges)
					return plugin.DiffResult{}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					assert.Equal(t, expectedIgnoreChanges, req.IgnoreChanges)
					return plugin.UpdateResponse{}, nil
				},
			}, nil
		}),
	}

	updateProgramWithProps := func(snap *deploy.Snapshot, props resource.PropertyMap, ignoreChanges []string,
		allowedOps []display.StepOp, name string,
	) *deploy.Snapshot {
		expectedIgnoreChanges = ignoreChanges
		programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:        props,
				IgnoreChanges: ignoreChanges,
			})
			assert.NoError(t, err)
			return nil
		})
		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
		p := &lt.TestPlan{
			// Skip display tests because secrets are serialized with the blinding crypter and can't be restored
			Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
			Steps: []lt.TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, err error,
					) error {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								if payload.Metadata.URN == "urn:pulumi:test::test::pkgA:m:typA::resA" {
									assert.Subset(t,
										allowedOps, []display.StepOp{payload.Metadata.Op},
										"event operation unexpected: %v", payload)
								}
							}
						}
						return err
					},
				},
			},
		}
		return p.RunWithName(t, snap, name)
	}

	snap := updateProgramWithProps(nil, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": "foo",
		},
		"d": []interface{}{1},
		"e": []interface{}{1},
		"f": map[string]interface{}{
			"g": map[string]interface{}{
				"h": "bar",
			},
		},
	}), []string{"a", "b.c", "d", "e[0]", "f.g[\"h\"]"}, []display.StepOp{deploy.OpCreate}, "initial")

	// Ensure that a change to an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 2,
		"b": map[string]interface{}{
			"c": "bar",
		},
		"d": []interface{}{2},
		"e": []interface{}{2},
		"f": map[string]interface{}{
			"g": map[string]interface{}{
				"h": "baz",
			},
		},
	}), []string{"a", "b.c", "d", "e[0]", "f.g[\"h\"]"}, []display.StepOp{deploy.OpSame}, "ignored-property")

	// Ensure that a change to an un-ignored property results in an OpUpdate
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 3,
		"b": map[string]interface{}{
			"c": "qux",
		},
		"d": []interface{}{3},
		"e": []interface{}{3},
		"f": map[string]interface{}{
			"g": map[string]interface{}{
				"h": "qux",
			},
		},
	}), nil, []display.StepOp{deploy.OpUpdate}, "unignored-property")

	// Ensure that a removing an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"e": []interface{}{},
	}), []string{"a", "b", "d", "e", "f"}, []display.StepOp{deploy.OpSame}, "ignored-property-removed")

	// Ensure that a removing an un-ignored property results in an OpUpdate
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"e": []interface{}{},
	}), nil, []display.StepOp{deploy.OpUpdate}, "unignored-property-removed")

	// Ensure that adding an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 4,
		"b": map[string]interface{}{
			"c": "zed",
		},
		"d": []interface{}{4},
		"e": []interface{}{},
	}), []string{"a", "b", "d", "e[0]", "f"}, []display.StepOp{deploy.OpSame}, "ignored-property-added")

	// Ensure that adding an un-ignored property results in an OpUpdate
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"e": []interface{}{},
		"i": 4,
	}), []string{"a", "b", "d", "e", "f"}, []display.StepOp{deploy.OpUpdate}, "unignored-property-added")

	// Ensure that sub-elements of arrays can be ignored, first reset to a simple state
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 3,
		"b": []string{"foo", "bar"},
	}), nil, []display.StepOp{deploy.OpUpdate}, "sub-elements-ignored")

	// Check that ignoring a specific sub-element of an array works
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 3,
		"b": []string{"foo", "baz"},
	}), []string{"b[1]"}, []display.StepOp{deploy.OpSame}, "ignore-specific-subelement")

	// Check that ignoring all sub-elements of an array works
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 3,
		"b": []string{"foo", "baz"},
	}), []string{"b[*]"}, []display.StepOp{deploy.OpSame}, "ignore-all-subelements")

	// Check that ignoring a secret value works, first update to make foo, bar secret
	snap = updateProgramWithProps(snap, resource.PropertyMap{
		"a": resource.NewNumberProperty(3),
		"b": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("foo"),
			resource.NewStringProperty("bar"),
		})),
	}, nil, []display.StepOp{deploy.OpUpdate}, "ignore-secret")

	// Now check that changing a value (but not secretness) can be ignored
	_ = updateProgramWithProps(snap, resource.PropertyMap{
		"a": resource.NewNumberProperty(3),
		"b": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("foo"),
			resource.NewStringProperty("baz"),
		})),
	}, []string{"b[1]"}, []display.StepOp{deploy.OpSame}, "change-value-not-secretness")
}

func TestIgnoreChangesInvalidPaths(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := func(monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
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

	runtimeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return program(monitor)
	})
	hostF := deploytest.NewPluginHostF(nil, nil, runtimeF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	program = func(monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:        resource.PropertyMap{},
			IgnoreChanges: []string{"foo.bar"},
		})
		assert.Error(t, err)
		return nil
	}

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.Error(t, err)

	program = func(monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"qux": resource.NewArrayProperty([]resource.PropertyValue{}),
			},
			IgnoreChanges: []string{"qux[0]"},
		})
		assert.Error(t, err)
		return nil
	}

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.Error(t, err)

	program = func(monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:        resource.PropertyMap{},
			IgnoreChanges: []string{"qux[0]"},
		})
		assert.Error(t, err)
		return nil
	}

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "3")
	assert.Error(t, err)

	program = func(monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"qux": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("zed"),
					resource.NewStringProperty("zob"),
				}),
			},
			IgnoreChanges: []string{"qux[1]"},
		})
		assert.Error(t, err)
		return nil
	}

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "4")
	assert.Error(t, err)
}

type DiffFunc = func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error)

func replaceOnChangesTest(t *testing.T, name string, diffFunc DiffFunc) {
	t.Run(name, func(t *testing.T) {
		t.Parallel()

		loaders := []*deploytest.ProviderLoader{
			deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
				return &deploytest.Provider{
					DiffF: diffFunc,
					CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
						return plugin.CreateResponse{
							ID:         resource.ID("id123"),
							Properties: req.Properties,
							Status:     resource.StatusOK,
						}, nil
					},
				}, nil
			}),
		}

		updateProgramWithProps := func(snap *deploy.Snapshot, props resource.PropertyMap, replaceOnChanges []string,
			allowedOps []display.StepOp,
		) *deploy.Snapshot {
			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
					Inputs:           props,
					ReplaceOnChanges: replaceOnChanges,
				})
				assert.NoError(t, err)
				return nil
			})
			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF},
				Steps: []lt.TestStep{
					{
						Op: Update,
						Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
							events []Event, err error,
						) error {
							for _, event := range events {
								if event.Type == ResourcePreEvent {
									payload := event.Payload().(ResourcePreEventPayload)
									// Ignore any events for default providers
									if !payload.Internal {
										assert.Subset(t, allowedOps, []display.StepOp{payload.Metadata.Op})
									}
								}
							}
							return err
						},
					},
				},
			}
			return p.RunWithName(t, snap, strings.ReplaceAll(fmt.Sprintf("%v", props), " ", "_"))
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
		func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
			// To establish a observable difference between the provider and engine diff function,
			// we treat 42 as an OpSame. We use this to check that the right diff function is being
			// used.
			for k, v := range req.NewInputs {
				if v == resource.NewNumberProperty(42) {
					req.NewInputs[k] = req.OldOutputs[k]
				}
			}
			diff := req.OldOutputs.Diff(req.NewInputs)
			if diff == nil {
				return plugin.DiffResult{Changes: plugin.DiffNone}, nil
			}
			detailedDiff := plugin.NewDetailedDiffFromObjectDiff(diff, false)
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

func TestPersistentDiff(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error) {
					return plugin.DiffResult{Changes: plugin.DiffSome}, nil
				},
			}, nil
		}),
	}

	inputs := resource.PropertyMap{}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update.
	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	// First, make no change to the inputs and run a preview. We should see an update to the resource due to
	// provider diffing.
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, err error,
		) error {
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
			return err
		})
	assert.NoError(t, err)

	// Next, enable legacy diff behavior. We should see no changes to the resource.
	p.Options.UseLegacyDiff = true
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, err error,
		) error {
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
			return err
		})
	assert.NoError(t, err)
}

func TestDetailedDiffReplace(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error) {
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
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update.
	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	// First, make no change to the inputs and run a preview. We should see an update to the resource due to
	// provider diffing.
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, err error,
		) error {
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
			return err
		})
	assert.NoError(t, err)
}

func TestCustomTimeouts(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			CustomTimeouts: &resource.CustomTimeouts{
				Create: 60, Delete: 60, Update: 240,
			},
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default")
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
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
				DiffConfigF: func(
					_ context.Context,
					req plugin.DiffConfigRequest,
				) (plugin.DiffResult, error) {
					// Always require replacement if any diff exists.
					if !req.OldOutputs.DeepEquals(req.NewInputs) {
						keys := []resource.PropertyKey{}
						for k := range req.NewInputs {
							keys = append(keys, k)
						}
						return plugin.DiffResult{Changes: plugin.DiffSome, ReplaceKeys: keys}, nil
					}
					return plugin.DiffResult{Changes: plugin.DiffNone}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Config: config.Map{
			config.MustMakeKey("pkgA", "foo"): config.NewValue("bar"),
		},
	}

	// Build a basic lifecycle.
	steps := lt.MakeBasicLifecycleSteps(t, 2)

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
	p.Steps = []lt.TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, err error,
		) error {
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

			return err
		},
	}}
	p.Run(t, snap)
}

func TestMissingRead(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{}, nil
				},
			}, nil
		}),
	}

	// Our program reads a resource and exits.
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "resA-some-id", "", resource.PropertyMap{}, "", "", "", "")
		assert.Error(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps:   []lt.TestStep{{Op: Update, ExpectFailure: true}},
	}
	p.Run(t, nil)
}

func TestProviderPreview(t *testing.T) {
	t.Parallel()

	sawPreview := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.Preview {
						sawPreview = true
					}

					assert.Equal(t, req.Preview, req.Properties.ContainsUnknowns())
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					if req.Preview {
						sawPreview = true
					}

					assert.Equal(t, req.Preview, req.NewInputs.ContainsUnknowns())
					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	preview := true
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		assert.True(t, resp.Outputs.DeepEquals(ins))

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run a preview. The inputs should be propagated to the outputs by the provider during the create.
	preview, sawPreview = true, false
	_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.True(t, sawPreview)

	// Run an update.
	preview, sawPreview = false, false
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.False(t, sawPreview)

	// Run another preview. The inputs should be propagated to the outputs during the update.
	preview, sawPreview = true, false
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, preview, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.True(t, sawPreview)
}

func TestProviderPreviewGrpc(t *testing.T) {
	t.Parallel()

	sawPreview := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.Preview {
						sawPreview = true
					}

					assert.Equal(t, req.Preview, req.Properties.ContainsUnknowns())
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					if req.Preview {
						sawPreview = true
					}

					assert.Equal(t, req.Preview, req.NewInputs.ContainsUnknowns())
					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	preview := true
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		assert.True(t, resp.Outputs.DeepEquals(ins))

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run a preview. The inputs should be propagated to the outputs by the provider during the create.
	preview, sawPreview = true, false
	_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.True(t, sawPreview)

	// Run an update.
	preview, sawPreview = false, false
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.False(t, sawPreview)

	// Run another preview. The inputs should be propagated to the outputs during the update.
	preview, sawPreview = true, false
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, preview, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.True(t, sawPreview)
}

func TestProviderPreviewUnknowns(t *testing.T) {
	t.Parallel()

	sawPreview := false
	loaders := []*deploytest.ProviderLoader{
		// NOTE: it is important that this test uses a gRPC-wrapped provider. The code that handles previews for unconfigured
		// providers is specific to the gRPC layer.
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					name := req.Args["name"]
					ret := "unexpected"
					if name.IsString() {
						ret = "Hello, " + name.StringValue() + "!"
					}

					return plugin.InvokeResponse{
						Properties: resource.NewPropertyMapFromMap(map[string]interface{}{
							"message": ret,
						}),
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.Preview {
						sawPreview = true
					}

					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					if req.Preview {
						sawPreview = true
					}

					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
				CallF: func(
					_ context.Context,
					req plugin.CallRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.CallResponse, error) {
					if req.Info.DryRun {
						sawPreview = true
					}

					assert.Equal(t, []resource.URN{"urn:pulumi:test::test::pkgA:m:typB::resB"}, req.Options.ArgDependencies["name"])

					ret := "unexpected"
					if req.Args["name"].IsString() {
						ret = "Hello, " + req.Args["name"].StringValue() + "!"
					}

					return plugin.CallResponse{
						Return: resource.NewPropertyMapFromMap(map[string]interface{}{
							"message": ret,
						}),
					}, nil
				},
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					if req.Info.DryRun {
						sawPreview = true
					}

					var err error
					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
						Parent:  req.Parent,
						Aliases: aliasesFromAliases(req.Options.Aliases),
						Protect: req.Options.Protect,
					})
					assert.NoError(t, err)

					_, err = monitor.RegisterResource("pkgA:m:typB", req.Name+"-resB", true, deploytest.ResourceOptions{
						Parent: resp.URN,
					})
					assert.NoError(t, err)

					outs := resource.PropertyMap{"foo": req.Inputs["name"]}
					err = monitor.RegisterResourceOutputs(resp.URN, outs)
					assert.NoError(t, err)

					return plugin.ConstructResponse{
						URN:     resp.URN,
						Outputs: outs,
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	preview := true
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		computed := interface{}(resource.Computed{Element: resource.NewStringProperty("")})
		if !preview {
			computed = "alpha"
		}

		resp, err := monitor.RegisterResource("pulumi:providers:pkgA", "provA", true,
			deploytest.ResourceOptions{
				Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{"foo": computed}),
			})
		require.NoError(t, err)

		provRef, err := providers.NewReference(resp.URN, resp.ID)
		assert.NoError(t, err)

		ins := resource.NewPropertyMapFromMap(map[string]interface{}{
			"foo": "bar",
			"baz": map[string]interface{}{
				"a": 42,
			},
			"qux": []interface{}{
				24,
			},
		})

		resp, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:   ins,
			Provider: provRef.String(),
		})
		require.NoError(t, err)

		if preview {
			assert.True(t, resp.Outputs.DeepEquals(resource.PropertyMap{}))
		} else {
			assert.True(t, resp.Outputs.DeepEquals(ins))
		}

		respC, err := monitor.RegisterResource("pkgA:m:typB", "resB", false, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"name": resp.Outputs["foo"],
			},
			Remote:   true,
			Provider: provRef.String(),
		})
		if preview {
			// We expect construction of remote component resources to fail during previews if the provider is
			// configured with unknowns.
			assert.NoError(t, err)
			assert.True(t, respC.Outputs.DeepEquals(resource.PropertyMap{}))
		} else {
			assert.NoError(t, err)
			assert.True(t, respC.Outputs.DeepEquals(resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
			}))
		}

		var outs resource.PropertyMap
		if preview {
			// We can't send any args or dependencies in preview because the RegisterResource call above failed.
			outs, _, _, err = monitor.Call("pkgA:m:typA/methodA", nil, nil, provRef.String(), "", "")
			assert.NoError(t, err)
		} else {
			outs, _, _, err = monitor.Call("pkgA:m:typA/methodA", resource.PropertyMap{
				"name": respC.Outputs["foo"],
			}, map[resource.PropertyKey][]resource.URN{
				"name": {respC.URN},
			}, provRef.String(), "", "")
			assert.NoError(t, err)
		}
		if preview {
			assert.True(t, outs.DeepEquals(resource.PropertyMap{}), "outs was %v", outs)
		} else {
			assert.True(t, outs.DeepEquals(resource.PropertyMap{
				"message": resource.NewStringProperty("Hello, bar!"),
			}), "outs was %v", outs)
		}

		if preview {
			outs, _, err = monitor.Invoke("pkgA:m:invokeA", resource.PropertyMap{
				"name": resource.PropertyValue{},
			}, provRef.String(), "", "")
		} else {
			outs, _, err = monitor.Invoke("pkgA:m:invokeA", resource.PropertyMap{
				"name": respC.Outputs["foo"],
			}, provRef.String(), "", "")
		}
		assert.NoError(t, err)
		if preview {
			assert.True(t, outs.DeepEquals(resource.PropertyMap{}), "outs was %v", outs)
		} else {
			assert.True(t, outs.DeepEquals(resource.PropertyMap{
				"message": resource.NewStringProperty("Hello, bar!"),
			}), "outs was %v", outs)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run a preview. The inputs should not be propagated to the outputs by the provider during the create because the
	// provider has unknown inputs.
	preview, sawPreview = true, false
	_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	require.NoError(t, err)
	assert.False(t, sawPreview)

	// Run an update.
	preview, sawPreview = false, false
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, preview, p.BackendClient, nil)
	require.NoError(t, err)
	assert.False(t, sawPreview)

	// Run another preview. The inputs should not be propagated to the outputs during the update because the provider
	// has unknown inputs.
	preview, sawPreview = true, false
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, preview, p.BackendClient, nil)
	require.NoError(t, err)
	assert.False(t, sawPreview)
}

type updateContext struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	*deploytest.ResourceMonitor

	resmon       chan *deploytest.ResourceMonitor
	programErr   chan error
	snap         chan *deploy.Snapshot
	updateResult chan error
}

func startUpdate(t *testing.T, hostF deploytest.PluginHostFactory) (*updateContext, error) {
	ctx := &updateContext{
		resmon:       make(chan *deploytest.ResourceMonitor),
		programErr:   make(chan error),
		snap:         make(chan *deploy.Snapshot),
		updateResult: make(chan error),
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

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Runtime: "client",
		RuntimeOptions: map[string]interface{}{
			"address": fmt.Sprintf("127.0.0.1:%d", port),
		},
	}

	go func() {
		snap, err := lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
		ctx.snap <- snap
		close(ctx.snap)
		ctx.updateResult <- err
		close(ctx.updateResult)
		stop <- true
	}()

	ctx.ResourceMonitor = <-ctx.resmon
	return ctx, nil
}

func (ctx *updateContext) Finish(err error) (*deploy.Snapshot, error) {
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
	conn, err := grpc.NewClient(
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

func (ctx *updateContext) GetPluginInfo(_ context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
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

	update, err := startUpdate(t, deploytest.NewPluginHostF(nil, nil, nil, loaders...))
	if err != nil {
		t.Fatalf("failed to start update: %v", err)
	}

	// Register resources, etc.
	_, err = update.RegisterResource("pkgA:m:typA", "resA", true)
	assert.NoError(t, err)

	snap, err := update.Finish(nil)
	assert.NoError(t, err)
	assert.Len(t, snap.Resources, 2)
}

func TestConfigSecrets(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	crypter := config.NewSymmetricCrypter(make([]byte, 32))
	secret, err := crypter.EncryptValue(context.Background(), "hunter2")
	assert.NoError(t, err)

	p := &lt.TestPlan{
		// Skip display tests because secrets are serialized with the blinding crypter and can't be restored
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
		Steps:   lt.MakeBasicLifecycleSteps(t, 2),
		Config: config.Map{
			config.MustMakeKey("pkgA", "secret"): config.NewSecureValue(secret),
		},
		Decrypter: crypter,
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

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
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("component", "resA", false)
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{}, resp.Outputs)

		err = monitor.RegisterResourceOutputs(resp.URN, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps:   lt.MakeBasicLifecycleSteps(t, 1),
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
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						// If foo changes do a replace, we use this to check we don't delete on replace
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", idCounter))
					idCounter = idCounter + 1
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleteCounter = deleteCounter + 1
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	shouldProtect := true
	createResource := true
	expectError := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:  ins,
				Protect: &shouldProtect,
			})
			if expectError {
				assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
			} else {
				assert.NoError(t, err)
			}
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())
	assert.Equal(t, 0, deleteCounter)

	expectedUrn := snap.Resources[1].URN
	expectedMessage := ""

	// Both updates below should give a diagnostic event
	validate := func(project workspace.Project,
		target deploy.Target, entries JournalEntries,
		events []Event, err error,
	) error {
		for _, event := range events {
			if event.Type == DiagEvent {
				payload := event.Payload().(DiagEventPayload)
				assert.Equal(t, expectedUrn, payload.URN)
				assert.Equal(t, expectedMessage, payload.Message)
				break
			}
		}
		return err
	}

	// Run a dry-run (preview) which will cause a replace, we should get an error.
	// However, the preview doesn't bail, so we expect one "created" resource as a result of this operation.
	expectedMessage = "<{%reset%}>unable to replace resource \"urn:pulumi:test::test::pkgA:m:typA::resA\"\n" +
		"as it is currently marked for protection. To unprotect the resource, remove the `protect` flag from " +
		"the resource in your Pulumi program and run `pulumi up`<{%reset%}>\n"
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient, validate, "1")
	assert.ErrorContains(t, err, "step generator errored")

	// Run an update which will cause a replace, we should get an error.
	// Contrary to the preview, the error is a bail, so no resources are created.
	expectError = true
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "2")
	assert.Error(t, err)
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
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "3")
	assert.Error(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())
	assert.Equal(t, true, snap.Resources[1].Protect)
	assert.Equal(t, 0, deleteCounter)

	// Run a new update to remove the protect and replace in the same update, this should delete the old one
	// and create the new one
	expectError = false
	createResource = true
	shouldProtect = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "4")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-2", snap.Resources[1].ID.String())
	assert.Equal(t, false, snap.Resources[1].Protect)
	assert.Equal(t, 1, deleteCounter)

	// Run a new update to add the protect flag, nothing else should change
	shouldProtect = true
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "5")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-2", snap.Resources[1].ID.String())
	assert.Equal(t, true, snap.Resources[1].Protect)
	assert.Equal(t, 1, deleteCounter)

	// Edit the snapshot to remove the protect flag and try and replace
	snap.Resources[1].Protect = false
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "daz",
	})
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "6")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-3", snap.Resources[1].ID.String())
	assert.Equal(t, 2, deleteCounter)
}

func TestDeletedWith(t *testing.T) {
	t.Parallel()

	idCounter := 0

	topURN := resource.URN("")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						// If foo changes do a replace, we use this to check we don't delete on replace
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", idCounter))
					idCounter = idCounter + 1
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.URN != topURN {
						// Only topURN (aURN) should be actually deleted
						assert.Fail(t, "Delete was called")
					}
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	createResource := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: ins,
			})
			assert.NoError(t, err)
			topURN = respA.URN

			respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs:      ins,
				DeletedWith: respA.URN,
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
				Inputs:      ins,
				DeletedWith: respB.URN,
			})
			assert.NoError(t, err)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
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
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, "created-id-3", snap.Resources[1].ID.String())
	assert.Equal(t, "created-id-4", snap.Resources[2].ID.String())
	assert.Equal(t, "created-id-5", snap.Resources[3].ID.String())

	// Run a new update which will cause a delete, we still shouldn't see a provider delete for anything but aURN
	createResource = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.NoError(t, err)
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "", "", resource.PropertyMap{}, "", "", "", "")
		assert.Error(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	validate := ExpectDiagMessage(t, regexp.QuoteMeta(
		"<{%reset%}>Expected an ID for urn:pulumi:test::test::pkgA:m:typA::resA<{%reset%}>"))

	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 1)
}

func TestEventSecrets(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					diff := req.OldOutputs.Diff(req.NewInputs)
					if diff == nil {
						return plugin.DiffResult{Changes: plugin.DiffNone}, nil
					}
					detailedDiff := plugin.NewDetailedDiffFromObjectDiff(diff, false)
					changedKeys := diff.ChangedKeys()

					return plugin.DiffResult{
						Changes:      plugin.DiffSome,
						ChangedKeys:  changedKeys,
						DetailedDiff: detailedDiff,
					}, nil
				},

				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "id123",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var inputs resource.PropertyMap
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because secrets are serialized with the blinding crypter and can't be restored
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
		Steps: []lt.TestStep{{
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
		evts []Event, err error,
	) error {
		for _, e := range evts {
			var step StepEventMetadata
			//nolint:exhaustive // We only care about a subset of events here
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
		return err
	}
	p.Run(t, snap)
}

func TestAdditionalSecretOutputs(t *testing.T) {
	t.Parallel()

	t.Skip("AdditionalSecretOutputs warning is currently disabled")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "id123",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var inputs resource.PropertyMap
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:                  inputs,
			AdditionalSecretOutputs: []resource.PropertyKey{"a", "b"},
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
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
		err error,
	) error {
		if err != nil {
			return err
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
		return errors.New("Expected a diagnostic message, got none")
	}
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	assert.NoError(t, err)

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
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource(
			resource.RootStackType,
			info.Project+"-"+info.Stack,
			false,
			deploytest.ResourceOptions{})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource(
			"pkgA:m:typA",
			"resA",
			true,
			deploytest.ResourceOptions{})
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
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
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if strings.Contains(string(req.URN), "typB") && failCreationOfTypB {
						return plugin.CreateResponse{}, errors.New("Could not create typB")
					}

					id := resource.ID(strconv.Itoa(len(cloudState)))
					if !req.Preview {
						cloudState[id] = req.Properties
					}
					return plugin.CreateResponse{
						ID:         id,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					// Fail if anything in cloud state still points to us
					for other, res := range cloudState {
						for _, v := range res {
							if v.IsString() && v.StringValue() == string(req.ID) {
								return plugin.DeleteResponse{}, fmt.Errorf("Can not delete %s used by %s", req.ID, other)
							}
						}
					}

					delete(cloudState, req.ID)
					return plugin.DeleteResponse{}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if strings.Contains(string(req.URN), "typA") {
						if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
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
					if strings.Contains(string(req.URN), "typB") {
						if !req.OldOutputs["parent"].DeepEquals(req.NewInputs["parent"]) {
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
				UpdateF: func(context.Context, plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					assert.Fail(t, "Didn't expect update to be called")
					return plugin.UpdateResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typB", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"parent": resp.ID,
			}),
			Dependencies: []resource.URN{resp.URN},
		})
		if failCreationOfTypB {
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		} else {
			assert.NoError(t, err)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resources
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)

	// Trigger a replacement of A but fail to create B
	failCreationOfTypB = true
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	// Assert that this fails, we should have two copies of A now, one new one and one old one pending delete
	assert.Error(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, snap.Resources[1].Type, tokens.Type("pkgA:m:typA"))
	assert.False(t, snap.Resources[1].Delete)
	assert.Equal(t, snap.Resources[2].Type, tokens.Type("pkgA:m:typA"))
	assert.True(t, snap.Resources[2].Delete)

	// Now allow B to create and try again
	failCreationOfTypB = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
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
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					if !req.Preview {
						id = resource.ID(strconv.Itoa(cloudID))
						cloudID = cloudID + 1
						cloudState[id] = req.Properties
					}
					return plugin.CreateResponse{
						ID:         id,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					// Fail if anything in cloud state still points to us
					for _, res := range cloudState {
						for _, v := range res {
							if v.IsString() && v.StringValue() == string(req.ID) {
								return plugin.DeleteResponse{}, fmt.Errorf("Can not delete %s", req.ID)
							}
						}
					}

					if strings.Contains(string(req.URN), "typB") && failDeletionOfTypB {
						return plugin.DeleteResponse{}, errors.New("Could not delete typB")
					}

					delete(cloudState, req.ID)
					return plugin.DeleteResponse{}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if strings.Contains(string(req.URN), "typA") {
						if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
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
					if strings.Contains(string(req.URN), "typB") {
						if !req.OldOutputs["parent"].DeepEquals(req.NewInputs["parent"]) {
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
						if !req.OldOutputs["frob"].DeepEquals(req.NewInputs["frob"]) {
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
				UpdateF: func(context.Context, plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					assert.Fail(t, "Didn't expect update to be called")
					return plugin.UpdateResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	insA := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	inB := "active"
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: insA,
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typB", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"parent": respA.ID,
				"frob":   inB,
			}),
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"parent": {respA.URN},
			},
			Dependencies: []resource.URN{respA.URN},
		})
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resources
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)

	// Trigger a replacement of B but fail to delete it
	inB = "inactive"
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	// Assert that this fails, we should have two B's one marked to delete
	assert.Error(t, err)
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
	snap, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	// Assert this is ok, we should have just one A and B
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, snap.Resources[1].Type, tokens.Type("pkgA:m:typA"))
	assert.False(t, snap.Resources[1].Delete)
	assert.Equal(t, snap.Resources[2].Type, tokens.Type("pkgA:m:typB"))
	assert.False(t, snap.Resources[2].Delete)
}

func TestTimestampTracking(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error) {
					return plugin.DiffResult{Changes: plugin.DiffSome}, nil
				},
				UpdateF: func(context.Context, plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					outputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"foo": "bar",
					})
					return plugin.UpdateResponse{
						Properties: outputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource(
			resource.RootStackType,
			info.Project+"-"+info.Stack,
			false,
			deploytest.ResourceOptions{})
		require.NoError(t, err)

		_, err = monitor.RegisterResource(
			"pkgA:m:typA",
			"resA",
			true,
			deploytest.ResourceOptions{})
		require.NoError(t, err)

		return nil
	})

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Options.T = t
	// Run an update to create the resource -- created and updated should be set and equal.
	p.Steps = []lt.TestStep{{Op: Update, SkipPreview: true}}
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
	p.Steps = []lt.TestStep{{Op: Refresh, SkipPreview: true}}
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
	p.Steps = []lt.TestStep{{Op: Update, SkipPreview: true}}
	snap = p.Run(t, snap)
	require.NotEmpty(t, snap.Resources)
	for _, resource := range snap.Resources {
		assert.NotNil(t, resource.Created, resource.URN, "missing created time: %v", resource.URN)
		assert.NotNil(t, resource.Modified, resource.URN, "missing modified time: %v", resource.URN)
		assert.Equal(t, creationTimes[resource.URN], *resource.Created,
			"created time changed: %v", resource.URN)

		//exhaustive:ignore
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

func TestOldCheckedInputsAreSent(t *testing.T) {
	// Test for https://github.com/pulumi/pulumi/issues/5973, check that the old inputs from Check are passed
	// to Diff, Update, and Delete.
	t.Parallel()

	firstUpdate := true

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(
					_ context.Context,
					req plugin.CheckRequest,
				) (plugin.CheckResponse, error) {
					// Check that the old inputs are passed to CheckF
					if firstUpdate {
						assert.Nil(t, req.Olds)
						assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
							"foo": "bar",
						}), req.News)
					} else {
						assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
							"foo":     "bar",
							"default": "default",
						}), req.Olds)
						assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
							"foo": "baz",
						}), req.News)
					}

					// Add a default property
					results := resource.PropertyMap{}
					for k, v := range req.News {
						results[k] = v
					}
					results["default"] = resource.NewStringProperty("default")

					return plugin.CheckResponse{Properties: results}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					// Check that the old inputs and outputs are passed to DiffF
					if firstUpdate {
						assert.Nil(t, req.OldInputs)
						assert.Nil(t, req.OldOutputs)
						assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
							"foo": "bar",
						}), req.NewInputs)
					} else {
						assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
							"foo":     "bar",
							"default": "default",
						}), req.OldInputs)
						assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
							"foo":      "bar",
							"default":  "default",
							"computed": "computed",
						}), req.OldOutputs)
						assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
							"foo":     "baz",
							"default": "default",
						}), req.NewInputs)
					}

					// Let the engine do the diff, we just want to assert the conditions above
					return plugin.DiffResult{}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					results := resource.PropertyMap{}
					for k, v := range req.Properties {
						results[k] = v
					}
					// Add a computed property
					results["computed"] = resource.MakeComputed(resource.NewStringProperty(""))

					if !req.Preview {
						id = resource.ID("1")
						results["computed"] = resource.NewStringProperty("computed")
					}
					return plugin.CreateResponse{
						ID:         id,
						Properties: results,
						Status:     resource.StatusOK,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					// Check that the old inputs and outputs are passed to UpdateF
					assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
						"foo":     "bar",
						"default": "default",
					}), req.OldInputs)
					assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
						"foo":      "bar",
						"default":  "default",
						"computed": "computed",
					}), req.OldOutputs)
					assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
						"foo":     "baz",
						"default": "default",
					}), req.NewInputs)

					results := resource.PropertyMap{}
					for k, v := range req.NewInputs {
						results[k] = v
					}
					// Add a computed property
					results["computed"] = resource.MakeComputed(resource.NewStringProperty(""))

					if !req.Preview {
						results["computed"] = resource.NewStringProperty("computed")
					}

					return plugin.UpdateResponse{
						Properties: results,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					// Check that the old inputs and outputs are passed to UpdateF
					assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
						"foo":     "baz",
						"default": "default",
					}), req.Inputs)
					assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
						"foo":      "baz",
						"default":  "default",
						"computed": "computed",
					}), req.Outputs)

					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	insA := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: insA,
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resources
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	resA := snap.Resources[1]
	assert.Equal(t, tokens.Type("pkgA:m:typA"), resA.Type)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":     "bar",
		"default": "default",
	}), resA.Inputs)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":      "bar",
		"default":  "default",
		"computed": "computed",
	}), resA.Outputs)

	// Now run another update with new inputs
	insA = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	firstUpdate = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	resA = snap.Resources[1]
	assert.Equal(t, tokens.Type("pkgA:m:typA"), resA.Type)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":     "baz",
		"default": "default",
	}), resA.Inputs)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo":      "baz",
		"default":  "default",
		"computed": "computed",
	}), resA.Outputs)

	// Now run a destroy to delete the resource and check the stored inputs and outputs are sent
	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 0)
}

func TestResourceNames(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/issues/10117
	t.Parallel()

	cases := []string{
		"foo",
		":colons",
		"-dashes",
		"file/path.txt",
		"bar|table",
		"spaces in names",
		"email@address",
		"<output object>",
		"[brackets]",
		"{braces}",
		"(parens)",
		"C:\\windows\\paths",
		"& @ $ % ^ * #",
		"'quotes'",
		"\"double quotes\"",
		"double::colons", // https://github.com/pulumi/pulumi/issues/13968
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							return plugin.CreateResponse{
								ID:         "1",
								Properties: resource.PropertyMap{},
								Status:     resource.StatusOK,
							}, nil
						},
					}, nil
				}),
			}
			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				// Check the name works as a provider
				resp, err := monitor.RegisterResource("pulumi:providers:pkgA", tt, true)
				assert.NoError(t, err)

				provRef, err := providers.NewReference(resp.URN, resp.ID)
				assert.NoError(t, err)

				// And a custom resource
				respCustom, err := monitor.RegisterResource("pkgA:m:typA", tt, true, deploytest.ResourceOptions{
					Provider: provRef.String(),
				})
				assert.NoError(t, err)

				// And a component resource
				_, err = monitor.RegisterResource("pkgA:m:typB", tt, false, deploytest.ResourceOptions{
					// And as a URN parameter
					Parent: respCustom.URN,
				})
				assert.NoError(t, err)

				return nil
			})
			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF},
			}

			snap, err := lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)

			require.NoError(t, err)
			require.Len(t, snap.Resources, 3)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pulumi:providers:pkgA::"+tt), snap.Resources[0].URN)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::"+tt), snap.Resources[1].URN)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typB::"+tt), snap.Resources[2].URN)
		})
	}
}

func TestSourcePositions(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							Inputs:  req.Inputs,
							Outputs: req.State,
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	const regPos = "/test/source/positions#1,2"
	const readPos = "/test/source/positions#3,4"
	inputs := resource.PropertyMap{}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:         inputs,
			SourcePosition: "file://" + regPos,
		})
		require.NoError(t, err)

		_, _, err = monitor.ReadResource("pkgA:m:typA", "resB", "id", "", inputs, "", "", "file://"+readPos, "")
		require.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	regURN := p.NewURN("pkgA:m:typA", "resA", "")
	readURN := p.NewURN("pkgA:m:typA", "resB", "")

	// Run the initial update.
	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	assert.Len(t, snap.Resources, 3)

	reg := snap.Resources[1]
	assert.Equal(t, regURN, reg.URN)
	assert.Equal(t, "project://"+regPos, reg.SourcePosition)

	read := snap.Resources[2]
	assert.Equal(t, readURN, read.URN)
	assert.Equal(t, "project://"+readPos, read.SourcePosition)
}

func TestBadResourceOptionURNs(t *testing.T) {
	// Test for https://github.com/pulumi/pulumi/issues/13490, check that if a user (or SDK) sends a malformed
	// URN we return an error.

	t.Parallel()

	cases := []struct {
		name     string
		opts     deploytest.ResourceOptions
		assertFn func(err error)
	}{
		{
			name: "malformed alias urn",
			opts: deploytest.ResourceOptions{
				Aliases: []*pulumirpc.Alias{
					makeUrnAlias("very-bad urn"),
				},
			},
			assertFn: func(err error) {
				assert.ErrorContains(t, err, "invalid alias URN: invalid URN \"very-bad urn\"")
			},
		},
		{
			name: "malformed alias parent urn",
			opts: deploytest.ResourceOptions{
				Aliases: []*pulumirpc.Alias{
					makeSpecAliasWithParent("", "", "", "", "very-bad urn"),
				},
			},
			assertFn: func(err error) {
				assert.ErrorContains(t, err, "invalid parent alias URN: invalid URN \"very-bad urn\"")
			},
		},
		{
			name: "malformed parent urn",
			opts: deploytest.ResourceOptions{
				Parent: "very-bad urn",
			},
			assertFn: func(err error) {
				assert.ErrorContains(t, err, "invalid parent URN: invalid URN \"very-bad urn\"")
			},
		},
		{
			name: "malformed deleted with urn",
			opts: deploytest.ResourceOptions{
				DeletedWith: "very-bad urn",
			},
			assertFn: func(err error) {
				assert.ErrorContains(t, err, "invalid DeletedWith URN: invalid URN \"very-bad urn\"")
			},
		},
		{
			name: "malformed dependency",
			opts: deploytest.ResourceOptions{
				Dependencies: []resource.URN{"very-bad urn"},
			},
			assertFn: func(err error) {
				assert.ErrorContains(t, err, "invalid dependency URN: invalid URN \"very-bad urn\"")
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{}, nil
				}, deploytest.WithoutGrpc),
			}

			programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pkgA:m:typA", "res", true, tt.opts)
				tt.assertFn(err)
				return nil
			})
			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF},
			}

			project := p.GetProject()

			snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
			assert.NoError(t, err)
			assert.NotNil(t, snap)
		})
	}
}

func TestProviderChecksums(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	createResource := true
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: ins,
				PluginChecksums: map[string][]byte{
					"windows-x64": {0, 1, 2, 3, 4},
				},
			})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	// Check the checksum was saved in the provider resource
	assert.Equal(t, tokens.Type("pulumi:providers:pkgA"), snap.Resources[0].Type)
	checksums := snap.Resources[0].Inputs["__internal"].ObjectValue()["pluginChecksums"].ObjectValue()
	assert.Equal(t, "0001020304", checksums["windows-x64"].StringValue())

	// Delete the resource and ensure the checksums are passed to EnsurePlugins
	createResource = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 0)
}

// Regression test for https://github.com/pulumi/pulumi/issues/14040, ensure the step generators automatic
// diff is tagged as an input diff.
func TestAutomaticDiff(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	inputs := resource.PropertyMap{
		"foo": resource.NewNumberProperty(1),
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Run the initial update.
	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	// Change the inputs and run again
	inputs = resource.PropertyMap{
		"foo": resource.NewNumberProperty(2),
	}
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, err error,
		) error {
			found := false
			for _, e := range events {
				if e.Type == ResourcePreEvent {
					p := e.Payload().(ResourcePreEventPayload).Metadata
					if p.URN == resURN {
						// Should find an update op with the diff set to an input diff
						assert.Equal(t, deploy.OpUpdate, p.Op)
						assert.Equal(t, []resource.PropertyKey{"foo"}, p.Diffs)
						assert.Equal(t, map[string]plugin.PropertyDiff{
							"foo": {
								Kind:      plugin.DiffUpdate,
								InputDiff: true,
							},
						}, p.DetailedDiff)
						found = true
					}
				}
			}
			assert.True(t, found)
			return err
		})
	assert.NoError(t, err)
}

// TestStackOutputsProgramError tests that previous stack outputs aren't deleted when an update fails because
// of a program error.
func TestStackOutputsProgramError(t *testing.T) {
	t.Parallel()

	var step int

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource(resource.RootStackType, "test", false)
		assert.NoError(t, err)

		val := resource.NewProperty(fmt.Sprintf("step %v", step))

		var outputs resource.PropertyMap

		switch step {
		case 0, 3:
			outputs = resource.PropertyMap{
				"first":  val,
				"second": val,
			}
		case 1:
			// If an error is raised between calling `pulumi.export("first", ...)` and `pulumi.export("second", ...)`
			// in SDKs like Python and Go, the first export is still registered via RegisterResourceOutputs.
			// This test simulates that by not including "second" when the program will error.
			outputs = resource.PropertyMap{
				"first": val,
			}
		case 2:
			// The Node.js SDK is a bit different, when an error is thrown between module exports, none of the exports
			// are included. An empty set of outputs is registered via RegisterResourceOutputs.
			outputs = resource.PropertyMap{}
		}

		err = monitor.RegisterResourceOutputs(resp.URN, outputs)
		assert.NoError(t, err)

		if step == 1 || step == 2 {
			return errors.New("program error")
		}

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	validateSnapshot := func(snap *deploy.Snapshot, expectedResourceCount int, expectedOutputs resource.PropertyMap) {
		assert.Len(t, snap.Resources, expectedResourceCount)
		assert.Equal(t, resource.RootStackType, snap.Resources[0].Type)
		assert.Equal(t, expectedOutputs, snap.Resources[0].Outputs)
	}

	// Run the initial update which sets some stack outputs.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnapshot(snap, 1, resource.PropertyMap{
		"first":  resource.NewProperty("step 0"),
		"second": resource.NewProperty("step 0"),
	})

	// Run another update where the program fails before registering all of the stack outputs, simulating the behavior
	// of returning an error after only the first output is set.
	// Ensure the original stack outputs are preserved.
	step = 1
	snap, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "program error")
	validateSnapshot(snap, 1, resource.PropertyMap{
		"first":  resource.NewProperty("step 1"),
		"second": resource.NewProperty("step 0"), // Prior output is preserved
	})

	// Run another update that fails to update both stack updates.
	// Ensure the prior stack outputs are preserved.
	step = 2
	snap, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.ErrorContains(t, err, "program error")
	validateSnapshot(snap, 1, resource.PropertyMap{
		"first":  resource.NewProperty("step 1"), // Prior output is preserved
		"second": resource.NewProperty("step 0"), // Prior output is preserved
	})

	// Run again, this time without erroring, to ensure the stack outputs are updated.
	step = 3
	snap, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "3")
	assert.NoError(t, err)
	validateSnapshot(snap, 1, resource.PropertyMap{
		"first":  resource.NewProperty("step 3"),
		"second": resource.NewProperty("step 3"),
	})
}

// TestStackOutputsResourceError tests that previous stack outputs aren't deleted when an update fails
// due to a resource operation error.
func TestStackOutputsResourceError(t *testing.T) {
	t.Parallel()

	var step int

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{Status: resource.StatusUnknown}, errors.New("oh no")
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource(resource.RootStackType, "test", false)
		assert.NoError(t, err)

		val := resource.NewProperty(fmt.Sprintf("step %v", step))

		switch step {
		case 0, 3:
			outsErr := monitor.RegisterResourceOutputs(resp.URN, resource.PropertyMap{
				"first":  val,
				"second": val,
			})
			assert.NoError(t, outsErr)

		case 1:
			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
			// RegisterResourceOutputs not called here, simulating what happens in SDKs when an output of resA
			// is exported as a stack output.

		case 2:
			outsErr := monitor.RegisterResourceOutputs(resp.URN, resource.PropertyMap{
				"first":  val,
				"second": val,
			})
			assert.NoError(t, outsErr)

			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		}

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		// Skip display tests because secrets are serialized with the blinding crypter and can't be restored
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	validateSnapshot := func(snap *deploy.Snapshot, expectedResourceCount int, expectedOutputs resource.PropertyMap) {
		assert.Len(t, snap.Resources, expectedResourceCount)
		assert.Equal(t, resource.RootStackType, snap.Resources[0].Type)
		assert.Equal(t, expectedOutputs, snap.Resources[0].Outputs)
	}

	// Run the initial update which sets some stack outputs.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnapshot(snap, 1, resource.PropertyMap{
		"first":  resource.NewProperty("step 0"),
		"second": resource.NewProperty("step 0"),
	})

	// Run another that simulates creating a resource that will error during creation and exporting an output of that
	// resource as a stack output, in which case no RegisterResourceOutputs call is made.
	step = 1
	snap, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "oh no")
	validateSnapshot(snap, 2, resource.PropertyMap{
		"first":  resource.NewProperty("step 0"), // Original output is preserved
		"second": resource.NewProperty("step 0"), // Original output is preserved
	})

	// Run another update that still registers a resource that will fail during creation, but do that after the
	// stack outputs are registered, which is in-line with the behavior of real-world programs.
	step = 2
	snap, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.ErrorContains(t, err, "oh no")
	validateSnapshot(snap, 2, resource.PropertyMap{
		"first":  resource.NewProperty("step 2"),
		"second": resource.NewProperty("step 2"),
	})

	// Run again, this time without erroring, to ensure the stack outputs are updated.
	step = 3
	snap, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "3")
	assert.NoError(t, err)
	validateSnapshot(snap, 1, resource.PropertyMap{
		"first":  resource.NewProperty("step 3"),
		"second": resource.NewProperty("step 3"),
	})
}

// Test that the step generator can issue diffs in parallel.
func TestParallelDiff(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	// We're going to expect to see two calls to diff, but we won't return from either till we see both
	wg.Add(2)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error) {
					wg.Done()
					wg.Wait()
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// Issue these register resources in parallel, neither will return till both are issued
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
			assert.NoError(t, err)
		}()

		go func() {
			defer wg.Done()
			_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
			assert.NoError(t, err)
		}()

		wg.Wait()

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			UpdateOptions: engine.UpdateOptions{
				ParallelDiff: true,
				// Need at least two workers for this
				Parallel: 2,
			},
			T:     t,
			HostF: hostF,
		},
	}

	// Run the initial update.
	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)

	// Now run a preview, expect the diff to be done in parallel.
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient, nil)
	require.NoError(t, err)

	// waitTimeout waits for the waitgroup for the specified max timeout.
	// Returns true if waiting timed out.
	waitTimeout := func(wg *sync.WaitGroup, timeout time.Duration) bool {
		c := make(chan struct{})
		go func() {
			defer close(c)
			wg.Wait()
		}()
		select {
		case <-c:
			return false // completed normally
		case <-time.After(timeout):
			return true // timed out
		}
	}

	// Wait for the diff to complete, but don't wait forever
	assert.False(t, waitTimeout(&wg, 10*time.Second), "waiting for diff to complete timed out")
}

func TestConstructHangsAfterRegisterResourceFailure(t *testing.T) {
	t.Parallel()

	done := make(chan bool)
	defer close(done)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					// Always fail the create operation
					return plugin.CreateResponse{}, errors.New("create failed intentionally")
				},
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					// Try to register a resource, which should fail
					_, err := monitor.RegisterResource("pkgA:m:typB", req.Name+"-child", true, deploytest.ResourceOptions{
						Parent: req.Parent,
					})
					require.Error(t, err)
					done <- true // This will block until we read from the channel at the end of the test
					return plugin.ConstructResponse{}, err
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.ErrorContains(t, err, "resource monitor shut down while waiting for construct to complete")
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run the update - it should complete with an error even though ConstructF hangs
	_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.True(t, result.IsBail(err))
	require.ErrorContains(t, err, "create failed intentionally")
	require.True(t, <-done)
}
