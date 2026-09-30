// Copyright 2026, Pulumi Corporation.
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

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/util/pdag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/propertyrpc"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type RunInfo struct {
	Project        string
	Stack          string
	Organization   string
	RootDirectory  string
	ProgramDir     string
	WorkingDir     string
	Config         map[string]string
	ConfigSecrets  []string
	MonitorAddress string
	EngineAddress  string
	LoaderAddress  string
	DryRun         bool
	Parallel       int32

	// PackageDescriptors are package blocks keyed by package name.
	PackageDescriptors map[string]*schema.PackageDescriptor
}

type Interpreter struct {
	program *pcl.Program
	info    RunInfo

	monitor pulumirpc.ResourceMonitorClient
	engine  pulumirpc.EngineClient
	loader  schema.ReferenceLoader

	evalContext *EvalContext
	stackURN    string

	// namePrefix is prepended to resource and component names when this interpreter is executing
	// inside a component. For example, if a component named "myComp" contains a resource "res",
	// the resource is registered as "myComp-res". Nested components accumulate the prefix.
	namePrefix string

	// packageRefs are package references returned by RegisterPackage keyed by package name.
	packageRefs map[string]string

	// callbacks is the server that handles resource hook callbacks.
	callbacks     *pclCallbackServer
	callbacksOnce sync.Once
	callbacksErr  error

	// snippetID is the UUID of the snippet driving this interpreter, if any. When set, it is
	// propagated onto every RegisterResourceRequest emitted by the interpreter.
	snippetID string
}

func NewInterpreter(program *pcl.Program, info RunInfo) *Interpreter {
	return &Interpreter{
		program:     program,
		info:        info,
		packageRefs: map[string]string{},
	}
}

// pclCallbackServer is a gRPC server that handles resource hook callbacks for the PCL runtime.
type pclCallbackServer struct {
	pulumirpc.UnimplementedCallbacksServer

	stop          chan bool
	handle        rpcutil.ServeHandle
	functions     map[string]func(context.Context, []byte) (proto.Message, error)
	functionsLock sync.RWMutex
}

func newPCLCallbackServer() (*pclCallbackServer, error) {
	s := &pclCallbackServer{
		functions: map[string]func(context.Context, []byte) (proto.Message, error){},
		stop:      make(chan bool),
	}
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: s.stop,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterCallbacksServer(srv, s)
			return nil
		},
		Options: rpcutil.TracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return nil, err
	}
	s.handle = handle
	return s, nil
}

func (s *pclCallbackServer) RegisterCallback(
	fn func(context.Context, []byte) (proto.Message, error),
) (*pulumirpc.Callback, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	uuidString := uuid.String()
	s.functionsLock.Lock()
	defer s.functionsLock.Unlock()
	s.functions[uuidString] = fn
	return &pulumirpc.Callback{
		Token:  uuidString,
		Target: "127.0.0.1:" + strconv.Itoa(s.handle.Port),
		// PCL decodes strings containing non-UTF8 bytes losslessly, so the engine may send them to callbacks
		// hosted here.
		AcceptsByteString: true,
	}, nil
}

func (s *pclCallbackServer) Invoke(
	ctx context.Context, req *pulumirpc.CallbackInvokeRequest,
) (*pulumirpc.CallbackInvokeResponse, error) {
	s.functionsLock.RLock()
	fn, ok := s.functions[req.Token]
	s.functionsLock.RUnlock()
	if !ok {
		return nil, errors.New("callback not found: " + req.Token)
	}
	resp, err := fn(ctx, req.Request)
	if err != nil {
		return nil, err
	}
	b, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshaling callback response: %w", err)
	}
	return &pulumirpc.CallbackInvokeResponse{Response: b}, nil
}

func (i *Interpreter) getCallbackServer() (*pclCallbackServer, error) {
	i.callbacksOnce.Do(func() {
		s, err := newPCLCallbackServer()
		i.callbacks = s
		i.callbacksErr = err
	})
	return i.callbacks, i.callbacksErr
}

// registerHookNode registers a named hook block with the engine, then stores the hook's
// registered name as a string variable so it can be referenced in resource hooks options.
// The command expression is evaluated lazily at invocation time so that `args.X` references
// can be resolved against the resource's actual args.
func (i *Interpreter) registerHookNode(ctx context.Context, h *pcl.Hook) error {
	if h.Command == nil {
		return fmt.Errorf("hook %s: missing command", h.Name())
	}

	onDryRun := false
	if h.OnDryRun != nil {
		odrVal, _, diags := i.evalContext.Evaluate(h.OnDryRun)
		if diags.HasErrors() {
			return diags
		}
		if odrVal.IsBool() {
			onDryRun = odrVal.AsBool()
		} else {
			return fmt.Errorf("hook %s: onDryRun must be a boolean", h.Name())
		}
	}

	ignoreErrors := false
	if h.IgnoreErrors != nil {
		ieVal, _, diags := i.evalContext.Evaluate(h.IgnoreErrors)
		if diags.HasErrors() {
			return diags
		}
		if ieVal.IsBool() {
			ignoreErrors = ieVal.AsBool()
		} else {
			return fmt.Errorf("hook %s: ignoreErrors must be a boolean", h.Name())
		}
	}

	hookName := i.effectiveName(h.LogicalName())
	cmdExpr := h.Command

	srv, err := i.getCallbackServer()
	if err != nil {
		return fmt.Errorf("creating callback server: %w", err)
	}

	workingDir := i.info.WorkingDir

	unmarshal := func(ctx context.Context, s *structpb.Struct) (cty.Value, error) {
		if s == nil {
			return cty.EmptyObjectVal, nil
		}

		props, err := propertyrpc.Unmarshal(s)
		if err != nil {
			return cty.EmptyObjectVal, err
		}
		val, err := propertyValueToCty(ctx, i.getResource, property.New(props))
		if err != nil {
			return cty.EmptyObjectVal, err
		}
		return val, nil
	}

	// hookArgs builds the `args` object for a hook invocation from the request's resource
	// identity and property structs.
	hookArgs := func(ctx context.Context, req interface {
		GetUrn() string
		GetId() string
		GetName() string
		GetType() string
	}, props map[string]*structpb.Struct,
	) (map[string]cty.Value, error) {
		args := map[string]cty.Value{
			"urn":  cty.StringVal(req.GetUrn()),
			"id":   cty.StringVal(req.GetId()),
			"name": cty.StringVal(req.GetName()),
			"type": cty.StringVal(req.GetType()),
		}
		for key, val := range props {
			val, err := unmarshal(ctx, val)
			if err != nil {
				return nil, fmt.Errorf("hook %s: failed to unmarshal %s: %w", hookName, key, err)
			}
			args[key] = val
		}
		return args, nil
	}

	// runCommand evaluates the hook's command with the given `args` and runs it. The first
	// return value is the error from running the command, if any; the second is an error
	// evaluating the command expression itself.
	runCommand := func(ctx context.Context, args map[string]cty.Value) (error, error) {
		evalCtx := i.evalContext.NewChild()
		evalCtx.SetVariable("args", cty.ObjectVal(args))

		cmdVal, _, evalDiags := evalCtx.Evaluate(cmdExpr)
		if evalDiags.HasErrors() {
			return nil, fmt.Errorf("hook %s: evaluating command: %v", hookName, evalDiags)
		}
		if !cmdVal.IsArray() {
			return nil, fmt.Errorf("hook %s: command must be a list of strings", hookName)
		}
		var cmdArgs []string
		for _, arg := range cmdVal.AsArray().All {
			if !arg.IsString() {
				return nil, fmt.Errorf("hook %s: command elements must be strings", hookName)
			}
			cmdArgs = append(cmdArgs, arg.AsString())
		}
		if len(cmdArgs) == 0 {
			return nil, fmt.Errorf("hook %s: command must not be empty", hookName)
		}

		cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = workingDir
		if out, runErr := cmd.CombinedOutput(); runErr != nil {
			return fmt.Errorf("hook command %v failed: %s\n%s", cmdArgs, runErr, out), nil
		}
		return nil, nil
	}

	if h.Kind == pcl.HookKindError {
		// Error hooks return whether the failed operation should be retried: retry if and
		// only if the command exits successfully.
		cb, err := srv.RegisterCallback(func(ctx context.Context, reqBytes []byte) (proto.Message, error) {
			var req pulumirpc.ErrorHookRequest
			if len(reqBytes) > 0 {
				if err := proto.Unmarshal(reqBytes, &req); err != nil {
					return &pulumirpc.ErrorHookResponse{
						Error: fmt.Sprintf("hook %s: failed to unmarshal request: %v", hookName, err),
					}, nil
				}
			}

			// Error hooks do not receive new outputs: they fire after an operation
			// fails, so there are none.
			args, err := hookArgs(ctx, &req, map[string]*structpb.Struct{
				"newInputs":  req.GetNewInputs(),
				"oldInputs":  req.GetOldInputs(),
				"oldOutputs": req.GetOldOutputs(),
			})
			if err != nil {
				return &pulumirpc.ErrorHookResponse{Error: err.Error()}, nil
			}

			runErr, evalErr := runCommand(ctx, args)
			if evalErr != nil {
				return &pulumirpc.ErrorHookResponse{Error: evalErr.Error()}, nil
			}
			return &pulumirpc.ErrorHookResponse{Retry: runErr == nil}, nil
		})
		if err != nil {
			return fmt.Errorf("allocating error hook callback %s: %w", hookName, err)
		}

		_, err = i.monitor.RegisterErrorHook(ctx, &pulumirpc.RegisterErrorHookRequest{
			Name:     hookName,
			Callback: cb,
		})
		if err != nil {
			return fmt.Errorf("registering error hook %s: %w", hookName, err)
		}

		// Store the hook's registered name as a string so it can be referenced in hooks options.
		i.evalContext.SetVariable(h.Name(), cty.StringVal(hookName))
		return nil
	}

	cb, err := srv.RegisterCallback(func(ctx context.Context, reqBytes []byte) (proto.Message, error) {
		var req pulumirpc.ResourceHookRequest
		if len(reqBytes) > 0 {
			if err := proto.Unmarshal(reqBytes, &req); err != nil {
				return &pulumirpc.ResourceHookResponse{
					Error: fmt.Sprintf("hook %s: failed to unmarshal request: %v", hookName, err),
				}, nil
			}
		}

		args, err := hookArgs(ctx, &req, map[string]*structpb.Struct{
			"newInputs":  req.GetNewInputs(),
			"oldInputs":  req.GetOldInputs(),
			"newOutputs": req.GetNewOutputs(),
			"oldOutputs": req.GetOldOutputs(),
		})
		if err != nil {
			return &pulumirpc.ResourceHookResponse{Error: err.Error()}, nil
		}

		runErr, evalErr := runCommand(ctx, args)
		if evalErr != nil {
			return &pulumirpc.ResourceHookResponse{Error: evalErr.Error()}, nil
		}
		if runErr != nil {
			return &pulumirpc.ResourceHookResponse{Error: runErr.Error()}, nil
		}
		return &pulumirpc.ResourceHookResponse{}, nil
	})
	if err != nil {
		return fmt.Errorf("allocating resource hook callback %s: %w", hookName, err)
	}

	_, err = i.monitor.RegisterResourceHook(ctx, &pulumirpc.RegisterResourceHookRequest{
		Name:         hookName,
		Callback:     cb,
		OnDryRun:     onDryRun,
		IgnoreErrors: ignoreErrors,
	})
	if err != nil {
		return fmt.Errorf("registering resource hook %s: %w", hookName, err)
	}

	// Store the hook's registered name as a string so it can be referenced in hooks options.
	i.evalContext.SetVariable(h.Name(), cty.StringVal(hookName))
	return nil
}

