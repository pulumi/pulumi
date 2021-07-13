// Copyright 2016-2018, Pulumi Corporation.
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

// nolint: goconst
package lifecycletest

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
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
	Op  deploy.StepOp
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
	grpcDefault := flag.Bool("grpc-providers", false, "enable or disable gRPC providers by default")

	flag.Parse()

	if *grpcDefault {
		deploytest.UseGrpcProvidersByDefault = true
	}

	os.Exit(m.Run())
}

func TestEmptyProgramLifecycle(t *testing.T) {
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
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

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
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// Now run a preview. Expect a warning because the diff is unavailable.
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result) result.Result {

			found := false
			for _, e := range events {
				if e.Type == DiagEvent {
					p := e.Payload().(DiagEventPayload)
					if p.URN == resURN && p.Severity == diag.Warning && p.Message == "diff unavailable" {
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
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(urn resource.URN,
					olds, news resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
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
				evts []Event, res result.Result) result.Result {

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
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(urn resource.URN,
					olds, news resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
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
				evts []Event, res result.Result) result.Result {

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
				evts []Event, res result.Result) result.Result {

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

func (b brokenDecrypter) DecryptValue(ciphertext string) (string, error) {
	return "", fmt.Errorf(b.ErrorMessage)
}

// Tests that the engine presents a reasonable error message when a decrypter fails to decrypt a config value.
func TestBrokenDecrypter(t *testing.T) {
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
				evts []Event, res result.Result) result.Result {

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
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

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
	project, target := p.GetProject(), p.GetTarget(nil)

	_, res := op.RunWithContext(ctx, project, target, options, false, nil, nil)
	assertIsErrorOrBailResult(t, res)

	// Wait for the program to finish.
	<-done
}

// Tests that a preview works for a stack with pending operations.
func TestPreviewWithPendingOperations(t *testing.T) {
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
	project, target := p.GetProject(), p.GetTarget(old)

	// A preview should succeed despite the pending operations.
	_, res := op.Run(project, target, options, true, nil, nil)
	assert.Nil(t, res)

	// But an update should fail.
	_, res = op.Run(project, target, options, false, nil, nil)
	assertIsErrorOrBailResult(t, res)
	assert.EqualError(t, res.Error(), deploy.PlanPendingOperationsError{}.Error())
}

// Tests that a failed partial update causes the engine to persist the resource's old inputs and new outputs.
func TestUpdatePartialFailure(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},

				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

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
			evts []Event, res result.Result) result.Result {

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
					return nil, errors.Errorf("unknown stack \"%s\"", name)
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
			evts []Event, res result.Result) result.Result {

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

	p := &TestPlan{}
	provAURN := p.NewProviderURN("pkgA", "default", "")
	provBURN := p.NewProviderURN("pkgB", "default", "")

	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:    provAURN.Type(),
				URN:     provAURN,
				Custom:  true,
				ID:      "0",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			{
				Type:    provBURN.Type(),
				URN:     provBURN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
		},
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	op := TestOp(Update)
	sink := diag.DefaultSink(sinkWriter, sinkWriter, diag.FormatOptions{Color: colors.Raw})
	options := UpdateOptions{Host: deploytest.NewPluginHost(sink, sink, program, loaders...)}
	project, target := p.GetProject(), p.GetTarget(old)

	_, res := op.Run(project, target, options, true, nil, nil)
	assertIsErrorOrBailResult(t, res)

	close(sinkWriter.channel)
	close(release)
	<-done
}

func TestSingleResourceIgnoreChanges(t *testing.T) {
	var expectedIgnoreChanges []string

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					assert.Equal(t, expectedIgnoreChanges, ignoreChanges)
					return plugin.DiffResult{}, nil
				},
				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

					assert.Equal(t, expectedIgnoreChanges, ignoreChanges)
					return resource.PropertyMap{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	updateProgramWithProps := func(snap *deploy.Snapshot, props resource.PropertyMap, ignoreChanges []string,
		allowedOps []deploy.StepOp) *deploy.Snapshot {
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
						events []Event, res result.Result) result.Result {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Subset(t, allowedOps, []deploy.StepOp{payload.Metadata.Op})
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
	}), []string{"a", "b.c"}, []deploy.StepOp{deploy.OpCreate})

	// Ensure that a change to an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 2,
		"b": map[string]interface{}{
			"c": "bar",
		},
	}), []string{"a", "b.c"}, []deploy.StepOp{deploy.OpSame})

	// Ensure that a change to an un-ignored property results in an OpUpdate
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 3,
		"b": map[string]interface{}{
			"c": "qux",
		},
	}), nil, []deploy.StepOp{deploy.OpUpdate})

	// Ensure that a removing an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.PropertyMap{}, []string{"a", "b"}, []deploy.StepOp{deploy.OpSame})

	// Ensure that a removing an un-ignored property results in an OpUpdate
	snap = updateProgramWithProps(snap, resource.PropertyMap{}, nil, []deploy.StepOp{deploy.OpUpdate})

	// Ensure that adding an ignored property results in an OpSame
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 4,
		"b": map[string]interface{}{
			"c": "zed",
		},
	}), []string{"a", "b"}, []deploy.StepOp{deploy.OpSame})

	// Ensure that adding an un-ignored property results in an OpUpdate
	_ = updateProgramWithProps(snap, resource.PropertyMap{
		"c": resource.NewNumberProperty(4),
	}, []string{"a", "b"}, []deploy.StepOp{deploy.OpUpdate})
}

