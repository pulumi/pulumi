package integration

import (
	"testing"

	"github.com/mitchellh/go-ps"
	"strings"
)

func detectOrphanProcesses(t *testing.T) {
	info, err := getProcInfo()
	if err != nil {
		t.Error(err)
		return
	}
	for _, p := range info.procs {
		if isOrphanPulumiLanguageProcess(p, info.parentMap) {
			parentInfo := ""
			if parent, ok := info.parentMap[p.Pid()]; ok && parent != nil {
				parentInfo = " parent=" + (*parent).Executable()
			}
			t.Errorf("Detected an orphan Pulumi language plugin process: %s pid=%d ppid=%d%s",
				p.Executable(), p.Pid(), p.PPid(), parentInfo)
		}
	}
}

type procInfo struct {
	procs     []ps.Process
	parentMap map[int]*ps.Process
}

func getProcInfo() (*procInfo, error) {
	procs, err := ps.Processes()
	if err != nil {
		return nil, err
	}

	byPid := map[int]ps.Process{}
	for _, p := range procs {
		byPid[p.Pid()] = p
	}

	parentMap := map[int]*ps.Process{}
	for _, p := range procs {
		par, havePar := byPid[p.PPid()]
		if havePar {
			parentMap[p.Pid()] = &par
		} else {
			parentMap[p.Pid()] = nil
		}
	}

	return &procInfo{procs, parentMap}, nil
}

func isOrphanPulumiLanguageProcess(pr ps.Process, parentMap map[int]*ps.Process) bool {
	pa, gotPa := parentMap[pr.Pid()]
	if !strings.Contains(pr.Executable(), "pulumi-language") {
		return false
	}
	return !gotPa || pa == nil || !strings.Contains((*pa).Executable(), "pulumi")
}