func (i *Interpreter) invoke(
	ctx context.Context, req *pulumirpc.ResourceInvokeRequest,
) (*pulumirpc.ResourceInvokeResponse, error) {
	req.PackageRef = i.getPackageRefFromToken(req.Tok)
	req.Parent = i.stackURN
	resp, err := i.monitor.Invoke(ctx, req)
	return resp, err
}

func (i *Interpreter) call(
	ctx context.Context, req *pulumirpc.ResourceCallRequest,
) (*pulumirpc.CallResponse, error) {
	req.PackageRef = i.getPackageRefFromToken(req.Tok)
	resp, err := i.monitor.Call(ctx, req)
	return resp, err
}

func (i *Interpreter) getResource(ctx context.Context, ref property.ResourceReference) (property.Map, error) {
	args, err := structpb.NewStruct(map[string]any{
		"urn": string(ref.URN),
	})
	contract.AssertNoErrorf(err, "failed to create structpb for resource reference")

	resp, err := i.monitor.Invoke(ctx, &pulumirpc.ResourceInvokeRequest{
		Tok:               "pulumi:pulumi:getResource",
		Args:              args,
		AcceptResources:   true,
		AcceptsByteString: true,
	})
	if err != nil {
		return property.Map{}, fmt.Errorf("invoke getResource for %s: %w", ref.URN, err)
	}

	outputs, err := propertyrpc.Unmarshal(resp.Return)
	if err != nil {
		return property.Map{}, fmt.Errorf("unmarshal stack outputs: %w", err)
	}
	return outputs.Get("state").AsMap().
		Set("id", ref.ID).
		Set("urn", property.New(string(ref.URN))).
		Set("__name", property.New(ref.URN.Name())).
		Set("__type", property.New(string(ref.URN.Type()))), nil
}

// effectiveName returns the name to use when registering a resource or component with the given
// logical name, prepending the current namePrefix if one is set.
func (i *Interpreter) effectiveName(logicalName string) string {
	if i.namePrefix == "" {
		return logicalName
	}
	return i.namePrefix + "-" + logicalName
}

func (i *Interpreter) Run(ctx context.Context) error {
	if i.info.MonitorAddress == "" {
		return errors.New("missing monitor address")
	}

	monitorConn, err := grpc.NewClient(
		i.info.MonitorAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return fmt.Errorf("connect to monitor: %w", err)
	}
	defer contract.IgnoreClose(monitorConn)
	i.monitor = pulumirpc.NewResourceMonitorClient(monitorConn)

	loader, err := schema.NewLoaderClient(i.info.LoaderAddress)
	if err != nil {
		return fmt.Errorf("connect to loader: %w", err)
	}
	i.loader = schema.NewCachedLoader(loader)

	if i.info.EngineAddress != "" {
		engineConn, err := grpc.NewClient(
			i.info.EngineAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			rpcutil.GrpcChannelOptions(),
		)
		if err != nil {
			return fmt.Errorf("connect to engine: %w", err)
		}
		defer contract.IgnoreClose(engineConn)
		i.engine = pulumirpc.NewEngineClient(engineConn)
	}

	i.evalContext = NewEvalContext(
		i.info.WorkingDir,
		i.info.RootDirectory,
		i.info.Organization,
		i.info.Project,
		i.info.Stack,
		i.lookupResource,
		i.lookupFunction,
		i.getResource,
		i.invoke,
		i.call,
	)

	if err := i.registerStack(ctx); err != nil {
		return err
	}

	if err := i.registerPackages(ctx); err != nil {
		return err
	}

	outputs, err := i.executeProgramNodes(ctx)
	if err != nil {
		return err
	}

	if err := i.registerStackOutputs(ctx, outputs); err != nil {
		return err
	}

	// Stop the callback server if it was started
	if i.callbacks != nil {
		close(i.callbacks.stop)
	}

	if i.monitor != nil {
		_, err := i.monitor.SignalAndWaitForShutdown(ctx, &emptypb.Empty{})
		if err != nil {
			if status, ok := status.FromError(err); ok && status.Code() == codes.Unimplemented {
				return nil
			}
			return err
		}
	}

	return nil
}

// RunEmbedded runs the interpreter against a pre-existing monitor and loader supplied by the
// caller, skipping the steps that only make sense for a top-level program run (stack registration,
// stack-output emission, and the SignalAndWaitForShutdown handshake). It is the entry point for
// callers that drive the interpreter as a sub-source inside a larger update — for example, the
// engine evaluating a PCL snippet alongside the main program.
//
// scopeVars, if non-nil, are converted and installed on the eval context as root-scope variables
// after init so the program body can reference resources resolved elsewhere (e.g. snippet
// References resolved via the engine's registration observer).
func (i *Interpreter) RunEmbedded(
	ctx context.Context,
	monitor pulumirpc.ResourceMonitorClient,
	loader schema.ReferenceLoader,
	scopeVars map[string]property.Value,
	snippetID string,
) error {
	i.monitor = monitor
	i.loader = loader
	i.snippetID = snippetID

	i.evalContext = NewEvalContext(
		i.info.WorkingDir,
		i.info.RootDirectory,
		i.info.Organization,
		i.info.Project,
		i.info.Stack,
		i.lookupResource,
		i.lookupFunction,
		i.getResource,
		i.invoke,
		i.call,
	)
	for name, val := range scopeVars {
		ctyVal, err := propertyValueToCty(ctx, i.getResource, val)
		if err != nil {
			return fmt.Errorf("converting scope variable %q: %w", name, err)
		}
		i.evalContext.SetVariable(name, ctyVal)
	}

	if err := i.registerPackages(ctx); err != nil {
		return err
	}

	if _, err := i.executeProgramNodes(ctx); err != nil {
		return err
	}

	if i.callbacks != nil {
		close(i.callbacks.stop)
	}

	return nil
}

