// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/tools/cidlc"
	"github.com/pulumi/coconut/pkg/util/cmdutil"
)

func NewCIDLCCmd() *cobra.Command {
	var logToStderr bool
	var name string
	var outPack string
	var outProvider string
	var root string
	var verbose int
	cmd := &cobra.Command{
		Use:   "cidlc --name <name> [paths...]",
		Short: "CIDLC generates Coconut metadata and RPC stubs from IDL written in Go",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmdutil.InitLogging(logToStderr, verbose)
		},
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return errors.New("missing required package name (--name or -n)")
			}
			if root == "" {
				root, _ = os.Getwd() // default to the current working directory.
			} else {
				root, _ = filepath.Abs(root)
			}
			if outPack != "" {
				outPack, _ = filepath.Abs(outPack)
			}
			if outProvider != "" {
				outProvider, _ = filepath.Abs(outProvider)
			}

			return cidlc.Compile(cidlc.CompileOptions{
				Name:        name,
				Root:        root,
				OutPack:     outPack,
				OutProvider: outProvider,
			}, args...)
		}),
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			glog.Flush()
		},
	}

	cmd.PersistentFlags().BoolVar(
		&logToStderr, "logtostderr", false, "Log to stderr instead of to files")
	cmd.PersistentFlags().StringVarP(
		&name, "name", "n", "", "The Coconut package name")
	cmd.PersistentFlags().StringVar(
		&outPack, "out-pack", "", "Save generated package metadata to this directory")
	cmd.PersistentFlags().StringVar(
		&outProvider, "out-provider", "", "Save generated RPC provider stubs to this directory")
	cmd.PersistentFlags().StringVarP(
		&root, "root", "r", "", "Pick a different root directory than `pwd` (the default)")
	cmd.PersistentFlags().IntVarP(
		&verbose, "verbose", "v", 0, "Enable verbose logging (e.g., v=3); anything >3 is very verbose")

	return cmd
}
