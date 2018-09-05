// Copyright 2016-2018, Pulumi Corporation.
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

import * as fs from "fs";
import * as filepath from "path";
import * as readPackageTree from "read-package-tree";
import * as asset from "../../asset";
import { RunError } from "../../errors";

/**
 * computeRequiredSubDependencyPaths Takes in the set of "require"d modules from a serialied function
 * and determines which dependencies of @pulumi/... need to be included.  Normally We do not include
 * @pulumi/... packages in a serialized function (largely due to the size as well as because these
 * libraries are only intended to be used at deployment time.  However, @pulumi/... might itself
 * depend on *other* libraries that *are* needed at runtime.  This function helps determine that set
 * so that we will include those sub-packages even if we filter out the main @pulumi/... package
 * itself.
 */
export function computeRequiredSubDependencyPaths(requiredModules: Set<string>): Set<string> {
    const requiredPaths = new Set<string>();

    const nodeModulesPiece = "node_modules";
    for (const requiredModule of requiredModules) {
        // Check if requiredModule is like:
        // "@pulumi/cloud-azure/node_modules/azure-sb/lib/servicebus.js"
        const split = requiredModule.split("/");

        // has to start with @pulumi.
        if (split[0] !== "@pulumi") {
            continue;
        }

        // Has to reference node_modules somewhere under that.
        const nodeModulesIndex = split.indexOf(nodeModulesPiece);
        if (nodeModulesIndex < 0) {
            continue;
        }

        // Has to have something following node_modules. (i.e. azure-sb)
        if (!split[nodeModulesIndex + 1]) {
            continue;
        }

        // We want to include all the pieces of the split up through and including the *next*
        // item after the node_modules
        const pieces = [nodeModulesPiece, ...split.slice(0, nodeModulesIndex + 2)];

        // This will generate a path like: node_modules/@pulumi/cloud-azure/node_modules/azure-sb
        // This is the form we want to pass in as an 'extraIncludePaths' when calling 'computeCodePaths'
        requiredPaths.add(pieces.join("/"));
    }

    return requiredPaths;
}

// computeCodePaths computes the local node_module paths to include in an uploaded cloud 'Lambda'.
// Specifically, it will examine the package.json for the caller's code, and will transitively walk
// it's 'dependencies' section to determine what packages should be included.
//
// During this walk, if an @pulumi/... package is encountered, it will not be walked into.  @pulumi
// packages are deployment-time only packages and will not runtime.  This prevents uploading
// packages needlessly which can majorly bloat up the size of uploaded code.
//
// [extraIncludePaths], [extraExcludePackages] and [extraExcludePackages] can all be used to adjust
// the final set of paths included in the resultant asset/archive map.
//
// Note: this functionality is specifically intended for use by downstream library code that is
// determining what is needed for a cloud-lambda.  i.e. the aws.serverless.Function or
// azure.serverless.FunctionApp libraries.  In general, other clients should not need to use this
// helper.
export async function computeCodePaths(
    extraIncludePaths?: string[],
    extraIncludePackages?: string[],
    extraExcludePackages?: string[]): Promise<Map<string, asset.Asset | asset.Archive>> {

    // Construct the set of paths to include in the archive for upload.

    const includedPackages = new Set<string>(extraIncludePackages || []);
    const excludedPackages = new Set<string>(extraExcludePackages || []);

    // Find folders for all packages requested by the user
    const pathSet = await allFoldersForPackages(includedPackages, excludedPackages);

    // Add all paths explicitly requested by the user
    extraIncludePaths = extraIncludePaths || [];
    for (const path of extraIncludePaths) {
        pathSet.add(path);
    }

    const codePaths: Map<string, asset.Asset | asset.Archive> = new Map();

    // For each of the required paths, add the corresponding FileArchive or FileAsset to the
    // AssetMap.
    for (const path of pathSet) {
        // Don't include a path if there is another path higher up that will include this one.
        if (isSubsumedByHigherPath(path, pathSet)) {
            continue;
        }

        // The Asset model does not support a consistent way to embed a file-or-directory into an
        // `AssetArchive`, so we stat the path to figure out which it is and use the appropriate
        // Asset constructor.
        const stats = fs.lstatSync(path);
        if (stats.isDirectory()) {
            codePaths.set(path, new asset.FileArchive(path));
        }
        else {
            codePaths.set(path, new asset.FileAsset(path));
        }
    }

    return codePaths;
}

