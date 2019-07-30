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

// tslint:disable:max-line-length

import * as fs from "fs";
import * as normalize from "normalize-package-data";
import * as readPackageTree from "read-package-tree";
import * as upath from "upath";
import { log } from "../..";
import * as asset from "../../asset";
import { ResourceError } from "../../errors";
import { Resource } from "../../resource";

/**
 * Options for controlling what gets returned by [computeCodePaths].
 */
export interface CodePathOptions {
    /**
     * Local file/directory paths that we always want to include when producing the Assets to be
     * included for a serialized closure.
     */
    extraIncludePaths?: string[];

    /**
     * Extra packages to include when producing the Assets for a serialized closure.  This can be
     * useful if the packages are acquired in a way that the serialization code does not understand.
     * For example, if there was some sort of module that was pulled in based off of a computed
     * string.
     */
    extraIncludePackages?: string[];

    /**
     * Packages to explicitly exclude from the Assets for a serialized closure.  This can be used
     * when clients want to trim down the size of a closure, and they know that some package won't
     * ever actually be needed at runtime, but is still a dependency of some package that is being
     * used at runtime.
     */
    extraExcludePackages?: string[];

    /**
     * The resource to log any errors we encounter against.
     */
    logResource?: Resource;
}

/**
 * computeCodePaths computes the local node_module paths to include in an uploaded cloud 'Lambda'.
 * Specifically, it will examine the package.json for the caller's code, and will transitively walk
 * it's 'dependencies' section to determine what packages should be included.
 *
 * During this walk, if a package is encountered that contains a `"pulumi": { ... }` section then
 * the normal `"dependencies": { ... }` section of that package will not be included.  These are
 * "pulumi" packages, and those dependencies are only intended for use at deployment time. However,
 * a "pulumi" package can also specify package that should be available at cloud-runtime.  These
 * packages are found in a `"runtimeDependencies": { ... }` section in the package.json file with
 * the same format as the normal "dependencies" section.
 *
 * See [CodePathOptions] for information on ways to control and configure the final set of paths
 * included in the resultant asset/archive map.
 *
 * Note: this functionality is specifically intended for use by downstream library code that is
 * determining what is needed for a cloud-lambda.  i.e. the aws.serverless.Function or
 * azure.serverless.FunctionApp libraries.  In general, other clients should not need to use this
 * helper.
 */
export async function computeCodePaths(options?: CodePathOptions): Promise<Map<string, asset.Asset | asset.Archive>>;

/**
 * @deprecated Use the [computeCodePaths] overload that takes a [CodePathOptions] instead.
 */
export async function computeCodePaths(extraIncludePaths?: string[], extraIncludePackages?: string[], extraExcludePackages?: string[]): Promise<Map<string, asset.Asset | asset.Archive>>;

export async function computeCodePaths(
    optionsOrExtraIncludePaths?: string[] | CodePathOptions,
    extraIncludePackages?: string[],
    extraExcludePackages?: string[]): Promise<Map<string, asset.Asset | asset.Archive>> {

    let options: CodePathOptions;
    if (Array.isArray(optionsOrExtraIncludePaths)) {
        log.warn("'function computeCodePaths(string[])' is deprecated. Use the [computeCodePaths] overload that takes a [CodePathOptions] instead.");
        options = {
            extraIncludePaths: optionsOrExtraIncludePaths,
            extraIncludePackages,
            extraExcludePackages,
        };
    }
    else if (optionsOrExtraIncludePaths) {
        options = optionsOrExtraIncludePaths;
    }
    else {
        options = {};
    }

    return computeCodePathsWorker(options);
}

