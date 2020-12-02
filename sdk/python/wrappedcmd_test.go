package python

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os/exec"
	"testing"
)

func Test_wrappedCmd_CombinedOutputIsSame(t *testing.T) {
	getCmdOutput := func(t *testing.T, cmd *wrappedCmd) string {
		out, err := cmd.CombinedOutput()
		require.NoError(t, err)
		return string(out)
	}

	for _, tt := range []struct {
		name string
		args []string
	}{
		{
			name: "StdOutOnly",
			args: []string{"-c", "for i in range(0, 10): print(i)"},
		},
		{
			name: "StdErrOnly",
			args: []string{"-c", `for i in range(0, 10): import sys; sys.stderr.write("%d\n" % i)`},
		},
		{
			name: "StdOutAndErr",
			args: []string{"-c", `for i in range(0, 10): import sys; print(i); sys.stderr.write("%d\n" % i)`},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			execCmd := &wrappedCmd{
				cmd: exec.Command("python", tt.args...),
			}
			execCmdOut := getCmdOutput(t, execCmd)
			startProc := &wrappedCmd{
				cmd:             exec.Command("python", tt.args...),
				useStartProcess: true,
			}
			assert.NotEmpty(t, execCmdOut)
			startProcOut := getCmdOutput(t, startProc)
			assert.Equal(t, startProcOut, execCmdOut)
			assert.Equal(t, execCmd.Path(), startProc.Path())
			assert.Equal(t, execCmd.Args(), startProc.Args())
		})

	}

}
