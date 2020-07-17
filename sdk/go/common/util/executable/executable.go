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

	// look in $GOPATH/bin
	if goPath := os.Getenv("GOPATH"); len(goPath) > 0 {
		goPathProgram := filepath.Join(goPath, "bin", program)
		if fileInfo, err := os.Stat(goPathProgram); !os.IsNotExist(err) && !fileInfo.Mode().IsDir() {
			logging.V(5).Infof("program %s found in $GOPATH/bin", program)
			return goPathProgram, nil
		}
	}

	// look in the $PATH somewhere
	if fullPath, err := exec.LookPath(program); err == nil {
		logging.V(5).Infof("program %s found in $PATH", program)
		return fullPath, nil
	}

	return "", errors.Errorf(unableToFindProgramTemplate, program)
}
