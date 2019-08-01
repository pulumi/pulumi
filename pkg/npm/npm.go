package npm

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// Pack runs `npm pack` in the given directory, packaging the Node.js app located there into a
// tarball and returning it as `[]byte`. `stdout` is ignored for the command, as it does not
// generate useful data.
func Pack(dir string, stderr io.Writer) ([]byte, error) {
	// We pass `--loglevel=error` to prevent `npm` from printing warnings about missing
	// `description`, `repository`, and `license` fields in the package.json file.
	c := exec.Command("npm", "pack", "--loglevel=error")
	c.Dir = dir

	// Run the command. Note that `npm pack` doesn't have the ability to rename the resulting
	// filename, since it's meant to be uploaded directly to npm, which means that we have to get
	// that information by parsing the output of the command.
	var stdout bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return nil, err
	}
	packfile := strings.TrimSpace(stdout.String())
	defer os.Remove(packfile)

	packTarball, err := ioutil.ReadFile(packfile)
	if err != nil {
		return nil, fmt.Errorf(
			"npm pack completed successfully but the packed .tar.gz file was not generated")
	}

	return packTarball, nil
}

// Install runs `npm install` in the given directory, installing the dependencies for the Node.js
// app located there.
func Install(dir string, stdout, stderr io.Writer) error {
	// We pass `--loglevel=error` to prevent `npm` from printing warnings about missing
	// `description`, `repository`, and `license` fields in the package.json file.
	c := exec.Command("npm", "install", "--loglevel=error")
	c.Dir = dir

	// Run the command.
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		return err
	}

	// Ensure the "node_modules" directory exists.
	if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
		return errors.Errorf("npm install reported success, but node_modeules directory is missing")
	}

	return nil
}