async function computeCodePathsWorker(options: CodePathOptions): Promise<Map<string, asset.Asset | asset.Archive>> {
    // Construct the set of paths to include in the archive for upload.

    // Find folders for all packages requested by the user.  Note: all paths in this should
    // be normalized.
    const normalizedPathSet = await allFoldersForPackages(
        new Set<string>(options.extraIncludePackages || []),
        new Set<string>(options.extraExcludePackages || []),
        options.logResource);

    // Add all paths explicitly requested by the user
    const extraIncludePaths = options.extraIncludePaths || [];
    for (const path of extraIncludePaths) {
        normalizedPathSet.add(upath.normalize(path));
    }

    const codePaths: Map<string, asset.Asset | asset.Archive> = new Map();

    // For each of the required paths, add the corresponding FileArchive or FileAsset to the
    // AssetMap.
    for (const normalizedPath of normalizedPathSet) {
        // Don't include a path if there is another path higher up that will include this one.
        if (isSubsumedByHigherPath(normalizedPath, normalizedPathSet)) {
            continue;
        }

        // The Asset model does not support a consistent way to embed a file-or-directory into an
        // `AssetArchive`, so we stat the path to figure out which it is and use the appropriate
        // Asset constructor.
        const stats = fs.lstatSync(normalizedPath);
        if (stats.isDirectory()) {
            codePaths.set(normalizedPath, new asset.FileArchive(normalizedPath));
        }
        else {
            codePaths.set(normalizedPath, new asset.FileAsset(normalizedPath));
        }
    }

    return codePaths;
}

function isSubsumedByHigherPath(normalizedPath: string, normalizedPathSet: Set<string>): boolean {
    for (const otherNormalizedPath of normalizedPathSet) {
        if (normalizedPath.length > otherNormalizedPath.length &&
            normalizedPath.startsWith(otherNormalizedPath)) {

            // Have to make sure we're actually a sub-directory of that other path.  For example,
            // if we have:  node_modules/mime-types, that's not subsumed by node_modules/mime
            const nextChar = normalizedPath.charAt(otherNormalizedPath.length);
            return nextChar === "/";
        }
    }

    return false;
}

// allFolders computes the set of package folders that are transitively required by the root
// 'dependencies' node in the client's project.json file.
function allFoldersForPackages(
        includedPackages: Set<string>,
        excludedPackages: Set<string>,
        logResource: Resource | undefined): Promise<Set<string>> {
    return new Promise((resolve, reject) => {
        readPackageTree(".", <any>undefined, (err: any, root: readPackageTree.Node) => {
            try {
                if (err) {
                    return reject(err);
                }

                // read-package-tree defers to read-package-json to parse the project.json file. If that
                // fails, root.error is set to the underlying error.  Unfortunately, read-package-json is
                // very finicky and can fail for reasons that are not relevant to us.  For example, it
                // can fail if a "version" string is not a legal semver.  We still want to proceed here
                // as this is not an actual problem for determining the set of dependencies.
                if (root.error) {
                    if (!root.realpath) {
                        throw new ResourceError(
                            "Failed to parse package.json. Underlying issue:\n  " + root.error.toString(), logResource);
                    }

                    // From: https://github.com/npm/read-package-tree/blob/5245c6e50d7f46ae65191782622ec75bbe80561d/rpt.js#L121
                    root.package = computeDependenciesDirectlyFromPackageFile(
                        upath.join(root.realpath, "package.json"), logResource);
                }

                // This is the core starting point of the algorithm.  We use readPackageTree to get
                // the package.json information for this project, and then we start by walking the
                // .dependencies node in that package.  Importantly, we do not look at things like
                // .devDependencies or or .peerDependencies.  These are not what are considered part
                // of the final runtime configuration of the app and should not be uploaded.
                const referencedPackages = new Set<string>(includedPackages);
                if (root.package && root.package.dependencies) {
                    for (const depName of Object.keys(root.package.dependencies)) {
                        referencedPackages.add(depName);
                    }
                }

                // package.json files can contain circularities.  For example es6-iterator depends
                // on es5-ext, which depends on es6-iterator, which depends on es5-ext:
                // https://github.com/medikoo/es6-iterator/blob/0eac672d3f4bb3ccc986bbd5b7ffc718a0822b74/package.json#L20
                // https://github.com/medikoo/es5-ext/blob/792c9051e5ad9d7671dd4e3957eee075107e9e43/package.json#L29
                //
                // So keep track of the paths we've looked and don't recurse if we hit something again.
                const seenPaths = new Set<string>();

                const normalizedPackagePaths = new Set<string>();
                for (const pkg of referencedPackages) {
                    addPackageAndDependenciesToSet(
                        root, pkg, seenPaths, normalizedPackagePaths, excludedPackages);
                }

                return resolve(normalizedPackagePaths);
            }
            catch (error) {
                return reject(error);
            }
        });
    });
}

