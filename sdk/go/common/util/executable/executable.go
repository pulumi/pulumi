package executable

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
)

const unableToFindProgramTemplate = "unable to find program: %s"

// FindExecutable attempts to find the needed executable in various locations on the
// filesystem, eventually resorting to searching in $PATH.
func FindExecutable(program string) (string, error) {
	if runtime.GOOS == "windows" && !strings.HasSuffix(program, ".exe") {
		program = fmt.Sprintf("%s.exe", program)
	}
	// look in the same directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "unable to get current working directory")
	}

	cwdProgram := filepath.Join(cwd, program)
	if fileInfo, err := os.Stat(cwdProgram); !os.IsNotExist(err) && !fileInfo.Mode().IsDir() {
		logging.V(5).Infof("program %s found in CWD", program)
		return cwdProgram, nil
	}

	// look in potentials $GOPATH/bin
	if goPath := os.Getenv("GOPATH"); len(goPath) > 0 {
		// getPotentialPaths will return paths where to look.
		// Because the GOPATH can take the form of multiple paths (e.g: GOPATH="home/user/go:/usr/local/go")
		// we need to split the GOPATH, and look into each of the paths.
		// If the GOPATH hold only one path, there will only be one element in the slice.
		potentialPaths := getPotentialPaths(goPath, runtime.GOOS)
		for i, pp := range potentialPaths {
			goPathProgram := filepath.Join(pp, "bin", program)
			fileInfo, err := os.Stat(goPathProgram)

			if err != nil && i+1 == len(potentialPaths) {
				if !os.IsNotExist(err) {
					return "", errors.Wrapf(err, "unable to find program in these paths: %q", strings.Join(potentialPaths, ", "))
				}
			}

			if fileInfo != nil && !fileInfo.Mode().IsDir() {
				logging.V(5).Infof("program %s found in %s/bin", program, pp)
				return goPathProgram, nil
			}
		}
	}

	// look in the $PATH somewhere
	if fullPath, err := exec.LookPath(program); err == nil {
		logging.V(5).Infof("program %s found in $PATH", program)
		return fullPath, nil
	}

	return "", errors.Errorf(unableToFindProgramTemplate, program)
}

func getPotentialPaths(goPath string, os string) []string {
	var sep string
	switch os {
	case "windows":
		sep = ";"
	case "linux", "darwin":
		sep = ":"
	}

	return strings.Split(goPath, sep)
}
