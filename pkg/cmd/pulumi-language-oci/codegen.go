// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Codegen RPCs for the OCI language host.
//
// `runtime: oci` is the *runtime* axis — how a program runs (as a container). The
// program's source language is a separate *dev* axis the project declares as
// `runtime.options.language`. SDK generation (GeneratePackage) and linking (Link) are
// dev-time, language-specific operations the container model doesn't change: the
// codegen for a language is toolchain-free Go compiled into `pulumi-language-<lang>`,
// so the OCI host *delegates* these RPCs to the real language host rather than
// reimplementing them. This keeps the CLI's model unchanged (it just calls "the
// project's runtime host"), makes the runtime/dev-time split literal (the OCI host
// owns Run/InstallDependencies; it forwards GeneratePackage/Link), and generalizes to
// any language — including ones distributed out of core — for free.
//
// The delegate binary is bundled in the engine/CLI image today (the OCI host loads the
// sibling `pulumi-language-<lang>` exactly as the CLI would); running it as its own
// container via the proxy is the same de-bundling already proven for providers, left
// for later.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	hcl "github.com/hashicorp/hcl/v2"

	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/oci"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// GeneratePackage generates an SDK package by delegating to the project's dev-language
// host. The container model doesn't change codegen, so we forward the request verbatim
// to `pulumi-language-<runtime.options.language>` and return its result.
func (h *ociHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	var diags hcl.Diagnostics
	err := h.withDelegateRuntime(ctx, func(lang plugin.LanguageRuntime) error {
		// All GeneratePackageRequest fields map 1:1 onto the interface method, so this
		// is a faithful forward. The delegate loads schemas from the loader_target the
		// CLI started (carried in the request), not through this host.
		d, err := lang.GeneratePackage(ctx,
			req.Directory, req.Schema, req.ExtraFiles, req.LoaderTarget, req.LocalDependencies, req.Local)
		diags = d
		return err
	})
	if err != nil {
		return nil, err
	}
	// Diagnostics (including error-level ones) ride back in the response, not as a gRPC
	// error: the CLI inspects diags.HasErrors() itself, exactly as for a normal host.
	return &pulumirpc.GeneratePackageResponse{
		Diagnostics: plugin.HclDiagnosticsToRPCDiagnostics(diags),
	}, nil
}

// Link wires a generated SDK into the program — and it is BUILD-OWNED, not delegated to
// the language host. Editing the program's manifest to declare the SDK (go.mod replace,
// package.json `file:`, `pip install -e`, a nix expression) is *package-manager*-specific,
// a finer axis than language: the same Python host shouldn't have to know pip vs poetry vs
// uv vs nix. The build controls the package manager, so the build owns linking. The OCI
// host runs the project's build-spec `link` command in the build environment image (which
// carries the package manager), handing it the just-generated SDK path(s); templates
// scaffold the right command per language×package-manager (oci-python-pip, oci-python-nix,
// …), so supporting a new one is a template, not a core change. See the design doc,
// "`Link` → build-owned".
func (h *ociHost) Link(ctx context.Context, req *pulumirpc.LinkRequest) (*pulumirpc.LinkResponse, error) {
	build := req.GetInfo().GetOptions().GetFields()["build"].GetStructValue()
	linkCmd := optString(build, "link")
	if linkCmd == "" {
		// No link command: the SDK is recorded as a ref only, and the program build wires
		// it when it runs (the degenerate "the build owns all of it" case). Deliberately a
		// no-op — the OCI host never edits a language manifest itself.
		fmt.Fprintf(os.Stderr, "oci: no build.link command configured; the program build will wire the SDK\n")
		return &pulumirpc.LinkResponse{}, nil
	}
	buildImage := optString(build, "image")
	if buildImage == "" {
		return nil, fmt.Errorf("oci: build.link is set but build.image (the build environment) is not")
	}

	var paths []string
	for _, p := range req.GetPackages() {
		if p.GetPath() != "" {
			paths = append(paths, p.GetPath())
		}
	}
	// The link command runs in the project root (reached via --volumes-from the engine,
	// like every build step) with the SDK path(s) it should wire, relative to that root.
	env := map[string]string{"PULUMI_LINK_SDK_PATHS": strings.Join(paths, "\n")}
	fmt.Fprintf(os.Stderr, "oci: linking SDK via build.link in %s: %s\n", buildImage, linkCmd)
	if _, err := oci.BuildInContainer(
		ctx, buildImage, linkCmd, req.GetInfo().GetRootDirectory(), optStringList(build, "caches"), env, os.Stderr,
	); err != nil {
		return nil, fmt.Errorf("oci: running build.link command: %w", err)
	}
	return &pulumirpc.LinkResponse{}, nil
}

// withDelegateRuntime loads the project's dev-language host and invokes fn with it.
// The host is minimal: no loader/mapper/resolver factories (the delegate reaches the
// CLI's loader directly via the request's loader_target), so this needs no registry or
// schema plumbing — just enough to spawn the sibling language plugin and forward a call.
func (h *ociHost) withDelegateRuntime(ctx context.Context, fn func(plugin.LanguageRuntime) error) error {
	lang, err := delegateLanguage()
	if err != nil {
		return err
	}

	sink := diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{Color: colors.Never})
	host, err := pkghost.New(ctx, sink, sink, nil /*debug*/, nil /*installLang*/, nil /*loader*/, nil /*mapper*/, nil /*resolver*/)
	if err != nil {
		return fmt.Errorf("oci: creating plugin host for codegen delegation: %w", err)
	}
	defer contract.IgnoreClose(host)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("oci: determining working directory for codegen delegation: %w", err)
	}
	pctx, err := plugin.NewContext(ctx, sink, sink, host, nil, cwd, nil, false, nil)
	if err != nil {
		return fmt.Errorf("oci: creating plugin context for codegen delegation: %w", err)
	}
	defer contract.IgnoreClose(pctx)

	langRuntime, err := host.LanguageRuntime(pctx, lang)
	if err != nil {
		return fmt.Errorf("oci: loading delegate language host %q: %w", lang, err)
	}
	return fn(langRuntime)
}

// delegateLanguage reads runtime.options.language from the project in the host's
// working directory — the engine launches a language host with cwd == the project root,
// the same directory plugin.NewContext detects the project from. The codegen RPCs carry
// no runtime options of their own, so the project file is the signal's home.
func delegateLanguage() (string, error) {
	projPath, err := workspace.DetectProjectPath()
	if err != nil {
		return "", fmt.Errorf("oci: locating the project to determine the SDK language: %w", err)
	}
	if projPath == "" {
		return "", fmt.Errorf("oci: no project found in the working directory to determine the SDK language")
	}
	proj, err := workspace.LoadProject(projPath)
	if err != nil {
		return "", fmt.Errorf("oci: loading project %q: %w", projPath, err)
	}
	lang, _ := proj.Runtime.Options()["language"].(string)
	if lang == "" {
		return "", fmt.Errorf(
			"oci: set runtime.options.language in %s to the SDK language (e.g. nodejs) "+
				"so the oci host can delegate SDK generation", projPath)
	}
	return lang, nil
}