function isSubsumedByHigherPath(path: string, pathSet: Set<string>): boolean {
    for (const otherPath of pathSet) {
        if (path.length > otherPath.length &&
            path.startsWith(otherPath)) {

            // Have to make sure we're actually a sub-directory of that other path.  For example,
            // if we have:  node_modules/mime-types, that's not subsumed by node_modules/mime
            const nextChar = path.charAt(otherPath.length);
            return nextChar === "/" || nextChar === "\\";
        }
    }

    return false;
}

// allFolders computes the set of package folders that are transitively required by the root
// 'dependencies' node in the client's project.json file.
function allFoldersForPackages(includedPackages: Set<string>, excludedPackages: Set<string>): Promise<Set<string>> {
    return new Promise((resolve, reject) => {
        readPackageTree(".", <any>undefined, (err: any, root: readPackageTree.Node) => {
            if (err) {
                return reject(err);
            }

            // read-package-tree defers to read-package-json to parse the project.json file. If that
            // fails, root.error is set to the underlying error.  In that case, we want to fail as
            // well.  Otherwise, we will silently proceed as if package.json was empty, which would
            // result in us uploading no node_modules.
            if (root.error) {
                return reject(new RunError(
                    "Failed to parse package.json. Underlying issue:\n  " + root.error.toString()));
            }

            // This is the core starting point of the algorithm.  We use readPackageTree to get the
            // package.json information for this project, and then we start by walking the
            // .dependencies node in that package.  Importantly, we do not look at things like
            // .devDependencies or or .peerDependencies.  These are not what are considered part of
            // the final runtime configuration of the app and should not be uploaded.
            const referencedPackages = new Set<string>(includedPackages);
            if (root.package && root.package.dependencies) {
                for (const depName of Object.keys(root.package.dependencies)) {
                    referencedPackages.add(depName);
                }
            }

            const packagePaths = new Set<string>();
            for (const pkg of referencedPackages) {
                addPackageAndDependenciesToSet(root, pkg, packagePaths, excludedPackages);
            }

            resolve(packagePaths);
        });
    });
}

// addPackageAndDependenciesToSet adds all required dependencies for the requested pkg name from the given root package
// into the set.  It will recurse into all dependencies of the package.
function addPackageAndDependenciesToSet(
    root: readPackageTree.Node, pkg: string, packagePaths: Set<string>, excludedPackages: Set<string>) {
    // Don't process this packages if it was in the set the user wants to exclude.

    // Also, exclude it if it's an @pulumi package.  These packages are intended for deployment
    // time only and will only bloat up the serialized lambda package.
    if (excludedPackages.has(pkg) ||
        pkg.startsWith("@pulumi")) {

        return;
    }

    const child = findDependency(root, pkg);
    if (!child) {
        console.warn(`Could not include required dependency '${pkg}' in '${filepath.resolve(root.path)}'.`);
        return;
    }

    packagePaths.add(child.path);
    if (child.package.dependencies) {
        for (const dep of Object.keys(child.package.dependencies)) {
            addPackageAndDependenciesToSet(child, dep, packagePaths, excludedPackages);
        }
    }
}

// findDependency searches the package tree starting at a root node (possibly a child) for a match
// for the given name. It is assumed that the tree was correctly constructed such that dependencies
// are resolved to compatible versions in the closest available match starting at the provided root
// and walking up to the head of the tree.
function findDependency(root: readPackageTree.Node | undefined | null, name: string) {
    for (; root; root = root.parent) {
        for (const child of root.children) {
            let childName = child.name;
            // Note: `read-package-tree` returns incorrect `.name` properties for packages in an
            // organization - like `@types/express` or `@protobufjs/path`.  Compute the correct name
            // from the `path` property instead. Match any name that ends with something that looks
            // like `@foo/bar`, such as `node_modules/@foo/bar` or
            // `node_modules/baz/node_modules/@foo/bar.
            const childFolderName = filepath.basename(child.path);
            const parentFolderName = filepath.basename(filepath.dirname(child.path));
            if (parentFolderName[0] === "@") {
                childName = filepath.join(parentFolderName, childFolderName);
            }

            if (childName === name) {
                return child;
            }
        }
    }

    return undefined;
}