func (i *Interpreter) executeProgramNodes(ctx context.Context) (property.Map, error) {
	dag := pdag.New[pcl.Node]()
	nodes := map[pcl.Node]pdag.Node{}

	for _, node := range i.program.Nodes {
		dagNode, done := dag.NewNode(node)
		done()
		nodes[node] = dagNode
	}

	for _, node := range i.program.Nodes {
		dagNodeA := nodes[node]
		for _, dep := range node.GetDependencies() {
			dagNodeB, ok := nodes[dep]
			contract.Assertf(ok, "missing node for dependency %s", dep.Name())
			err := dag.NewEdge(dagNodeB, dagNodeA)
			if err != nil {
				return property.Map{}, fmt.Errorf("failed to create edge from %s to %s: %w", dep.Name(), node.Name(), err)
			}
		}
	}

	// The required version check must run before any resource is created. Make every resource and
	// component depend on the pulumi block so the gate is enforced once its own dependencies (e.g. a
	// config variable holding the version) have been evaluated.
	if gate := i.requiredVersionGate(); gate != nil {
		gateDagNode := nodes[gate]
		for _, node := range i.program.Nodes {
			switch node.(type) {
			case *pcl.Resource, *pcl.Component:
				if err := dag.NewEdge(gateDagNode, nodes[node]); err != nil {
					return property.Map{}, fmt.Errorf("failed to create edge from %s to %s: %w",
						gate.Name(), node.Name(), err)
				}
			}
		}
	}

	var outputsLock sync.Mutex
	outputs := map[string]property.Value{}
	err := dag.Walk(ctx, func(ctx context.Context, node pcl.Node) error {
		switch node := node.(type) {
		case *pcl.ConfigVariable:
			// When executing a component program, config variables are supplied as component inputs
			// and set before the walk; don't override them with the enclosing program's config.
			if i.evalContext.HasVariable(node.Name()) {
				return nil
			}
			if diags := i.bindConfigVariable(ctx, node); diags.HasErrors() {
				return diags
			}
			return nil
		case *pcl.PulumiBlock:
			if err := i.enforceRequiredVersion(ctx); err != nil {
				return err
			}
			return nil
		case *pcl.Hook:
			if err := i.registerHookNode(ctx, node); err != nil {
				return fmt.Errorf("failed to register hook %s: %w", node.Name(), err)
			}
		case *pcl.LocalVariable:
			value, poison, diags := i.evalContext.Evaluate(node.Definition.Value)
			if poison != nil {
				i.evalContext.SetVariable(node.Name(), makePoisonValue(*poison))
				return nil
			}
			if diags.HasErrors() {
				return diags
			}
			if err := i.setVariable(ctx, node.Name(), value); err != nil {
				return fmt.Errorf("failed to set variable %s: %w", node.Name(), err)
			}
		case *pcl.Resource:
			if err := i.registerResource(ctx, node); err != nil {
				return fmt.Errorf("failed to register resource %s: %w", node.Name(), err)
			}
		case *pcl.ReadResource:
			if err := i.registerReadResource(ctx, node); err != nil {
				return fmt.Errorf("failed to read resource %s: %w", node.Name(), err)
			}
		case *pcl.Component:
			if err := i.registerComponent(ctx, node); err != nil {
				return fmt.Errorf("failed to register component %s: %w", node.Name(), err)
			}
		case *pcl.OutputVariable:
			value, poison, diags := i.evalContext.Evaluate(node.Value)
			if poison != nil {
				return nil
			}
			if diags.HasErrors() {
				return diags
			}
			outputsLock.Lock()
			outputs[node.LogicalName()] = value
			outputsLock.Unlock()
		default:
			return fmt.Errorf("unknown node type: %T", node)
		}
		return nil
	}, pdag.MaxProcs(int(i.info.Parallel)))
	if err != nil {
		return property.Map{}, err
	}

	return property.NewMap(outputs), nil
}

func (i *Interpreter) lookupResource(ctx context.Context, token string) (*schema.Resource, error) {
	pkg, mod, typ, diags := pcl.DecomposeToken(token, hcl.Range{})
	contract.Assertf(!diags.HasErrors(), "invalid token format for resource token %s", token)

	token = fmt.Sprintf("%s:%s:%s", pkg, mod, typ)
	pkgName := pkg
	if pkg == "pulumi" && mod == "providers" {
		pkgName = typ
	}

	descriptor := i.lookupPackageDescriptor(pkgName)
	pkgref, err := i.loader.LoadPackageReferenceV2(ctx, descriptor)
	if err != nil {
		return nil, fmt.Errorf("load package for token %s: %w", token, err)
	}
	if pkg == "pulumi" && mod == "providers" {
		return pkgref.Provider()
	}
	resources := pkgref.Resources()
	schemaResource, ok, err := resources.Get(token)
	if err != nil {
		return nil, fmt.Errorf("get resource from package for token %s: %w", token, err)
	}
	if !ok {
		return nil, fmt.Errorf("get resource from package for token %s", token)
	}
	return schemaResource, nil
}

func (i *Interpreter) lookupFunction(ctx context.Context, token string) (*schema.Function, error) {
	pkg, mod, typ, diags := pcl.DecomposeToken(token, hcl.Range{})
	contract.Assertf(!diags.HasErrors(), "invalid token format for function token %s", token)

	token = fmt.Sprintf("%s:%s:%s", pkg, mod, typ)

	descriptor := i.lookupPackageDescriptor(pkg)
	pkgref, err := i.loader.LoadPackageReferenceV2(ctx, descriptor)
	if err != nil {
		return nil, fmt.Errorf("load package for token %s: %w", token, err)
	}
	functions := pkgref.Functions()
	schemaFunction, ok, err := functions.Get(token)
	if err != nil {
		return nil, fmt.Errorf("get function from package for token %s: %w", token, err)
	}
	if !ok {
		return nil, fmt.Errorf("get function from package for token %s", token)
	}
	return schemaFunction, nil
}

func (i *Interpreter) lookupPackageDescriptor(pkgName string) *schema.PackageDescriptor {
	if descriptor, ok := i.info.PackageDescriptors[pkgName]; ok && descriptor != nil {
		return descriptor
	}
	return &schema.PackageDescriptor{Name: pkgName}
}

func PackageNameFromToken(token string) (string, error) {
	pkg, mod, name, diags := pcl.DecomposeToken(token, hcl.Range{})
	if diags.HasErrors() {
		return "", diags
	}
	if pkg == "pulumi" {
		if mod == "providers" {
			return name, nil
		}
		return "", nil
	}
	return pkg, nil
}

func (i *Interpreter) getPackageRefFromToken(token string) string {
	pkgName, err := PackageNameFromToken(token)
	contract.AssertNoErrorf(err, "invalid token %q", token)
	return i.packageRefs[pkgName]
}

func (i *Interpreter) registerPackages(ctx context.Context) error {
	if i.monitor == nil {
		return nil
	}

	keys := make([]string, 0, len(i.info.PackageDescriptors))
	for k := range i.info.PackageDescriptors {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		descriptor := i.info.PackageDescriptors[key]
		if descriptor == nil {
			continue
		}
		if descriptor.Parameterization == nil {
			continue
		}

		request := &pulumirpc.RegisterPackageRequest{
			Name:        descriptor.Name,
			DownloadUrl: descriptor.DownloadURL,
		}
		if descriptor.Version != nil {
			request.Version = descriptor.Version.String()
		}
		param := &pulumirpc.Parameterization{
			Name:    descriptor.Parameterization.Name,
			Version: descriptor.Parameterization.Version.String(),
			Value:   descriptor.Parameterization.Value,
		}
		// Route to the wire field matching the schema's parameterization flavor.
		pkgref, err := i.loader.LoadPackageReferenceV2(ctx, descriptor)
		if err != nil {
			return fmt.Errorf("load package %q for register: %w", key, err)
		}
		def, err := pkgref.Definition()
		if err != nil {
			return fmt.Errorf("load definition for package %q: %w", key, err)
		}
		if def.ExtensionParameterization != nil {
			request.Extension = param
		} else {
			request.Parameterization = param
		}

		resp, err := i.monitor.RegisterPackage(ctx, request)
		if err != nil {
			return fmt.Errorf("register package %q: %w", key, err)
		}
		if resp.GetRef() == "" {
			return fmt.Errorf("register package %q returned empty reference", key)
		}

		i.packageRefs[key] = resp.GetRef()
		i.packageRefs[descriptor.PackageName()] = resp.GetRef()
	}

	return nil
}

