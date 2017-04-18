// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/loader"

	"github.com/pulumi/coconut/pkg/util/cmdutil"
)

func NewCocogoCmd() *cobra.Command {
	var logToStderr bool
	var verbose int
	cmd := &cobra.Command{
		Use:   "cocogo [packages]",
		Short: "CocoGo compiles Go programs into Coconut metadata",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmdutil.InitLogging(logToStderr, verbose)
		},
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return Compile(args...)
		}),
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			glog.Flush()
		},
	}

	cmd.PersistentFlags().BoolVar(&logToStderr, "logtostderr", false, "Log to stderr instead of to files")
	cmd.PersistentFlags().IntVarP(
		&verbose, "verbose", "v", 0, "Enable verbose logging (e.g., v=3); anything >3 is very verbose")

	return cmd
}

func Compile(pkgs ...string) error {
	var conf loader.Config
	if _, err := conf.FromArgs(pkgs, false); err != nil {
		return err
	}

	prog, err := conf.Load()
	if err != nil {
		return err
	}

	for _, pkginfo := range prog.Created {
		pkg := pkginfo.Pkg
		fmt.Printf("parsed package %v\n", pkg.Name())
		fmt.Printf("\tpath=%v\n", pkg.Path())
		fmt.Printf("\timports=%v\n", pkg.Imports())
		fmt.Printf("\tscope=%v\n", pkg.Scope())
	}

	return nil
}
