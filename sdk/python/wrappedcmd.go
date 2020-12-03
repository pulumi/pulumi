package python

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

// WrappedCmd wraps the key portions of exec.Cmd used
// to launch the python interpreter. Large portions of this
// are borrowed directly from exec.Cmd but tightly constrained
// to the surface area within use at Pulumi. This allows us to
// provide a consistent interface between exec.Cmd and
// os.StartProcess which is required on certain Windows environment
// due to a bug in Go - see https://github.com/golang/go/issues/42919
// for details.
type WrappedCmd interface {
	// CombinedOutput has the same semantics as exec.Cmd's CombinedOutput.
	// Depending on the platform and binary being run, it will either
	// directly use the exec.Cmd or transparently create a
	// os.StartProcess from the provided exec.Cmd.
	CombinedOutput() ([]byte, error)
	// Run has the same semantics as exec.Cmd's Run. Depending on the platform
	// and binary being run, it will either directly use the exec.Cmd or
	// transparently create a os.StartProcess from the provided exec.Cmd.
	Run() error
	// Args lists the arguments currently being used by the instance.
	Args() []string
	// Path provides the path to the command to execute.
	Path() string
}

// newWrappedCmd creates a new WrappedCmd from the provided exec.Cmd.
// It determines if the platform/binary requires using os.StartProcess
// instead.
func newWrappedCmd(cmd *exec.Cmd) (*wrappedCmd, error) {
	finfo, err := os.Lstat(cmd.Path)
	if err != nil {
		return nil, err
	}

	var useStartProcess bool
	if isReparsePoint(finfo) {
		useStartProcess = true
	}

	return &wrappedCmd{
		cmd:             cmd,
		useStartProcess: useStartProcess,
	}, nil
}

type wrappedCmd struct {
	cmd             *exec.Cmd
	stdout          *os.File
	stderr          *os.File
	useStartProcess bool
	captureStdout   io.Writer
	captureStderr   io.Writer
	closeAfterStart []io.Closer
	closeAfterWait  []io.Closer
	goroutines      []func() error
}

func (w *wrappedCmd) Args() []string {
	return w.cmd.Args
}

func closeDescriptors(toClose []io.Closer) {
	for _, f := range toClose {
		_ = f.Close()
	}
}

// Lot of this is borrowed from exec.Cmd Run but much more
// constrained to the surface area relevant to our use-case within
// our codebase.
func (w *wrappedCmd) Run() error {
	if !w.useStartProcess {
		return w.cmd.Run()
	}

	stdin, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	w.closeAfterStart = append(w.closeAfterStart, stdin)

	stdout := w.stdout
	if w.stdout == nil {
		stdout, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			closeDescriptors(w.closeAfterStart)
			return err
		}
		w.closeAfterStart = append(w.closeAfterStart, w.stdout)
	}

	stderr := w.stderr
	if w.stderr == nil {
		stderr, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			closeDescriptors(w.closeAfterStart)
			return err
		}
		w.closeAfterStart = append(w.closeAfterStart, w.stderr)
	}

	if w.captureStdout != nil {
		copyStream := func(dst io.Writer, src io.ReadCloser) func() error {
			return func() error {
				_, err := io.Copy(dst, src)
				_ = src.Close()
				return err
			}
		}

		pro, pwo, err := os.Pipe()
		if err != nil {
			closeDescriptors(w.closeAfterStart)
			return err
		}

		w.closeAfterStart = append(w.closeAfterStart, pwo)
		w.closeAfterWait = append(w.closeAfterWait, pro)
		w.goroutines = append(w.goroutines, copyStream(w.captureStdout, pro))
		stdout = pwo

		// This is important - we don't want to be writing to the same stream through
		// multiple goroutines. exec.Cmd does the same thing.
		if w.captureStderr != nil && interfaceEqual(w.captureStderr, w.captureStdout) {
			stderr = stdout
		} else {
			pre, pwe, err := os.Pipe()
			if err != nil {
				closeDescriptors(w.closeAfterStart)
				closeDescriptors(w.closeAfterWait)
				return err
			}

			w.closeAfterStart = append(w.closeAfterStart, pwe)
			w.closeAfterWait = append(w.closeAfterWait, pre)
			w.goroutines = append(w.goroutines, copyStream(w.captureStderr, pre))
			stderr = pwe
		}
	}

	env, err := w.env()
	if err != nil {
		return err
	}
	procAttr := &os.ProcAttr{
		Dir:   w.cmd.Dir,
		Env:   addCriticalEnv(dedupEnv(env)),
		Files: []*os.File{stdin, stdout, stderr},
	}

	args := w.cmd.Args
	if len(args) == 0 {
		args = []string{w.cmd.Path}
	}

	p, err := os.StartProcess(w.cmd.Path, args, procAttr)
	if err != nil {
		closeDescriptors(w.closeAfterStart)
		closeDescriptors(w.closeAfterWait)
		return err
	}

	closeDescriptors(w.closeAfterStart)

	var errch chan error
	if len(w.goroutines) > 0 {
		errch = make(chan error, len(w.goroutines))
		for _, fn := range w.goroutines {
			go func(f func() error) {
				errch <- f()
			}(fn)
		}
	}

	state, err := p.Wait()

	var copyError error
	for range w.goroutines {
		if err := <-errch; err != nil && copyError == nil {
			copyError = err
		}
	}
	closeDescriptors(w.closeAfterWait)

	if err != nil {
		return err
	}
	if !state.Success() {
		return &exec.ExitError{
			ProcessState: state,
		}
	}
	return copyError
}