// requiredVersionGate returns the pulumi block that enforces a required engine version, or nil if the
// program has no such constraint.
func (i *Interpreter) requiredVersionGate() pcl.Node {
	for _, node := range i.program.Nodes {
		if block, ok := node.(*pcl.PulumiBlock); ok && block.RequiredVersion != nil {
			return node
		}
	}
	return nil
}

func (i *Interpreter) bindConfigVariable(ctx context.Context, cfg *pcl.ConfigVariable) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics
	secretKeys := map[string]struct{}{}
	for _, key := range i.info.ConfigSecrets {
		secretKeys[key] = struct{}{}
	}
	key := fmt.Sprintf("%s:%s", i.info.Project, cfg.LogicalName())
	raw, has := i.info.Config[key]
	if !has {
		if cfg.DefaultValue != nil {
			value, poison, diags := i.evalContext.Evaluate(cfg.DefaultValue)
			contract.Assertf(poison == nil, "config variables can't be poisoned")
			diagnostics = append(diagnostics, diags...)
			if _, isSecret := secretKeys[key]; isSecret || cfg.Secret {
				value = value.WithSecret(true)
			}
			if !diags.HasErrors() {
				if err := i.setVariable(ctx, cfg.Name(), value); err != nil {
					diagnostics = append(diagnostics, &hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  err.Error(),
					})
				}
			}
			return diagnostics
		}
		if !cfg.Nullable {
			rng := cfg.SyntaxNode().Range()
			diagnostics = append(diagnostics, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("missing required configuration value %q", cfg.LogicalName()),
				Subject:  &rng,
			})
		}
		return diagnostics
	}

	value, diags := parseConfigPropertyValue(raw, cfg.Type())
	diagnostics = append(diagnostics, diags...)
	if !diags.HasErrors() {
		if _, isSecret := secretKeys[key]; isSecret || cfg.Secret {
			value = value.WithSecret(true)
		}
		if err := i.setVariable(ctx, cfg.Name(), value); err != nil {
			diagnostics = append(diagnostics, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  err.Error(),
			})
		}
	}
	return diagnostics
}

func (i *Interpreter) enforceRequiredVersion(ctx context.Context) error {
	var required model.Expression
	for _, node := range i.program.Nodes {
		if block, ok := node.(*pcl.PulumiBlock); ok {
			required = block.RequiredVersion
			break
		}
	}
	if required == nil {
		return nil
	}

	value, poison, diags := i.evalContext.Evaluate(required)
	if poison != nil {
		return fmt.Errorf("could not evaluate requiredVersion because of failure from %s", *poison)
	}
	if diags.HasErrors() {
		return diags
	}
	if !value.IsString() {
		return errors.New("requiredVersion must be a string")
	}
	if i.engine == nil {
		return errors.New("engine client not available to validate requiredVersion")
	}

	_, err := i.engine.RequirePulumiVersion(ctx, &pulumirpc.RequirePulumiVersionRequest{
		PulumiVersionRange: value.AsString(),
	})
	return err
}

func (i *Interpreter) registerStack(ctx context.Context) error {
	stackName := fmt.Sprintf("%s-%s", i.info.Project, i.info.Stack)
	resp, err := i.monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:   string(resource.RootStackType),
		Name:   stackName,
		Custom: false,
	})
	if err != nil {
		return err
	}
	if resp.GetResult() != pulumirpc.Result_SUCCESS {
		return fmt.Errorf("stack registration failed: %s", resp.GetResult())
	}
	if resp.GetUrn() == "" {
		return errors.New("stack URN was empty")
	}
	if resp.Object != nil {
		objectValue, err := propertyrpc.Unmarshal(resp.Object)
		if err != nil {
			return fmt.Errorf("unmarshal stack outputs: %w", err)
		}

		err = i.setVariable(ctx, "pulumi", property.New(objectValue))
		if err != nil {
			return fmt.Errorf("set pulumi variable: %w", err)
		}
	}
	i.stackURN = resp.GetUrn()
	return nil
}

func allDependencies(value property.Value) []urn.URN {
	var deps []urn.URN
	var inner func(value property.Value)
	inner = func(value property.Value) {
		deps = append(deps, value.Dependencies()...)
		switch {
		case value.IsMap():
			for _, v := range value.AsMap().AllStable {
				inner(v)
			}
		case value.IsArray():
			for _, v := range value.AsArray().All {
				inner(v)
			}
		}
	}
	inner(value)
	return deps
}

// providerReferences translates an evaluated `providers` option into the package name to provider
// reference map the resource monitor expects. The option may be written either as an array of
// provider resources, in which case each provider's package is taken from its URN, or as a map from
// package name to provider resource.
func providerReferences(providers property.Value) (map[string]string, error) {
	reference := func(v property.Value) (string, string, error) {
		urn, id, err := unwrapResource(v)
		if err != nil {
			return "", "", fmt.Errorf("providers: %w", err)
		}
		idstr := plugin.UnknownStringValue
		if id.IsString() {
			idstr = id.AsString()
		}
		return urn, fmt.Sprintf("%s::%s", urn, idstr), nil
	}

	psopt := map[string]string{}
	switch {
	case providers.IsMap():
		for k, v := range providers.AsMap().All {
			_, ref, err := reference(v)
			if err != nil {
				return nil, err
			}
			psopt[k] = ref
		}
	case providers.IsArray():
		for _, v := range providers.AsArray().All {
			urn, ref, err := reference(v)
			if err != nil {
				return nil, err
			}
			typ := resource.URN(urn).Type()
			_, _, pkg, diags := pcl.DecomposeToken(string(typ), hcl.Range{})
			contract.Assertf(!diags.HasErrors(), "invalid token format from URN %s: %s", urn, typ)
			psopt[pkg] = ref
		}
	default:
		return nil, errors.New(
			"providers must be an array of provider objects or a map of provider name to provider objects")
	}
	return psopt, nil
}

func unwrapResource(value property.Value) (string, property.Value, error) {
	if !value.IsMap() {
		return "", property.Value{}, errors.New("expected resource object")
	}
	obj := value.AsMap()
	urnVal, ok := obj.GetOk("urn")
	if !ok || urnVal.IsNull() || urnVal.IsComputed() || !urnVal.IsString() {
		return "", property.Value{}, errors.New("expected resource object with known urn property")
	}
	idVal, ok := obj.GetOk("id")
	if !ok || idVal.IsNull() {
		return "", property.Value{}, errors.New("expected resource object with id property of type string")
	}

	if !idVal.IsComputed() && !idVal.IsString() {
		return "", property.Value{}, errors.New("expected resource object with id property of type string")
	}

	return urnVal.AsString(), idVal, nil
}

func collapseResourceReferences(value property.Value) property.Value {
	switch {
	case value.IsArray():
		array := value.AsArray()
		collapsed := make([]property.Value, array.Len())
		for i, elem := range array.All {
			collapsed[i] = collapseResourceReferences(elem)
		}
		return property.WithGoValue(value, collapsed)
	case value.IsMap():
		obj := value.AsMap()
		collapsed := make(map[string]property.Value, obj.Len())
		for key, elem := range obj.All {
			collapsed[key] = collapseResourceReferences(elem)
		}

		// Resource references are expanded into resource-shaped objects for evaluation.
		// Collapse these back before marshaling register requests.
		urn, hasURN := collapsed["urn"]
		id, hasID := collapsed["id"]
		typ, hasType := collapsed["__type"]
		if hasURN && hasID && hasType && urn.IsString() && typ.IsString() {
			ref := property.ResourceReference{
				URN: resource.URN(urn.AsString()),
				ID:  id,
			}
			// A dependency on only the referenced resource adds nothing to the reference itself.
			if d := value.Dependencies(); len(d) == 1 && d[0] == ref.URN {
				value = value.WithDependencies(nil)
			}
			return property.WithGoValue(value, ref)
		}
		return property.WithGoValue(value, collapsed)
	default:
		return value
	}
}