func objectDiffToDetailedDiff(prefix string, d *resource.ObjectDiff) map[string]plugin.PropertyDiff {
	ret := map[string]plugin.PropertyDiff{}
	for k, vd := range d.Updates {
		var nestedPrefix string
		if prefix == "" {
			nestedPrefix = string(k)
		} else {
			nestedPrefix = fmt.Sprintf("%s.%s", prefix, string(k))
		}
		for kk, pd := range valueDiffToDetailedDiff(nestedPrefix, vd) {
			ret[kk] = pd
		}
	}
	return ret
}

func arrayDiffToDetailedDiff(prefix string, d *resource.ArrayDiff) map[string]plugin.PropertyDiff {
	ret := map[string]plugin.PropertyDiff{}
	for i, vd := range d.Updates {
		for kk, pd := range valueDiffToDetailedDiff(fmt.Sprintf("%s[%d]", prefix, i), vd) {
			ret[kk] = pd
		}
	}
	return ret
}

func valueDiffToDetailedDiff(prefix string, vd resource.ValueDiff) map[string]plugin.PropertyDiff {
	ret := map[string]plugin.PropertyDiff{}
	if vd.Object != nil {
		for kk, pd := range objectDiffToDetailedDiff(prefix, vd.Object) {
			ret[kk] = pd
		}
	} else if vd.Array != nil {
		for kk, pd := range arrayDiffToDetailedDiff(prefix, vd.Array) {
			ret[kk] = pd
		}
	} else {
		ret[prefix] = plugin.PropertyDiff{Kind: plugin.DiffUpdate}
	}
	return ret
}

func TestReplaceOnChanges(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					diff := olds.Diff(news)
					if diff == nil {
						return plugin.DiffResult{Changes: plugin.DiffNone}, nil
					}
					detailedDiff := objectDiffToDetailedDiff("", diff)
					var changedKeys []resource.PropertyKey
					for _, k := range diff.Keys() {
						if diff.Changed(k) {
							changedKeys = append(changedKeys, k)
						}
					}
					return plugin.DiffResult{
						Changes:      plugin.DiffSome,
						ChangedKeys:  changedKeys,
						DetailedDiff: detailedDiff,
					}, nil
				},
				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {
					return news, resource.StatusOK, nil
				},
				CreateF: func(urn resource.URN, inputs resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return resource.ID("id123"), inputs, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	updateProgramWithProps := func(snap *deploy.Snapshot, props resource.PropertyMap, replaceOnChanges []string,
		allowedOps []deploy.StepOp) *deploy.Snapshot {
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
						events []Event, res result.Result) result.Result {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Subset(t, allowedOps, []deploy.StepOp{payload.Metadata.Op})
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
	}), []string{"a", "b.c"}, []deploy.StepOp{deploy.OpCreate})

	// Ensure that a change to a replaceOnChange property results in an OpReplace
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 2,
		"b": map[string]interface{}{
			"c": "foo",
		},
	}), []string{"a"}, []deploy.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

	// Ensure that a change to a nested replaceOnChange property results in an OpReplace
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 2,
		"b": map[string]interface{}{
			"c": "bar",
		},
	}), []string{"b.c"}, []deploy.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

	// Ensure that a change to any property of a "*" replaceOnChange results in an OpReplace
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 3,
		"b": map[string]interface{}{
			"c": "baz",
		},
	}), []string{"*"}, []deploy.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

	// Ensure that a change to an non-replaceOnChange property results in an OpUpdate
	snap = updateProgramWithProps(snap, resource.NewPropertyMapFromMap(map[string]interface{}{
		"a": 4,
		"b": map[string]interface{}{
			"c": "qux",
		},
	}), nil, []deploy.StepOp{deploy.OpUpdate})

	_ = snap
}

