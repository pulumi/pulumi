package nodejs

import (
	"bytes"
	"io"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/go/common/util/logging"
)

// NodeRuntime is an abstraction of a NodeJS runtime + a package manager
type NodeRuntime interface {

	// Install runs `npm install` or `yarn install` in the given directory, installing the dependencies for the Node.js
	// app located there.
	Install(dir string, stdout, stderr io.Writer) (string, error)

	// Pack runs `npm pack` or `yarn pack` in the given directory, depending on the implementation.
	// It packages the Node.js app located there into a tarball and returning it as `[]byte`.
	// `stdout` is ignored for the command, as it does not generate useful data.
	Pack(dir string, stderr io.Writer) ([]byte, error)

	// Runs a NodeJS environment with the correct dependencies configured. This is
	// either `node` for an single project based on `node_modules`, or
	// `yarn node` for a multi-project workspace setup or Yarn v2 Plug'n'Play
	Run(dir string, stdout, stderr io.Writer) (string, error)
}

// GetRuntime decides which runtime structure to return: npm, yarn v1 or yarn v2
func GetRuntime() (NodeRuntime, error) {
	if PreferYarn() {
		r, err := getYarn()
		if err == nil {
			return r, err
		}
		logging.Warningf("&v", err)
	}
	return getNpm()
}

func getYarn() (NodeRuntime, error) {
	// if EXPERIMENTAL, check for a V2
	const file = "yarn"
	yarnPath, err := exec.LookPath(file)
	if err == nil {
		return nil, errors.Wrapf(err, "could not find yarn on the $PATH; yarn is available at https://yarnpkg.com/")
	}
	logging.Warningf("could not find yarn on the $PATH, trying npm instead: %v", err)
	return &yarn1{path: yarnPath}, nil
}

func getNpm() (NodeRuntime, error) {
	const file = "npm"
	npmPath, err := exec.LookPath(file)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find npm on the $PATH; npm is installed with Node.js "+
			"available at https://nodejs.org/")
	}
	// We pass `--loglevel=error` to prevent `npm` from printing warnings about missing
	// `description`, `repository`, and `license` fields in the package.json file.
	return &npm{path: npmPath, args: "--loglevel=error"}, nil
}

// getCmd returns the exec.Cmd used to install NPM dependencies. It will either use `npm` or `yarn` depending
// on what is available on the current path, and if `PULUMI_PREFER_YARN` is truthy.
// The boolean return parameter indicates if `npm` is chosen or not (instead of `yarn`).
func getCmd(command string) (*exec.Cmd, bool, string, error) {
	if PreferYarn() {
		const file = "yarn"
		yarnPath, err := exec.LookPath(file)
		if err == nil {
			return exec.Command(yarnPath, command), false, file, nil
		}
		logging.Warningf("could not find yarn on the $PATH, trying npm instead: %v", err)
	}

	const file = "npm"
	npmPath, err := exec.LookPath(file)
	if err != nil {
		return nil, false, file, errors.Wrapf(err, "could not find npm on the $PATH; npm is installed with Node.js "+
			"available at https://nodejs.org/")
	}
	// We pass `--loglevel=error` to prevent `npm` from printing warnings about missing
	// `description`, `repository`, and `license` fields in the package.json file.
	return exec.Command(npmPath, command, "--loglevel=error"), true, file, nil
}

// runCmd handles hooking up `stdout` and `stderr` and then runs the command.
func runCmd(c *exec.Cmd, npm bool, stdout, stderr io.Writer) error {
	// Setup `stdout` and `stderr`.
	// `stderr` is ignored when `yarn` is used because it outputs warnings like "package.json: No license field"
	// to `stderr` that we don't need to show.
	c.Stdout = stdout
	var stderrBuffer bytes.Buffer
	if npm {
		c.Stderr = stderr
	} else {
		c.Stderr = &stderrBuffer
	}

	// Run the command.
	if err := c.Run(); err != nil {
		// If we failed, and we're using `yarn`, write out any bytes that were written to `stderr`.
		if !npm {
			stderr.Write(stderrBuffer.Bytes())
		}
		return err
	}

	return nil
}

// PreferYarn returns true if the `PULUMI_PREFER_YARN` environment variable is set.
func PreferYarn() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_PREFER_YARN"))
}