func (i *Interpreter) registerResource(ctx context.Context, res *pcl.Resource) error {
	lexicalBaseName := res.Name()
	logicalBaseName := i.effectiveName(res.LogicalName())
	if res.Options == nil || res.Options.Range == nil {
		result, err := i.registerResourceWith(ctx, res, i.evalContext, logicalBaseName)
		if err != nil {
			return err
		}
		i.evalContext.SetVariable(lexicalBaseName, result)
		return nil
	}

	rangeValue, poison, diags := i.evalContext.Evaluate(res.Options.Range)
	if poison != nil {
		i.evalContext.SetVariable(lexicalBaseName, makePoisonValue(*poison))
	}
	if diags.HasErrors() {
		return diags
	}

	// If the range is computed we just have to skip this resource
	if rangeValue.IsComputed() {
		return nil
	}

	if rangeValue.IsBool() {
		if !rangeValue.AsBool() {
			return nil
		}
		result, err := i.registerResourceWith(ctx, res, i.evalContext, logicalBaseName)
		if err != nil {
			return err
		}
		i.evalContext.SetVariable(lexicalBaseName, result)
		return nil
	}

	makeRangeCtx := func(key cty.Value, value cty.Value) *EvalContext {
		rangeCtx := i.evalContext.NewChild()
		rangeVars := map[string]cty.Value{"value": value}
		if key != cty.NilVal {
			rangeVars["key"] = key
		}
		rangeCtx.SetVariable("range", cty.ObjectVal(rangeVars))
		return rangeCtx
	}

	registerMany := func(items []struct {
		suffix  string
		evalCtx *EvalContext
	},
	) error {
		results := make([]cty.Value, 0, len(items))
		for _, item := range items {
			name := fmt.Sprintf("%s-%s", logicalBaseName, item.suffix)
			result, err := i.registerResourceWith(ctx, res, item.evalCtx, name)
			if err != nil {
				return err
			}
			results = append(results, result)
		}
		if len(results) == 0 {
			i.evalContext.SetVariable(lexicalBaseName, cty.ListValEmpty(cty.DynamicPseudoType))
			return nil
		}
		i.evalContext.SetVariable(lexicalBaseName, cty.ListVal(results))
		return nil
	}

	if rangeValue.IsNumber() {
		count := max(int(rangeValue.AsNumber()), 0)
		items := make([]struct {
			suffix  string
			evalCtx *EvalContext
		}, 0, count)
		for idx := range count {
			idxVal := cty.NumberIntVal(int64(idx))
			items = append(items, struct {
				suffix  string
				evalCtx *EvalContext
			}{
				suffix:  strconv.Itoa(idx),
				evalCtx: makeRangeCtx(cty.NilVal, idxVal),
			})
		}
		return registerMany(items)
	}

	if rangeValue.IsArray() {
		values := rangeValue.AsArray()
		items := make([]struct {
			suffix  string
			evalCtx *EvalContext
		}, 0, values.Len())
		for idx, v := range values.All {
			val, err := propertyValueToCty(ctx, i.getResource, v)
			if err != nil {
				return err
			}
			items = append(items, struct {
				suffix  string
				evalCtx *EvalContext
			}{
				suffix:  strconv.Itoa(idx),
				evalCtx: makeRangeCtx(cty.NumberIntVal(int64(idx)), val),
			})
		}
		return registerMany(items)
	}

	if rangeValue.IsMap() {
		values := rangeValue.AsMap()
		resultMap := make(map[string]cty.Value, values.Len())
		for key, value := range values.AllStable {
			val, err := propertyValueToCty(ctx, i.getResource, value)
			if err != nil {
				return err
			}
			name := fmt.Sprintf("%s-%s", logicalBaseName, key)
			result, err := i.registerResourceWith(ctx, res, makeRangeCtx(cty.StringVal(key), val), name)
			if err != nil {
				return err
			}
			resultMap[key] = result
		}
		if len(resultMap) == 0 {
			i.evalContext.SetVariable(lexicalBaseName, cty.EmptyObjectVal)
		} else {
			i.evalContext.SetVariable(lexicalBaseName, cty.ObjectVal(resultMap))
		}
		return nil
	}

	return fmt.Errorf("unsupported range type for resource %s", res.Name())
}