// Resource is an abstract representation of a resource graph
type Resource struct {
	t                   tokens.Type
	name                string
	children            []Resource
	props               resource.PropertyMap
	aliases             []resource.URN
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
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// The `forcesReplacement` key forces replacement and all other keys can update in place
				DiffF: func(res resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

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
		snap *deploy.Snapshot, resources []Resource, allowedOps []deploy.StepOp) *deploy.Snapshot {
		program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			err := registerResources(t, monitor, resources)
			return err
		})
		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result) result.Result {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Subset(t, allowedOps, []deploy.StepOp{payload.Metadata.Op})
							}
						}

						for _, entry := range entries {
							if entry.Step.Type() == "pulumi:providers:pkgA" {
								continue
							}
							switch entry.Kind {
							case JournalEntrySuccess:
								assert.Subset(t, allowedOps, []deploy.StepOp{entry.Step.Op()})
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
	}}, []deploy.StepOp{deploy.OpCreate})

	// Ensure that rename produces Same
	snap = updateProgramWithResource(snap, []Resource{{
		t:       "pkgA:index:t1",
		name:    "n2",
		aliases: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1::n1"},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that rename produces Same with multiple aliases
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:index:t1::n2",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that rename produces Same with multiple aliases (reversed)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n2",
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that aliasing back to original name is okay
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n3",
			"urn:pulumi:test::test::pkgA:index:t1::n2",
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that changing the type works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t2",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that changing the type again works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:index:t2::n1",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that order of aliases doesn't matter
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:othermod:t3::n1",
			"urn:pulumi:test::test::pkgA:index:t2::n1",
		},
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
	}}, []deploy.StepOp{deploy.OpSame})

	// Ensure that changing everything (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t4",
		name: "n2",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(42),
		},
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:othermod:t3::n1",
		},
	}}, []deploy.StepOp{deploy.OpUpdate})

	// Ensure that changing everything again (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t5",
		name: "n3",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(1000),
		},
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t4::n2",
		},
	}}, []deploy.StepOp{deploy.OpUpdate})

	// Ensure that changing a forceNew property while also changing type and name leads to replacement not delete+create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t6",
		name: "n4",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1000),
		},
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t5::n3",
		},
	}}, []deploy.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

	// Ensure that changing a forceNew property and deleteBeforeReplace while also changing type and name leads to
	// replacement not delete+create
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t7",
		name: "n5",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(999),
		},
		deleteBeforeReplace: true,
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t6::n4",
		},
	}}, []deploy.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

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
	}}, []deploy.StepOp{deploy.OpCreate})

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:            "pkgA:index:t2-new",
		name:         "n2-new",
		dependencies: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1-new::n1-new"},
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t2::n2",
		},
	}}, []deploy.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

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
	}}, []deploy.StepOp{deploy.OpCreate})

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:      "pkgA:index:t2-new",
		name:   "n2-new",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new::n1-new"),
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n2",
		},
	}}, []deploy.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced})

	// ensure failure when different resources use duplicate aliases
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n2",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []deploy.StepOp{deploy.OpCreate})

	err := snap.NormalizeURNReferences()
	assert.Equal(t, err.Error(),
		"Two resources ('urn:pulumi:test::test::pkgA:index:t1::n1'"+
			" and 'urn:pulumi:test::test::pkgA:index:t2::n2') aliased to the same: 'urn:pulumi:test::test::pkgA:index:t1::n1'")

	// ensure different resources can use different aliases
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n2",
		aliases: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n2",
		},
	}}, []deploy.StepOp{deploy.OpCreate})

	err = snap.NormalizeURNReferences()
	assert.Nil(t, err)
}