function computeDependenciesDirectlyFromPackageFile(path: string, logResource: Resource | undefined): any {
    // read the package.json file in directly.  if any of these fail an error will be thrown
    // and bubbled back out to user.
    const contents = readFile();
    const data = parse();

    // 'normalize-package-data' can throw if 'version' isn't a valid string.  We don't care about
    // 'version' so just delete it.
    // https://github.com/npm/normalize-package-data/blob/df8ea05b8cd38531e8b70ac7906f420344f55bab/lib/fixer.js#L191
    delete data.version;

    // 'normalize-package-data' can throw if 'name' isn't a valid string.  We don't care about
    // 'name' so just delete it.
    // https://github.com/npm/normalize-package-data/blob/df8ea05b8cd38531e8b70ac7906f420344f55bab/lib/fixer.js#L211
    delete data.name;

    normalize(data);

    return data;

    function readFile() {
        try {
            return fs.readFileSync(path);
        } catch (err) {
            throw new ResourceError(`Error reading file '${path}' when computing package dependencies. ${err}`, logResource);
        }
    }

    function parse() {
        try {
            return JSON.parse(contents.toString());
        } catch (err) {
            throw new ResourceError(`Error parsing file '${path}' when computing package dependencies. ${err}`, logResource);
        }
    }
}

// addPackageAndDependenciesToSet adds all required dependencies for the requested pkg name from the given root package
// into the set.  It will recurse into all dependencies of the package.
function addPackageAndDependenciesToSet(
    root: readPackageTree.Node, pkg: string, seenPaths: Set<string>,
    normalizedPackagePaths: Set<string>, excludedPackages: Set<string>) {

    // Don't process this packages if it was in the set the user wants to exclude.
    if (excludedPackages.has(pkg)) {
        return;
    }

    const child = findDependency(root, pkg);
    if (!child) {
        console.warn(`Could not include required dependency '${pkg}' in '${upath.resolve(root.path)}'.`);
        return;
    }

    // Don't process a child path if we've already encountered it.
    const normalizedPath = upath.normalize(child.path);
    if (seenPaths.has(normalizedPath)) {
        return;
    }
    seenPaths.add(normalizedPath);

    if (child.package.pulumi) {
        // This was a pulumi deployment-time package.  Check if it had a:
        //
        //    `pulumi: { runtimeDependencies: ... }`
        //
        // section.  In this case, we don't want to add this specific package, but we do want to
        // include all the runtime dependencies it says are necessary.
        recurse(child.package.pulumi.runtimeDependencies);
    }
    else if (pkg.startsWith("@pulumi")) {
        // exclude it if it's an @pulumi package.  These packages are intended for deployment
        // time only and will only bloat up the serialized lambda package.  Note: this code can
        // be removed once all pulumi packages add a "pulumi" section to their package.json.
        return;
    }
    else {
        // Normal package.  Add the normalized path to it, and all transitively add all of its
        // dependencies.
        normalizedPackagePaths.add(normalizedPath);
        recurse(child.package.dependencies);
    }

    return;

    function recurse(dependencies: any) {
        if (dependencies) {
            for (const dep of Object.keys(dependencies)) {
                addPackageAndDependenciesToSet(
                    child!, dep, seenPaths, normalizedPackagePaths, excludedPackages);
            }
        }
    }
}

// findDependency searches the package tree starting at a root node (possibly a child) for a match
// for the given name. It is assumed that the tree was correctly constructed such that dependencies
// are resolved to compatible versions in the closest available match starting at the provided root
// and walking up to the head of the tree.
function findDependency(root: readPackageTree.Node | undefined | null, name: string): readPackageTree.Node | undefined {
    for (; root; root = root.parent) {
        for (const child of root.children) {
            let childName = child.name;
            // Note: `read-package-tree` returns incorrect `.name` properties for packages in an
            // organization - like `@types/express` or `@protobufjs/path`.  Compute the correct name
            // from the `path` property instead. Match any name that ends with something that looks
            // like `@foo/bar`, such as `node_modules/@foo/bar` or
            // `node_modules/baz/node_modules/@foo/bar.
            const childFolderName = upath.basename(child.path);
            const parentFolderName = upath.basename(upath.dirname(child.path));
            if (parentFolderName[0] === "@") {
                childName = upath.join(parentFolderName, childFolderName);
            }

            if (childName === name) {
                return child;
            }
        }
    }

    return undefined;
}
