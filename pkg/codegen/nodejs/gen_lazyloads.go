// Copyright 2016-2022, Pulumi Corporation.
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

// Large providers like azure-native generate code that has a high
// startup overhead in Node JS due to loading thousands of modules at
// once. The code in this file is dedicated to support on-demand
// (lazy) loading of modules at Node level to speed up program
// startup.

package nodejs

import (
	"fmt"
	"io"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type lazyLoadGen struct{}

func newLazyLoadGen() *lazyLoadGen {
	return &lazyLoadGen{}
}

func nodeModuleOptimizationFlags(pkg *schema.Package) struct {
	lazyLoadFunctions          bool
	lazyLoadResources          bool
	useTypeOnlyEnumsReferences bool
} {
	flags := struct {
		lazyLoadFunctions          bool
		lazyLoadResources          bool
		useTypeOnlyEnumsReferences bool
	}{}
	nodePackageInfo := NodePackageInfo{}
	if languageInfo, ok := pkg.Language["nodejs"]; ok {
		if info, ok2 := languageInfo.(NodePackageInfo); ok2 {
			nodePackageInfo = info
		}
	}

	for _, s := range nodePackageInfo.OptimizeNodeModuleLoading {
		switch s {
		case "lazy-load-functions":
			flags.lazyLoadFunctions = true
		case "lazy-load-resources":
			flags.lazyLoadResources = true
		case "use-type-only-enums-references":
			flags.useTypeOnlyEnumsReferences = true
		default:
			continue
		}
	}
	return flags
}

// Generates TypeScript code to re-export a generated module. For
// resources and functions this is optimized to use lazy loading.
// Falls back to eager re-export for everything else.
func (ll *lazyLoadGen) genReexport(w io.Writer, exp fileInfo, importPath string) {
	if exp.fileType == functionFileType {
		// optimize lazy-loading function modules
		ll.genFunctionReexport(w, exp.functionFileInfo, importPath)
	} else if exp.fileType == resourceFileType {
		// optimize lazy-loading resource modules
		ll.genResourceReexport(w, exp.resourceFileInfo, importPath)
	} else {
		// non-optimized but foolproof eager reexport
		fmt.Fprintf(w, "export * from %s;\n", importPath)
	}
}

// Generates TypeScript code that lazily imports and re-exports a
// module defining a resource, while also importing the resoure class
// in-scope. Needs to know which names the module defines
// (resourceFileInfo).
func (*lazyLoadGen) genResourceReexport(w io.Writer, i resourceFileInfo, importPath string) {
	defer fmt.Fprintf(w, "\n")

	quotedImport := fmt.Sprintf("%q", importPath)

	// not sure how to lazy-load in presence of re-exported
	// namespaces; bail and use an eager load in this case; also
	// eager-import the class.
	if i.methodsNamespaceName != "" {
		fmt.Fprintf(w, "export * from %s;\n", quotedImport)
		fmt.Fprintf(w, "import { %s } from %s;\n", i.resourceClassName, quotedImport)
		return
	}

	// Optimize the case of resource files to lazy-load them in
	// Node; instead of reexporting everything which generates
	// require() calls.
	//
	// Note that we cannot yet do this reliably if the resource
	// file exports a methods namespace.
	interfaces := []string{
		i.resourceArgsInterfaceName,
	}
	if i.stateInterfaceName != "" {
		interfaces = append(interfaces, i.stateInterfaceName)
	}
	// Re-export interfaces. This is type-only and does not
	// generate a require() call.
	fmt.Fprintf(w, "export { %s } from %s;\n",
		strings.Join(interfaces, ", "),
		quotedImport)

	// Re-export class type into the type group, see
	// https://www.typescriptlang.org/docs/handbook/declaration-merging.html
	fmt.Fprintf(w, "export type %[1]s = import(%[2]s).%[1]s;\n",
		i.resourceClassName,
		quotedImport)

	// Mock re-export class value into the value group - for compilation.
	fmt.Fprintf(w, "export const %[1]s: typeof import(%[2]s).%[1]s = null as any;\n",
		i.resourceClassName,
		quotedImport)

	// At runtime, install lazy loading. This has no effect on types.
	fmt.Fprintf(w, "utilities.lazyLoadProperty(exports, %q, () => require(%s));\n",
		i.resourceClassName, quotedImport)
}

// Generates TypeScript code that lazily imports and re-exports a
// module defining a function. Needs to which names the module defines
// (functionFileInfo).
func (*lazyLoadGen) genFunctionReexport(w io.Writer, i functionFileInfo, importPath string) {
	defer fmt.Fprintf(w, "\n")

	quotedImport := fmt.Sprintf("%q", importPath)

	// Re-export interfaces. This is type-only and does not
	// generate a require() call.
	interfaces := i.interfaces()
	if len(interfaces) > 0 {
		fmt.Fprintf(w, "export { %s } from %s;\n",
			strings.Join(interfaces, ", "),
			quotedImport)
	}

	// Re-export function values into the value group, and install lazy loading.
	for _, f := range i.functions() {
		fmt.Fprintf(w, "export const %[1]s: typeof import(%[2]s).%[1]s = null as any;\n",
			f, quotedImport)
		fmt.Fprintf(w, "utilities.lazyLoadProperty(exports, %q, () => require(%s));\n",
			f, quotedImport)
	}
}