func (i *Interpreter) registerResourceWith(
	ctx context.Context, res *pcl.Resource, evalCtx *EvalContext, logicalName string,
) (cty.Value, error) {
	token, _ := res.GetToken()
	schemaResource, err := i.lookupResource(ctx, token)
	if err != nil {
		return cty.NilVal, fmt.Errorf("lookup resource schema for token %s: %w", token, err)
	}
	if schemaResource != nil {
		token = schemaResource.Token
	}

	var properties []*schema.Property
	if schemaResource != nil {
		properties = schemaResource.InputProperties
	}

	inputs, poison, diags := evalCtx.EvaluateObject(res.Inputs, res.InputType, properties)
	if poison != nil {
		return makePoisonValue(*poison), nil
	}
	if diags.HasErrors() {
		return cty.NilVal, diags
	}

	obj := propertyrpc.Marshal(inputs)
	custom := true
	if schemaResource != nil {
		custom = !schemaResource.IsComponent
	}

	dependencies := []string{}
	propertyDependencies := map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies{}
	for key, val := range inputs.All {
		deps := castSliceToString(allDependencies(val))
		if len(deps) > 0 {
			dependencies = append(dependencies, deps...)
			propertyDependencies[key] = &pulumirpc.RegisterResourceRequest_PropertyDependencies{
				Urns: deps,
			}
		}
	}

	request := &pulumirpc.RegisterResourceRequest{
		Type:                    token,
		Name:                    logicalName,
		Custom:                  custom,
		Remote:                  !custom,
		Object:                  obj,
		PropertyDependencies:    propertyDependencies,
		Dependencies:            dependencies,
		AcceptSecrets:           true,
		AcceptResources:         true,
		AcceptsByteString:       true,
		SupportsResultReporting: true,
		SnippetId:               i.snippetID,
	}
	request.PackageRef = i.getPackageRefFromToken(token)

	if res.Options != nil {
		if res.Options.AdditionalSecretOutputs != nil {
			additionalSecretOutputs, poison, diags := evalCtx.Evaluate(res.Options.AdditionalSecretOutputs)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !additionalSecretOutputs.IsNull() && !additionalSecretOutputs.IsComputed() {
				if !additionalSecretOutputs.IsArray() {
					return cty.NilVal, errors.New("additionalSecretOutputs must be an array of strings")
				}
				var additionalSecretOutputKeys []string
				for _, v := range additionalSecretOutputs.AsArray().All {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return cty.NilVal, errors.New("additionalSecretOutputs must be an array of strings")
					}
					additionalSecretOutputKeys = append(additionalSecretOutputKeys, v.AsString())
				}
				request.AdditionalSecretOutputs = additionalSecretOutputKeys
			}
		}
		if res.Options.Aliases != nil {
			aliases, poison, diags := evalCtx.Evaluate(res.Options.Aliases)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !aliases.IsNull() && !aliases.IsComputed() {
				if !aliases.IsArray() {
					return cty.NilVal, errors.New("aliases must be an array of strings or alias objects")
				}
				var aliasOpts []*pulumirpc.Alias
				// Translate each alias expression (either string or object) into an rpc alias object
				for _, alias := range aliases.AsArray().All {
					if alias.IsString() {
						aliasOpts = append(aliasOpts, &pulumirpc.Alias{
							Alias: &pulumirpc.Alias_Urn{
								Urn: alias.AsString(),
							},
						})
					} else if alias.IsMap() {
						obj := alias.AsMap()
						aliasOpt := &pulumirpc.Alias_Spec{}

						setString := func(field string, setter func(string)) error {
							attr, ok := obj.GetOk(field)
							if ok && !attr.IsNull() && !attr.IsComputed() {
								if !attr.IsString() {
									return fmt.Errorf("%s must be a string", field)
								}
								setter(attr.AsString())
							}
							return nil
						}

						err := setString("name", func(name string) { aliasOpt.Name = name })
						if err != nil {
							return cty.NilVal, err
						}
						err = setString("type", func(typ string) { aliasOpt.Type = typ })
						if err != nil {
							return cty.NilVal, err
						}

						noParent, ok := obj.GetOk("noParent")
						if ok && !noParent.IsNull() && !noParent.IsComputed() {
							if !noParent.IsBool() {
								return cty.NilVal, errors.New("noParent must be a boolean")
							}
							aliasOpt.Parent = &pulumirpc.Alias_Spec_NoParent{
								NoParent: noParent.AsBool(),
							}
						}

						parent, ok := obj.GetOk("parent")
						if ok && !parent.IsNull() && !parent.IsComputed() {
							urn, _, err := unwrapResource(parent)
							if err != nil {
								return cty.NilVal, fmt.Errorf("parent: %w", err)
							}
							aliasOpt.Parent = &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: urn,
							}
						}

						aliasOpts = append(aliasOpts, &pulumirpc.Alias{
							Alias: &pulumirpc.Alias_Spec_{
								Spec: aliasOpt,
							},
						})
					} else {
						return cty.NilVal, errors.New("aliases must be an array of strings or alias objects")
					}
				}
				request.Aliases = aliasOpts
			}
		}
		if res.Options.DependsOn != nil {
			dependsOn, poison, diags := evalCtx.Evaluate(res.Options.DependsOn)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !dependsOn.IsNull() && !dependsOn.IsComputed() {
				if !dependsOn.IsArray() {
					return cty.NilVal, errors.New("dependsOn must be an array of resource objects")
				}
				var dependsOnUrns []string
				for _, v := range dependsOn.AsArray().All {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					urn, _, err := unwrapResource(v)
					if err != nil {
						return cty.NilVal, fmt.Errorf("dependsOn: %w", err)
					}
					dependsOnUrns = append(dependsOnUrns, urn)
				}
				request.Dependencies = append(request.Dependencies, dependsOnUrns...)
			}
		}
		if res.Options.EnvVarMappings != nil {
			envVars, poison, diags := evalCtx.Evaluate(res.Options.EnvVarMappings)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !envVars.IsNull() && !envVars.IsComputed() {
				if !envVars.IsMap() {
					return cty.NilVal, errors.New(
						"envVarMappings must be an object mapping environment variable names to input property keys")
				}
				envVarMappings := map[string]string{}
				for envVar, propKey := range envVars.AsMap().All {
					if propKey.IsNull() || propKey.IsComputed() || !propKey.IsString() {
						return cty.NilVal, errors.New(
							"envVarMappings must be an object mapping environment variable names to input property keys")
					}
					envVarMappings[envVar] = propKey.AsString()
				}
				request.EnvVarMappings = envVarMappings
			}
		}
		if res.Options.ImportID != nil {
			importID, poison, diags := evalCtx.Evaluate(res.Options.ImportID)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !importID.IsNull() && !importID.IsComputed() {
				if !importID.IsString() {
					return cty.NilVal, errors.New("import must be a string")
				}
				request.ImportId = importID.AsString()
			}
		}
		if res.Options.IgnoreChanges != nil {
			ignoreChanges, poison, diags := evalCtx.Evaluate(res.Options.IgnoreChanges)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !ignoreChanges.IsNull() && !ignoreChanges.IsComputed() {
				if !ignoreChanges.IsArray() {
					return cty.NilVal, errors.New("ignoreChanges must be an array of strings")
				}
				icopt := []string{}
				for _, v := range ignoreChanges.AsArray().All {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return cty.NilVal, errors.New("ignoreChanges must be an array of strings")
					}
					icopt = append(icopt, v.AsString())
				}
				request.IgnoreChanges = icopt
			}
		}
		if res.Options.Protect != nil {
			protect, poison, diags := evalCtx.Evaluate(res.Options.Protect)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !protect.IsComputed() {
				var popt *bool
				if protect.IsBool() {
					popt = new(protect.AsBool())
				} else if !protect.IsNull() {
					return cty.NilVal, errors.New("protect must be a boolean or null")
				}
				request.Protect = popt
			}
		}
		if res.Options.ReplaceWith != nil {
			replaceWith, poison, diags := evalCtx.Evaluate(res.Options.ReplaceWith)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !replaceWith.IsNull() && !replaceWith.IsComputed() {
				if !replaceWith.IsArray() {
					return cty.NilVal, errors.New("replaceWith must be an array of resources")
				}
				var rwopt []string
				for _, v := range replaceWith.AsArray().All {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					urn, _, err := unwrapResource(v)
					if err != nil {
						return cty.NilVal, fmt.Errorf("replaceWith: %w", err)
					}
					rwopt = append(rwopt, urn)
				}
				request.ReplaceWith = rwopt
			}
		}
		if res.Options.ReplaceOnChanges != nil {
			replaceOnChanges, poison, diags := evalCtx.Evaluate(res.Options.ReplaceOnChanges)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !replaceOnChanges.IsNull() && !replaceOnChanges.IsComputed() {
				if !replaceOnChanges.IsArray() {
					return cty.NilVal, errors.New("replaceOnChanges must be an array of strings")
				}
				rocopt := []string{}
				for _, v := range replaceOnChanges.AsArray().All {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return cty.NilVal, errors.New("replaceOnChanges must be an array of strings")
					}
					rocopt = append(rocopt, v.AsString())
				}
				request.ReplaceOnChanges = rocopt
			}
		}
		if res.Options.ReplacementTrigger != nil {
			replacement, poison, diags := evalCtx.Evaluate(res.Options.ReplacementTrigger)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			request.ReplacementTrigger = propertyrpc.MarshalValue(replacement)
		}
		if res.Options.RetainOnDelete != nil {
			retain, poison, diags := evalCtx.Evaluate(res.Options.RetainOnDelete)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !retain.IsNull() && !retain.IsComputed() {
				var retainOnDelete *bool
				if retain.IsBool() {
					retainOnDelete = new(retain.AsBool())
				} else {
					return cty.NilVal, errors.New("retainOnDelete must be a boolean or null")
				}
				request.RetainOnDelete = retainOnDelete
			}
		}
		if res.Options.Version != nil {
			version, poison, diags := evalCtx.Evaluate(res.Options.Version)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !version.IsNull() && !version.IsComputed() {
				if !version.IsString() {
					return cty.NilVal, errors.New("version must be a string")
				}
				request.Version = version.AsString()
			}
		}
		if res.Options.CustomTimeouts != nil {
			timeouts, poison, diags := evalCtx.Evaluate(res.Options.CustomTimeouts)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !timeouts.IsNull() && !timeouts.IsComputed() {
				if !timeouts.IsMap() {
					return cty.NilVal, errors.New("customTimeouts must be an object")
				}
				timeoutValues := map[string]string{}
				for k, v := range timeouts.AsMap().All {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return cty.NilVal, fmt.Errorf("customTimeouts.%s must be a string", k)
					}
					timeoutValues[k] = v.AsString()
				}
				request.CustomTimeouts = &pulumirpc.RegisterResourceRequest_CustomTimeouts{
					Create: timeoutValues["create"],
					Update: timeoutValues["update"],
					Delete: timeoutValues["delete"],
					Read:   timeoutValues["read"],
				}
			}
		}
		if res.Options.DeleteBeforeReplace != nil {
			dbr, poison, diags := evalCtx.Evaluate(res.Options.DeleteBeforeReplace)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !dbr.IsNull() && !dbr.IsComputed() {
				if dbr.IsBool() {
					request.DeleteBeforeReplace = dbr.AsBool()
					request.DeleteBeforeReplaceDefined = true
				} else if !dbr.IsNull() {
					return cty.NilVal, errors.New("deleteBeforeReplace must be a boolean or null")
				}
			}
		}
		if res.Options.DeletedWith != nil {
			deletedWith, poison, diags := evalCtx.Evaluate(res.Options.DeletedWith)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !deletedWith.IsNull() && !deletedWith.IsComputed() {
				urn, _, err := unwrapResource(deletedWith)
				if err != nil {
					return cty.NilVal, fmt.Errorf("deletedWith: %w", err)
				}
				request.DeletedWith = urn
			}
		}
		if res.Options.PluginDownloadURL != nil {
			downloadURL, poison, diags := evalCtx.Evaluate(res.Options.PluginDownloadURL)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !downloadURL.IsNull() && !downloadURL.IsComputed() {
				if !downloadURL.IsString() {
					return cty.NilVal, errors.New("pluginDownloadURL must be a string")
				}
				request.PluginDownloadURL = downloadURL.AsString()
			}
		}
		if res.Options.Parent != nil {
			parent, poison, diags := evalCtx.Evaluate(res.Options.Parent)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !parent.IsNull() && !parent.IsComputed() {
				urn, _, err := unwrapResource(parent)
				if err != nil {
					return cty.NilVal, fmt.Errorf("parent: %w", err)
				}
				request.Parent = urn
			}
		}
		if res.Options.Provider != nil {
			provider, poison, diags := evalCtx.Evaluate(res.Options.Provider)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !provider.IsNull() && !provider.IsComputed() {
				urn, id, err := unwrapResource(provider)
				if err != nil {
					return cty.NilVal, fmt.Errorf("provider: %w", err)
				}
				var idstr string
				if id.IsString() {
					idstr = id.AsString()
				} else {
					idstr = plugin.UnknownStringValue
				}
				request.Provider = fmt.Sprintf("%s::%s", urn, idstr)
			}
		}
		if res.Options.Providers != nil {
			providers, poison, diags := evalCtx.Evaluate(res.Options.Providers)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !providers.IsNull() && !providers.IsComputed() {
				psopt, err := providerReferences(providers)
				if err != nil {
					return cty.NilVal, err
				}
				request.Providers = psopt
			}
		}
		if res.Options.HideDiffs != nil {
			hideDiffs, poison, diags := evalCtx.Evaluate(res.Options.HideDiffs)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !hideDiffs.IsNull() && !hideDiffs.IsComputed() {
				if !hideDiffs.IsArray() {
					return cty.NilVal, errors.New("hideDiffs must be an array of strings")
				}
				hdopt := []string{}
				for _, v := range hideDiffs.AsArray().All {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return cty.NilVal, errors.New("hideDiffs must be an array of strings")
					}
					hdopt = append(hdopt, v.AsString())
				}
				request.HideDiffs = hdopt
			}
		}

		// Process hooks - register command hooks and build the hooks binding
		if res.Options.Hooks != nil {
			hooksVal, poison, diags := evalCtx.Evaluate(res.Options.Hooks)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !hooksVal.IsNull() && !hooksVal.IsComputed() {
				if !hooksVal.IsMap() {
					return cty.NilVal, errors.New("hooks must be an object mapping hook types to command lists")
				}
				binding := &pulumirpc.RegisterResourceRequest_ResourceHooksBinding{}
				for hookType, commandLists := range hooksVal.AsMap().All {
					if commandLists.IsNull() || commandLists.IsComputed() {
						continue
					}
					if !commandLists.IsArray() {
						return cty.NilVal, fmt.Errorf("hooks.%s must be an array of hooks", hookType)
					}
					var hookNames []string
					for idx, hookVal := range commandLists.AsArray().All {
						if hookVal.IsNull() || hookVal.IsComputed() {
							continue
						}
						if !hookVal.IsString() {
							return cty.NilVal, fmt.Errorf("hooks.%s[%d] must be a reference to a named hook block", hookType, idx)
						}
						hookNames = append(hookNames, hookVal.AsString())
					}
					switch hookType {
					case "beforeCreate":
						binding.BeforeCreate = append(binding.BeforeCreate, hookNames...)
					case "afterCreate":
						binding.AfterCreate = append(binding.AfterCreate, hookNames...)
					case "beforeUpdate":
						binding.BeforeUpdate = append(binding.BeforeUpdate, hookNames...)
					case "afterUpdate":
						binding.AfterUpdate = append(binding.AfterUpdate, hookNames...)
					case "beforeDelete":
						binding.BeforeDelete = append(binding.BeforeDelete, hookNames...)
					case "afterDelete":
						binding.AfterDelete = append(binding.AfterDelete, hookNames...)
					case "onError":
						binding.OnError = append(binding.OnError, hookNames...)
					default:
						return cty.NilVal, fmt.Errorf("invalid hook type: %s", hookType)
					}
				}
				if len(binding.BeforeCreate)+len(binding.AfterCreate)+
					len(binding.BeforeUpdate)+len(binding.AfterUpdate)+
					len(binding.BeforeDelete)+len(binding.AfterDelete)+
					len(binding.OnError) > 0 {
					request.Hooks = binding
				}
			}
		}
	}

	// Add schema-based replaceOnChanges paths and additionalSecretOutput keys.
	if schemaResource != nil {
		schemaReplaceOnChanges, _ := schemaResource.ReplaceOnChanges()
		request.ReplaceOnChanges = append(
			request.ReplaceOnChanges,
			schema.PropertyListJoinToString(schemaReplaceOnChanges, func(s string) string { return s })...)

		var additionalSecretOutputs []string
		for _, prop := range schemaResource.Properties {
			if prop.Secret {
				additionalSecretOutputs = append(additionalSecretOutputs, prop.Name)
			}
		}
		request.AdditionalSecretOutputs = append(request.AdditionalSecretOutputs, additionalSecretOutputs...)
	}

	// Default parent to the stack if not specified
	if request.Parent == "" {
		request.Parent = i.stackURN
	}

	resp, err := i.monitor.RegisterResource(ctx, request)
	if err != nil {
		return cty.NilVal, err
	}
	if resp.GetResult() != pulumirpc.Result_SUCCESS {
		// This resource failed to register but we might be running with --continue-on-error so mark this resource as
		// poisoned so that any downstream resources that depend on it will also be marked as poisoned and skip
		// registering while allowing the rest of the graph to continue registering.
		return makePoisonValue(res.Name()), nil
	}

	outputs, err := propertyrpc.Unmarshal(resp.Object)
	if err != nil {
		return cty.NilVal, err
	}

	unknown := custom && !i.info.DryRun && resp.GetUnknown()

	// During previews a created resource has no ID yet, and a skipped create never gets
	// one; represent it as unknown rather than a known empty string so it can't be
	// observed as a real value.
	if id := resp.GetId(); id == "" && (i.info.DryRun || unknown) {
		outputs = outputs.Set("id", property.New(property.Computed))
	} else {
		outputs = outputs.Set("id", property.New(id))
	}
	outputs = outputs.
		Set("urn", property.New(resp.GetUrn())).
		Set("__name", property.New(request.Name)).
		Set("__type", property.New(request.Type))

	// Ensure every schema-declared output property is present, recursing into nested object
	// types so that programs which traverse into an optional inner field see a typed null
	// rather than triggering an HCL "unsupported attribute" error.
	// - preview or skipped create: unknown/computed
	// - update: explicit null
	if schemaResource != nil {
		outputs = fillSchemaOutputs(outputs, schemaResource.Properties, i.info.DryRun || unknown)
	}

	return propertyValueToCty(ctx, i.getResource, property.New(outputs).
		WithDependencies([]resource.URN{resource.URN(resp.GetUrn())}))
}