func TestPersistentDiff(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

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
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// First, make no change to the inputs and run a preview. We should see an update to the resource due to
	// provider diffing.
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result) result.Result {

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
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result) result.Result {

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
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

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
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// First, make no change to the inputs and run a preview. We should see an update to the resource due to
	// provider diffing.
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
			events []Event, res result.Result) result.Result {

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
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {
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
			_ []Event, res result.Result) result.Result {

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
	sawPreview := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					if preview {
						sawPreview = true
					}

					assert.Equal(t, preview, news.ContainsUnknowns())
					return "created-id", news, resource.StatusOK, nil
				},
				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

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
	_, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.True(t, sawPreview)

	// Run an update.
	preview, sawPreview = false, false
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.False(t, sawPreview)

	// Run another preview. The inputs should be propagated to the outputs during the update.
	preview, sawPreview = true, false
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.True(t, sawPreview)
}

func TestProviderPreviewGrpc(t *testing.T) {
	sawPreview := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					if preview {
						sawPreview = true
					}

					assert.Equal(t, preview, news.ContainsUnknowns())
					return "created-id", news, resource.StatusOK, nil
				},
				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

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
	_, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.True(t, sawPreview)

	// Run an update.
	preview, sawPreview = false, false
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.False(t, sawPreview)

	// Run another preview. The inputs should be propagated to the outputs during the update.
	preview, sawPreview = true, false
	_, res = TestOp(Update).Run(project, p.GetTarget(snap), p.Options, preview, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.True(t, sawPreview)
}

func TestSingleComponentDefaultProviderLifecycle(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(monitor *deploytest.ResourceMonitor,
				typ, name string, parent resource.URN, inputs resource.PropertyMap,
				options plugin.ConstructOptions) (plugin.ConstructResult, error) {

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
	*deploytest.ResourceMonitor

	resmon       chan *deploytest.ResourceMonitor
	programErr   chan error
	snap         chan *deploy.Snapshot
	updateResult chan result.Result
}

func startUpdate(host plugin.Host) (*updateContext, error) {
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
		snap, res := TestOp(Update).Run(p.GetProject(), p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
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
	req *pulumirpc.GetRequiredPluginsRequest) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (ctx *updateContext) Run(_ context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	// Connect to the resource monitor and create an appropriate client.
	conn, err := grpc.Dial(
		req.MonitorAddress,
		grpc.WithInsecure(),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "could not connect to resource monitor")
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

func TestLanguageClient(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	update, err := startUpdate(deploytest.NewPluginHost(nil, nil, nil, loaders...))
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
	var urnB resource.URN
	var idB resource.ID

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(monitor *deploytest.ResourceMonitor, typ, name string, parent resource.URN,
				inputs resource.PropertyMap, options plugin.ConstructOptions) (plugin.ConstructResult, error) {

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
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", inputs, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
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
	secret, err := crypter.EncryptValue("hunter2")
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
	snap, res := TestOp(Update).Run(project, p.GetTarget(nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	if !assert.Len(t, snap.Resources, 2) {
		return
	}

	provider := snap.Resources[0]
	assert.True(t, provider.Inputs["secret"].IsSecret())
	assert.True(t, provider.Outputs["secret"].IsSecret())
}

func TestComponentOutputs(t *testing.T) {
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
	var urn resource.URN

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(monitor *deploytest.ResourceMonitor,
				typ, name string, parent resource.URN, inputs resource.PropertyMap,
				options plugin.ConstructOptions) (plugin.ConstructResult, error) {

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
				info plugin.CallInfo, options plugin.CallOptions) (plugin.CallResult, error) {

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
	var urn resource.URN

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(monitor *deploytest.ResourceMonitor,
				typ, name string, parent resource.URN, inputs resource.PropertyMap,
				options plugin.ConstructOptions) (plugin.ConstructResult, error) {

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
				info plugin.CallInfo, options plugin.CallOptions) (plugin.CallResult, error) {

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
