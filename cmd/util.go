package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/pulumi/pulumi/pkg/pack"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// newWorkspace creates a new workspace using the current working directory.
func newWorkspace() (workspace.W, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return workspace.New(pwd)
}

// explicitOrCurrent returns an stack name after ensuring the stack exists. When a empty
// stack name is passed, the "current" ambient stack is returned
func explicitOrCurrent(name string) (tokens.QName, error) {
	if name == "" {
		return getCurrentStack()
	}

	_, _, err := getStack(tokens.QName(name))
	return tokens.QName(name), err
}

// getCurrentStack reads the current stack.
func getCurrentStack() (tokens.QName, error) {
	w, err := newWorkspace()
	if err != nil {
		return "", err
	}

	stack := w.Settings().Stack
	if stack == "" {
		return "", errors.New("no current stack detected; please use `pulumi stack init` to create one")
	}
	return stack, nil
}

// setCurrentStack changes the current stack to the given stack name, issuing an error if it doesn't exist.
func setCurrentStack(name tokens.QName, verify bool) error {
	if verify {
		if _, _, err := getStack(name); err != nil {
			return err
		}
	}

	// Switch the current workspace to that stack.
	w, err := newWorkspace()
	if err != nil {
		return err
	}

	w.Settings().Stack = name
	return w.Save()
}

// displayEvents reads events from the `events` channel until it is closed, displaying each event as it comes in.
// Once all events have been read from the channel and displayed, it closes the `done` channel so the caller can
// await all the events being written.
func displayEvents(events <-chan engine.Event, done chan bool, debug bool) {
	defer close(done)

	for event := range events {
		switch event.Type {
		case "stdoutcolor":
			fmt.Print(colors.ColorizeText(event.Payload.(string)))
		case "diag":
			payload := event.Payload.(engine.DiagEventPayload)
			var out io.Writer
			out = os.Stdout

			if payload.Severity == diag.Error || payload.Severity == diag.Warning {
				out = os.Stderr
			}
			if payload.Severity == diag.Debug && !debug {
				out = ioutil.Discard
			}
			msg := payload.Message
			if payload.UseColor {
				msg = colors.ColorizeText(msg)
			}
			fmt.Fprintf(out, msg)
		default:
			contract.Failf("unknown event type '%s'", event.Type)
		}
	}
}

func getPackage() (*pack.Package, error) {
	pkgPath, err := getPackageFilePath()
	if err != nil {
		return nil, err
	}

	pkg, err := pack.Load(pkgPath)

	return pkg, err
}

func savePackage(pkg *pack.Package) error {
	pkgPath, err := getPackageFilePath()
	if err != nil {
		return err
	}

	return pack.Save(pkgPath, pkg)
}

func getPackageFilePath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	pkgPath, err := workspace.DetectPackage(dir)
	if err != nil {
		return "", err
	}

	if pkgPath == "" {
		return "", errors.Errorf("could not find Pulumi.yaml, started search in %s", dir)
	}

	return pkgPath, nil
}
