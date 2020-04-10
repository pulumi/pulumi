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

	// GetNodePath returns the command to run to start NodeJS correctly under the respective
	// package manager.
	GetNodePath() string
}

// GetRuntime decides which runtime structure to return: npm, yarn v1 or yarn v2
func GetRuntime() (NodeRuntime, error) {
	if preferYarn() {
		r, err := getYarn()
		if err == nil {
			return r, err
		}
		logging.Warningf("&v", err)
	}
	return getNpm()
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

func preferYarn() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_PREFER_YARN"))
}

func getNodePath() (string, error) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return "", errors.Wrapf(err, "could not find node on the $PATH; Node.js is available at https://nodejs.org/")
	}
	return nodePath, nil
}