func (w *wrappedCmd) env() ([]string, error) {
	if w.cmd.Env != nil {
		return w.cmd.Env, nil
	}
	return syscall.Environ(), nil
}

// addCriticalEnv adds any critical environment variables that are required
// (or at least almost always required) on the operating system.
// Currently this is only used for Windows.
func addCriticalEnv(env []string) []string {
	if runtime.GOOS != "windows" {
		return env
	}
	for _, kv := range env {
		eq := strings.Index(kv, "=")
		if eq < 0 {
			continue
		}
		k := kv[:eq]
		if strings.EqualFold(k, "SYSTEMROOT") {
			// We already have it.
			return env
		}
	}
	return append(env, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"))
}

// dedupEnv returns a copy of env with any duplicates removed, in favor of
// later values.
// Items not of the normal environment "key=value" form are preserved unchanged.
func dedupEnv(env []string) []string {
	return dedupEnvCase(runtime.GOOS == "windows", env)
}

// dedupEnvCase is dedupEnv with a case option for testing.
// If caseInsensitive is true, the case of keys is ignored.
func dedupEnvCase(caseInsensitive bool, env []string) []string {
	out := make([]string, 0, len(env))
	saw := make(map[string]int, len(env)) // key => index into out
	for _, kv := range env {
		eq := strings.Index(kv, "=")
		if eq < 0 {
			out = append(out, kv)
			continue
		}
		k := kv[:eq]
		if caseInsensitive {
			k = strings.ToLower(k)
		}
		if dupIdx, isDup := saw[k]; isDup {
			out[dupIdx] = kv
			continue
		}
		saw[k] = len(out)
		out = append(out, kv)
	}
	return out
}

// interfaceEqual protects against panics from doing equality tests on
// two interfaces with non-comparable underlying types.
func interfaceEqual(a, b interface{}) bool {
	defer func() {
		_ = recover()
	}()
	return a == b
}

func (w *wrappedCmd) CombinedOutput() ([]byte, error) {
	if !w.useStartProcess {
		return w.cmd.CombinedOutput()
	}

	var b bytes.Buffer
	w.captureStdout = &b
	w.captureStderr = &b
	err := w.Run()
	return b.Bytes(), err
}

func (w *wrappedCmd) Path() string {
	return w.cmd.Path
}
