package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rogpeppe/go-internal/lockedfile"
)

func main() {
	flag.Parse()
	args := flag.Args()

	cfg := requireConfig()

	if len(args) >= 1 && args[0] == "run" || args[0] == "build" {
		exitCode := executeLockedGo(cfg.realgo, cfg.lockfile, args)
		os.Exit(exitCode)
	} else {
		exitCode := executeGo(cfg.realgo, args)
		os.Exit(exitCode)
	}
}

type config struct {
	lockfile string
	realgo   string
}

func requireConfig() config {
	runnerTemp := os.Getenv("RUNNER_TEMP")
	if runnerTemp == "" {
		log.Fatal("RUNNER_TEMP env var must be set")
	}
	lockfile := filepath.Join(runnerTemp, "go-wrapper", "go.lock")
	realgoFile := filepath.Join(runnerTemp, "go-wrapper", "realgo.path")
	realgoBytes, err := ioutil.ReadFile(realgoFile)
	if err != nil {
		log.Fatal(err)
	}
	realgo := strings.TrimSpace(string(realgoBytes))
	return config{lockfile, realgo}
}

func executeLockedGo(realgo, lockfile string, args []string) int {
	mutex := lockedfile.MutexAt(lockfile)
	unlock, err := mutex.Lock()
	if err != nil {
		log.Fatal(err)
	}
	defer unlock()
	return executeGo(realgo, args)
}

func executeGo(realgo string, args []string) int {
	cmd := exec.Command(realgo, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Run()
	return cmd.ProcessState.ExitCode()
}
