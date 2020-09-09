package auto

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func AutoRun(pulumiFunc pulumi.RunFunc) {
	ctx := context.Background()
	w, err := NewLocalWorkspace(ctx, WorkDir(filepath.Join(".")))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: program failed: %v\n", err)
		os.Exit(1)
	}

	w.SetProgram(pulumiFunc)

	operation, stackName := parseArgs()

	if stackName == "" {
		sSummary, err := w.Stack(ctx)
		if err != nil || sSummary == nil {
			fmt.Fprintf(os.Stderr, "error: program failed: no stack selected: %v\n", err)
			os.Exit(1)
		}
		stackName = sSummary.Name
	}

	stack, err := SelectStack(ctx, stackName, w)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: program failed: unable to select stack: %v\n", err)
		os.Exit(1)
	}

	switch operation {
	case up:
		_, err = stack.Up(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: program failed: %v\n", err)
			os.Exit(1)
		}
		break
	case preview:
		_, err = stack.Preview(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: program failed: %v\n", err)
			os.Exit(1)
		}
		break
	case destroy:
		_, err = stack.Destroy(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: program failed: %v\n", err)
			os.Exit(1)
		}
	case refresh:
		_, err = stack.Refresh(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: program failed: %v\n", err)
			os.Exit(1)
		}
	}

}

type op int

const (
	up op = iota
	preview
	refresh
	destroy
)

func parseArgs() (op, string) {
	operation := up
	stack := ""

	for _, a := range os.Args[1:] {
		stackParts := strings.Split(a, "/")
		if len(stackParts) == 3 {
			stack = a
			continue
		}
		switch a {
		case "up", "update":
			operation = up
			continue
		case "pre", "preview":
			operation = preview
			continue
		case "refresh":
			operation = refresh
			continue
		case "destroy":
			operation = destroy
		}
	}

	return operation, stack
}