func (i *Interpreter) registerReadResource(ctx context.Context, res *pcl.ReadResource) error {
	logicalName := i.effectiveName(res.LogicalName())
	token, _ := res.GetToken()
	schemaResource, err := i.lookupResource(ctx, token)
	if err != nil {
		return fmt.Errorf("lookup resource schema for token %s: %w", token, err)
	}
	if schemaResource != nil {
		token = schemaResource.Token
	}

	// The bindable inputs are "id" plus the schema's state inputs.
	properties := []*schema.Property{{Name: "id", Type: schema.StringType}}
	if schemaResource != nil && schemaResource.StateInputs != nil {
		properties = append(properties, schemaResource.StateInputs.Properties...)
	}

	inputs, poison, diags := i.evalContext.EvaluateObject(res.Inputs, res.InputType, properties)
	if poison != nil {
		i.evalContext.SetVariable(res.Name(), makePoisonValue(*poison))
		return nil
	}
	if diags.HasErrors() {
		return diags
	}

	idVal, hasID := inputs.GetOk("id")
	if !hasID {
		return fmt.Errorf("read resource %s is missing the id attribute", res.Name())
	}
	inputs = inputs.Delete("id")

	obj := propertyrpc.Marshal(inputs)

	dependencies := []string{}
	for _, val := range allDependencies(property.New(inputs)) {
		dependencies = append(dependencies, string(val))
	}

	idDeps := allDependencies(idVal)
	for _, dep := range idDeps {
		dependencies = append(dependencies, string(dep))
	}
	idStr := plugin.UnknownStringValue
	if idVal.IsString() {
		idStr = idVal.AsString()
	}

	if res.Options != nil && res.Options.DependsOn != nil {
		dependsOn, poison, diags := i.evalContext.Evaluate(res.Options.DependsOn)
		if poison != nil {
			i.evalContext.SetVariable(res.Name(), makePoisonValue(*poison))
			return nil
		}
		if diags.HasErrors() {
			return diags
		}
		if !dependsOn.IsNull() && !dependsOn.IsComputed() {
			if !dependsOn.IsArray() {
				return errors.New("dependsOn must be an array of resource objects")
			}
			for _, v := range dependsOn.AsArray().All {
				if v.IsNull() || v.IsComputed() {
					continue
				}
				urn, _, err := unwrapResource(v)
				if err != nil {
					return fmt.Errorf("dependsOn: %w", err)
				}
				dependencies = append(dependencies, urn)
			}
		}
	}

	request := &pulumirpc.ReadResourceRequest{
		Id:                idStr,
		Type:              token,
		Name:              logicalName,
		Parent:            i.stackURN,
		Properties:        obj,
		Dependencies:      dependencies,
		AcceptSecrets:     true,
		AcceptResources:   true,
		AcceptsByteString: true,
	}
	request.PackageRef = i.getPackageRefFromToken(token)

	resp, err := i.monitor.ReadResource(ctx, request)
	if err != nil {
		return err
	}

	outputs, err := propertyrpc.Unmarshal(resp.GetProperties())
	if err != nil {
		return err
	}

	outputs = outputs.
		Set("id", property.New(request.Id)).
		Set("urn", property.New(resp.GetUrn())).
		Set("__name", property.New(logicalName)).
		Set("__type", property.New(token))

	if schemaResource != nil {
		// A skipped read reports Unknown=true; treat outputs as unknown so dependents propagate
		// unknowns instead of seeing empty values as real.
		outputs = fillSchemaOutputs(outputs, schemaResource.Properties, i.info.DryRun || resp.GetUnknown())
	}

	result := property.New(outputs).WithDependencies([]urn.URN{resource.URN(resp.GetUrn())})

	ctyResult, err := propertyValueToCty(ctx, i.getResource, result)
	if err != nil {
		return err
	}
	i.evalContext.SetVariable(res.Name(), ctyResult)
	return nil
}

