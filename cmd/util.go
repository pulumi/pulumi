package cmd

import (
	"os"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/tokens"
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

	_, _, _, err := lumiEngine.Environment.GetEnvironment(tokens.QName(name))
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
		if _, _, _, err := lumiEngine.Environment.GetEnvironment(name); err != nil {
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
