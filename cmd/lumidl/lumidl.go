// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/tools/lumidl"
	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
)

func NewIDLCCmd() *cobra.Command {
	var logToStderr bool
	var outPack string
	var outRPC string
	var pkgBaseIDL string
	var pkgBaseRPC string
	var quiet bool
	var recursive bool
	var verbose int
	cmd := &cobra.Command{
		Use:   "lumidl pkg-name idl-path",
		Short: "The Lumi IDL compiler generates Lumi metadata and RPC stubs from IDL written in Go",
		Long: "The Lumi IDL compiler generates Lumi metadata and RPC stubs from IDL written in Go.\n" +
			"\n" +
			"The tool accepts a subset of Go types and produces packages that can be consumed by\n" +
			"ordinary Lumi programs and libraries in any language.  The pkg-name argument\n" +
			"controls the output package name and idl-path is the path to the IDL source code.\n" +
			"\n" +
			"The --out-pack and --out-rpc flags indicate where generated code is to be saved,\n" +
			"and pkg-base-idl and --pkg-base-rpc may be used to override the default inferred Go\n" +
			"package names (which, by default, are based on your GOPATH).",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmdutil.InitLogging(logToStderr, verbose, true)
		},
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Usage()
			} else if len(args) == 1 {
				return errors.New("missing required [idl-path] argument")
			}

			// Now pass the arguments and compile the package.
			name := args[0] // the name of the Lumi package.
			path := args[1] // the path to the IDL directory that is compiled recursively.
			return lumidl.Compile(lumidl.CompileOptions{
				Name:       tokens.PackageName(name),
				PkgBaseIDL: pkgBaseIDL,
				PkgBaseRPC: pkgBaseRPC,
				OutPack:    outPack,
				OutRPC:     outRPC,
				Quiet:      quiet,
				Recursive:  recursive,
			}, path)
		}),
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			glog.Flush()
		},
	}

	cmd.PersistentFlags().BoolVar(
		&logToStderr, "logtostderr", false, "Log to stderr instead of to files")
	cmd.PersistentFlags().BoolVarP(
		&recursive, "recursive", "r", false, "Recursively generate code for all sub-packages in the target")
	cmd.PersistentFlags().StringVar(
		&outPack, "out-pack", "", "Save generated package metadata to this directory")
	cmd.PersistentFlags().StringVar(
		&outRPC, "out-rpc", "", "Save generated RPC provider stubs to this directory")
	cmd.PersistentFlags().StringVar(
		&pkgBaseIDL, "pkg-base-idl", "", "Override the base URL where the IDL package is published")
	cmd.PersistentFlags().StringVar(
		&pkgBaseRPC, "pkg-base-rpc", "", "Override the base URL where the RPC package is published")
	cmd.PersistentFlags().BoolVarP(
		&quiet, "quiet", "q", false, "Suppress non-error output progress messages")
	cmd.PersistentFlags().IntVarP(
		&verbose, "verbose", "v", 0, "Enable verbose logging (e.g., v=3); anything >3 is very verbose")

	return cmd
}
