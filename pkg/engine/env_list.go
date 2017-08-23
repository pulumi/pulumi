package engine

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-fabric/pkg/encoding"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/workspace"
)

func ListEnvs() error {
	// Read the environment directory.
	path := workspace.EnvPath("")
	files, err := ioutil.ReadDir(path)
	if err != nil && !os.IsNotExist(err) {
		return errors.Errorf("could not read environments: %v", err)
	}

	fmt.Fprintf(E.Stdout, "%-20s %-48s %-12s\n", "NAME", "LAST DEPLOYMENT", "RESOURCE COUNT")
	curr := getCurrentEnv()
	for _, file := range files {
		// Ignore directories.
		if file.IsDir() {
			continue
		}

		// Skip files without valid extensions (e.g., *.bak files).
		envfn := file.Name()
		ext := filepath.Ext(envfn)
		if _, has := encoding.Marshalers[ext]; !has {
			continue
		}

		// Read in this environment's information.
		name := tokens.QName(envfn[:len(envfn)-len(ext)])
		target, snapshot, checkpoint := readEnv(name)
		if checkpoint == nil {
			continue // failure reading the environment information.
		}

		// Now print out the name, last deployment time (if any), and resources (if any).
		lastDeploy := "n/a"
		resourceCount := "n/a"
		if checkpoint.Latest != nil {
			lastDeploy = checkpoint.Latest.Time.String()
		}
		if snapshot != nil {
			resourceCount = strconv.Itoa(len(snapshot.Resources))
		}
		display := target.Name
		if display == curr {
			display += "*" // fancify the current environment.
		}
		fmt.Fprintf(E.Stdout, "%-20s %-48s %-12s\n", display, lastDeploy, resourceCount)
	}

	return nil
}
