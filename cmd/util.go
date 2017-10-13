package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

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

// explicitOrCurrent returns an environment name after ensuring the environment exists. When a empty
// environment name is passed, the "current" ambient environment is returned
func explicitOrCurrent(name string) (tokens.QName, error) {
	if name == "" {
		return getCurrentEnv()
	}

	_, _, err := getEnvironment(tokens.QName(name))
	return tokens.QName(name), err
}

// getCurrentEnv reads the current environment.
func getCurrentEnv() (tokens.QName, error) {
	w, err := newWorkspace()
	if err != nil {
		return "", err
	}

	env := w.Settings().Env
	if env == "" {
		return "", errors.New("no current environment detected; please use `pulumi env init` to create one")
	}
	return env, nil
}

// setCurrentEnv changes the current environment to the given environment name, issuing an error if it doesn't exist.
func setCurrentEnv(name tokens.QName, verify bool) error {
	if verify {
		if _, _, err := getEnvironment(name); err != nil {
			return err
		}
	}

	// Switch the current workspace to that environment.
	w, err := newWorkspace()
	if err != nil {
		return err
	}

	w.Settings().Env = name
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
