package pulumi

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
)

type languageRuntimeServer struct {
	fn      RunFunc
	address string
	cancel  chan bool
	done    chan error

	requests sync.WaitGroup

	fnErrors chan error
}

func startLanguageRuntimeServer(fn RunFunc) (*languageRuntimeServer, error) {
	s := &languageRuntimeServer{
		fn:       fn,
		cancel:   make(chan bool),
		fnErrors: make(chan error),
	}

	port, done, err := rpcutil.Serve(0, s.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, s)
			return nil
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	s.address, s.done = fmt.Sprintf("127.0.0.1:%d", port), done
	return s, nil
}

func (s *languageRuntimeServer) Close() error {
	s.cancel <- true
	close(s.cancel)
	err := <-s.done
	go func() {
		s.requests.Wait()
		close(s.fnErrors)
	}()
	return err
}

func (s *languageRuntimeServer) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (s *languageRuntimeServer) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	s.requests.Add(1)

	var engineAddress string
	if len(req.Args) > 0 {
		engineAddress = req.Args[0]
	}
	runInfo := RunInfo{
		EngineAddr:  engineAddress,
		MonitorAddr: req.GetMonitorAddress(),
		Config:      req.GetConfig(),
		Project:     req.GetProject(),
		Stack:       req.GetStack(),
		Parallel:    int(req.GetParallel()),
	}

	pulumiCtx, err := NewContext(ctx, runInfo)
	if err != nil {
		s.requests.Done()
		return nil, err
	}

	err = func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("panic executing callback: %v", r)
				}
			}
		}()

		return RunWithContext(pulumiCtx, s.fn)
	}()
	go func() {
		s.fnErrors <- err
		s.requests.Done()
	}()

	if err != nil {
		return &pulumirpc.RunResponse{Error: err.Error()}, nil
	}
	return &pulumirpc.RunResponse{}, nil
}

func (s *languageRuntimeServer) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: "1.0.0",
	}, nil
}

type UpOptions struct {
	Preview      bool
	Echo         bool
	DebugLogging bool
	Verbosity    int

	args []string
}

type Update struct {
	m sync.Mutex

	ctx     context.Context
	fn      RunFunc
	options *UpOptions

	stdoutWriter io.WriteCloser
	stderrWriter io.WriteCloser
	Stdout       io.ReadCloser
	Stderr       io.ReadCloser

	started  bool
	finished error

	languageRuntimeServer *languageRuntimeServer
	pulumiCommand         *exec.Cmd
}

func (u *Update) Start() error {
	u.m.Lock()
	defer u.m.Unlock()

	if u.started {
		return fmt.Errorf("update already started")
	}

	s, err := startLanguageRuntimeServer(u.fn)
	if err != nil {
		return fmt.Errorf("failed to start language runtime service: %v", err)
	}
	u.languageRuntimeServer = s

	var args []string
	if len(u.options.args) != 0 {
		args = u.options.args
	} else {
		if u.options.Preview {
			args = []string{"preview"}
		} else {
			args = []string{"up", "--skip-preview"}
		}

		isInteractive := u.options.Echo && cmdutil.Interactive()
		if u.options.Preview && isInteractive {
			args = append(args, "--yes")
		}
		if u.options.Verbosity > 0 {
			args = append(args, fmt.Sprintf("-v=%d", u.options.Verbosity), "--logtostderr")
		}
		if u.options.DebugLogging {
			args = append(args, "-d")
		}
	}
	args = append(args, "--client="+s.address)

	cmd := exec.Command("pulumi", args...)
	if u.options.Echo {
		contract.IgnoreClose(u.stdoutWriter)
		contract.IgnoreClose(u.stderrWriter)

		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = u.stdoutWriter
		cmd.Stderr = u.stderrWriter
	}
	if err := cmd.Start(); err != nil {
		contract.IgnoreClose(u.languageRuntimeServer)
		return err
	}
	u.pulumiCommand = cmd

	u.started = true
	return nil
}

func (u *Update) Wait() error {
	u.m.Lock()
	defer u.m.Unlock()

	if !u.started {
		return fmt.Errorf("update has not started")
	}
	if u.finished != nil {
		return u.finished
	}

	defer contract.IgnoreClose(u.stdoutWriter)
	defer contract.IgnoreClose(u.stderrWriter)

	cmdError := u.pulumiCommand.Wait()
	contract.IgnoreClose(u.languageRuntimeServer)

	var fnError error
	for err := range u.languageRuntimeServer.fnErrors {
		fnError = err
	}
	if fnError != nil {
		u.finished = fnError
		return fnError
	}

	u.finished = cmdError
	return cmdError
}

func (u *Update) Run() error {
	if err := u.Start(); err != nil {
		return err
	}
	return u.Wait()
}

func Up(ctx context.Context, fn RunFunc, options *UpOptions) *Update {
	if options == nil {
		options = &UpOptions{}
	}

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	return &Update{
		ctx:          ctx,
		fn:           fn,
		options:      options,
		stdoutWriter: stdoutWriter,
		stderrWriter: stderrWriter,
		Stdout:       stdoutReader,
		Stderr:       stderrReader,
	}
}

type PreviewOptions struct {
	Echo         bool
	DebugLogging bool
	Verbosity    int
}

func Preview(ctx context.Context, fn RunFunc, options *PreviewOptions) *Update {
	if options == nil {
		options = &PreviewOptions{}
	}
	return Up(ctx, fn, &UpOptions{
		Echo:         options.Echo,
		DebugLogging: options.DebugLogging,
		Verbosity:    options.Verbosity,
	})
}