func (i *Interpreter) registerComponent(ctx context.Context, component *pcl.Component) hcl.Diagnostics {
	inputs, poison, diags := i.evalContext.EvaluateObject(component.Inputs, component.InputType, nil)
	if poison != nil {
		i.evalContext.SetVariable(component.Name(), makePoisonValue(*poison))
		return nil
	}
	if diags.HasErrors() {
		return diags
	}

	obj := propertyrpc.Marshal(inputs)

	dependencies := []string{}
	propertyDependencies := map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies{}
	for key, val := range inputs.All {
		deps := castSliceToString(allDependencies(val))
		if len(deps) > 0 {
			dependencies = append(dependencies, deps...)
			propertyDependencies[key] = &pulumirpc.RegisterResourceRequest_PropertyDependencies{
				Urns: deps,
			}
		}
	}

	componentName := i.effectiveName(component.LogicalName())
	// A component declared without a source names an existing component resource by its type token; otherwise the
	// token is synthesised from the declaration.
	componentType := component.Token
	if componentType == "" {
		componentType = "components:index:" + component.DeclarationName()
	}
	request := &pulumirpc.RegisterResourceRequest{
		Type:                 componentType,
		Name:                 componentName,
		Custom:               false,
		Object:               obj,
		PropertyDependencies: propertyDependencies,
		Dependencies:         dependencies,
		AcceptSecrets:        true,
		AcceptResources:      true,
		AcceptsByteString:    true,
		Parent:               i.stackURN,
		SnippetId:            i.snippetID,
	}
	if component.Options != nil && component.Options.Parent != nil {
		parent, poison, diags := i.evalContext.Evaluate(component.Options.Parent)
		if poison != nil {
			i.evalContext.SetVariable(component.Name(), makePoisonValue(*poison))
			return nil
		}
		if diags.HasErrors() {
			return diags
		}
		if !parent.IsNull() && !parent.IsComputed() {
			urn, _, err := unwrapResource(parent)
			if err != nil {
				return hcl.Diagnostics{{
					Severity: hcl.DiagError,
					Summary:  "Failed to unwrap parent resource",
					Detail:   err.Error(),
				}}
			}
			request.Parent = urn
		}
	}

	// Providers given to a component apply to everything the component registers; the engine applies
	// them to child resources and to invokes parented to the component.
	if component.Options != nil && component.Options.Providers != nil {
		providers, poison, diags := i.evalContext.Evaluate(component.Options.Providers)
		if poison != nil {
			i.evalContext.SetVariable(component.Name(), makePoisonValue(*poison))
			return nil
		}
		if diags.HasErrors() {
			return diags
		}
		if !providers.IsNull() && !providers.IsComputed() {
			psopt, err := providerReferences(providers)
			if err != nil {
				return hcl.Diagnostics{{
					Severity: hcl.DiagError,
					Summary:  "Failed to evaluate component providers",
					Detail:   err.Error(),
				}}
			}
			request.Providers = psopt
		}
	}

	resp, err := i.monitor.RegisterResource(ctx, request)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to register component",
			Detail:   err.Error(),
		}}
	}
	if resp.GetResult() != pulumirpc.Result_SUCCESS {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Component registration failed",
			Detail:   resp.GetResult().String(),
		}}
	}

	// A component declared without a source has no inner program to interpret and no outputs, but its variable
	// must still be published so that children can parent to it.
	if component.Program == nil {
		return i.setComponentVariable(ctx, component, resp, property.Map{})
	}

	componentInterpreter := &Interpreter{
		program:     component.Program,
		info:        i.info,
		monitor:     i.monitor,
		engine:      i.engine,
		loader:      i.loader,
		stackURN:    resp.GetUrn(),
		namePrefix:  componentName,
		packageRefs: i.packageRefs,
	}
	// The eval context must call back into the component's own interpreter so that invokes written
	// in the component are parented to the component.
	componentInterpreter.evalContext = NewEvalContext(
		i.info.WorkingDir,
		i.info.RootDirectory,
		i.info.Organization,
		i.info.Project,
		i.info.Stack,
		i.lookupResource,
		i.lookupFunction,
		i.getResource,
		componentInterpreter.invoke,
		componentInterpreter.call,
	)

	for k, v := range inputs.All {
		if err := componentInterpreter.setVariable(ctx, k, v); err != nil {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  "Failed to set component input " + k,
				Detail:   err.Error(),
			}}
		}
	}

	componentOutputs, err := componentInterpreter.executeProgramNodes(ctx)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to execute component program nodes",
			Detail:   err.Error(),
		}}
	}
	for key, val := range componentOutputs.All {
		componentOutputs = componentOutputs.Set(key, collapseResourceReferences(val))
	}

	outObj := propertyrpc.Marshal(componentOutputs)
	_, err = i.monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     resp.GetUrn(),
		Outputs: outObj,
	})
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to register component outputs",
			Detail:   err.Error(),
		}}
	}

	return i.setComponentVariable(ctx, component, resp, componentOutputs)
}

// setComponentVariable publishes a registered component as a variable so that later nodes can reference it.
func (i *Interpreter) setComponentVariable(
	ctx context.Context,
	component *pcl.Component,
	resp *pulumirpc.RegisterResourceResponse,
	componentOutputs property.Map,
) hcl.Diagnostics {
	result := property.New(componentOutputs.
		Set("id", property.New(resp.GetId())).
		Set("urn", property.New(resp.GetUrn()))).
		WithDependencies([]resource.URN{resource.URN(resp.GetUrn())})

	if err := i.setVariable(ctx, component.Name(), result); err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to set component output" + component.Name(),
			Detail:   err.Error(),
		}}
	}
	return nil
}

func castSliceToString[T ~string](arr []T) []string {
	if arr == nil {
		return nil
	}
	dst := make([]string, len(arr))
	for i, v := range arr {
		dst[i] = string(v)
	}
	return dst
}

func (i *Interpreter) registerStackOutputs(ctx context.Context, outputs property.Map) error {
	if i.stackURN == "" {
		return errors.New("missing stack URN")
	}
	collapsedOutputs := make(map[string]property.Value, outputs.Len())
	for key, val := range outputs.All {
		collapsedOutputs[key] = collapseResourceReferences(val)
	}

	obj := propertyrpc.Marshal(property.NewMap(collapsedOutputs))
	_, err := i.monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     i.stackURN,
		Outputs: obj,
	})
	return err
}

func (i *Interpreter) setVariable(ctx context.Context, name string, value property.Value) error {
	ctyValue, err := propertyValueToCty(ctx, i.getResource, value)
	if err != nil {
		return err
	}
	i.evalContext.SetVariable(name, ctyValue)
	return nil
}

func parseConfigPropertyValue(raw string, typ model.Type) (property.Value, hcl.Diagnostics) {
	ctyValue, diags := parseConfigValue(raw, typ)
	if diags.HasErrors() {
		return property.Value{}, diags
	}
	pv, err := ctyToPropertyValue(ctyValue)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  err.Error(),
		})
	}
	return pv, diags
}

func getStackOutput(stackRef property.Value, outputName string) (property.Value, error) {
	if !stackRef.IsMap() {
		return property.New(property.Null), nil
	}

	obj := stackRef.AsMap()
	outputs, ok := obj.GetOk("outputs")
	if !ok {
		return property.New(property.Null), nil
	}
	if !outputs.IsMap() {
		return property.New(property.Null), nil
	}

	outMap := outputs.AsMap()
	output, ok := outMap.GetOk(outputName)
	if !ok {
		return property.New(property.Null), nil
	}

	secretByName := false
	if secretNames, ok := obj.GetOk("secretOutputNames"); ok {
		if secretNames.IsArray() {
			for _, name := range secretNames.AsArray().All {
				if name.IsString() && name.AsString() == outputName {
					secretByName = true
					break
				}
			}
		}
	}

	return output.WithSecret(secretByName || output.Secret()), nil
}
