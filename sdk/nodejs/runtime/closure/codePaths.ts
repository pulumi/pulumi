// Copyright 2016, Pulumi Corporation.
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
import { glob } from "fs/promises";
import normalize from "normalize-package-data";
import * as arborist from "@npmcli/arborist";
import * as upath from "upath";
import { log } from "../..";
import * as asset from "../../asset";
import { ResourceError } from "../../errors";
import { Resource } from "../../resource";

/**
 * Options for controlling what gets returned by {@link computeCodePaths}.
 */
export interface CodePathOptions {
    /**
     * Local file/directory paths that we always want to include when producing
     * the assets to be included for a serialized closure.
     */
    extraIncludePaths?: string[];

    /**
     * Extra packages to include when producing the assets for a serialized
     * closure. This can be useful if the packages are acquired in a way that
     * the serialization code does not understand. For example, if there was
     * some sort of module that was pulled in based off of a computed string.
     */
    extraIncludePackages?: string[];

    /**
     * Packages to explicitly exclude from the assets for a serialized closure.
     * This can be used when clients want to trim down the size of a closure,
     * and they know that some package won't ever actually be needed at runtime,
     * but is still a dependency of some package that is being used at runtime.
     */
    extraExcludePackages?: string[];

    /**
     * The resource to log any errors we encounter against.
     */
    logResource?: Resource;
}

/**
 * Computes the local `node_module` paths to include in an uploaded cloud
 * "lambda". Specifically, it will examine the `package.json` for the caller's
 * code and transitively walk its `dependencies` section to determine what
 * packages should be included.
 *
 * During this walk, if a package is encountered that contains a `"pulumi": {
 * ... }` section then the normal `"dependencies": { ... }` section of that
 * package will not be included.  These are "pulumi" packages, and those
 * dependencies are only intended for use at deployment time. However, a
 * "pulumi" package can also specify package that should be available at
 * cloud-runtime.  These packages are found in a `"runtimeDependencies": { ...
 * }` section in the `package.json` file with the same format as the normal
 * `dependencies` section.
 *
 * See {@link CodePathOptions} for information on ways to control and configure
 * the final set of paths included in the resultant asset/archive map.
 *
 * Note: this functionality is specifically intended for use by downstream
 * library code that is determining what is needed for a cloud-lambda.  i.e. the
 * `aws.serverless.Function` or `azure.serverless.FunctionApp` libraries. In
 * general, other clients should not need to use this helper.
 */
export async function computeCodePaths(options?: CodePathOptions): Promise<Map<string, asset.Asset | asset.Archive>>;

/**
 * @deprecated
 *  Use the {@link computeCodePaths} overload that takes a
 *  {@link CodePathOptions} instead.
 */
export async function computeCodePaths(
    extraIncludePaths?: string[],
    extraIncludePackages?: string[],
    extraExcludePackages?: string[],
): Promise<Map<string, asset.Asset | asset.Archive>>;

export async function computeCodePaths(
    optionsOrExtraIncludePaths?: string[] | CodePathOptions,
    extraIncludePackages?: string[],
    extraExcludePackages?: string[],
): Promise<Map<string, asset.Asset | asset.Archive>> {
    let options: CodePathOptions;
    if (Array.isArray(optionsOrExtraIncludePaths)) {
        log.warn(
            "'function computeCodePaths(string[])' is deprecated. Use the [computeCodePaths] overload that takes a [CodePathOptions] instead.",
        );
        options = {
            extraIncludePaths: optionsOrExtraIncludePaths,
            extraIncludePackages,
            extraExcludePackages,
        };
    } else {
        options = optionsOrExtraIncludePaths || {};
    }

    return computeCodePathsWorker(options);
}

async function computeCodePathsWorker(options: CodePathOptions): Promise<Map<string, asset.Asset | asset.Archive>> {
    // Construct the set of paths to include in the archive for upload.

    // Find folders for all packages requested by the user.  Note: all paths in this should
    // be normalized. Keys are the paths at which the packages are included in the archive,
    // values are the on-disk paths the contents are read from. The two differ when a package
    // is symlinked, e.g. with pnpm's virtual store or workspace packages.
    const packagePaths = await allFoldersForPackages(
        new Set<string>(options.extraIncludePackages || []),
        new Set<string>(options.extraExcludePackages || []),
        options.logResource,
    );

    // Add all paths explicitly requested by the user
    const extraIncludePaths = options.extraIncludePaths || [];
    for (const path of extraIncludePaths) {
        const normalizedPath = upath.normalize(path);
        packagePaths.set(normalizedPath, normalizedPath);
    }

    const codePaths: Map<string, asset.Asset | asset.Archive> = new Map();

    // For each of the required paths, add the corresponding FileArchive or FileAsset to the
    // AssetMap.
    const sourcePaths = new Set<string>(packagePaths.values());
    for (const [archivePath, sourcePath] of packagePaths) {
        // Don't include a path if there is another path higher up that will include this one.
        if (isSubsumedByHigherPath(sourcePath, sourcePaths)) {
            continue;
        }

        // The Asset model does not support a consistent way to embed a file-or-directory into an
        // `AssetArchive`, so we stat the path to figure out which it is and use the appropriate
        // Asset constructor.
        const stats = fs.statSync(sourcePath);
        if (stats.isDirectory()) {
            codePaths.set(archivePath, new asset.FileArchive(sourcePath));
        } else {
            codePaths.set(archivePath, new asset.FileAsset(sourcePath));
        }
    }

    return codePaths;
}

function isSubsumedByHigherPath(normalizedPath: string, normalizedPathSet: Set<string>): boolean {
    for (const otherNormalizedPath of normalizedPathSet) {
        if (normalizedPath.length > otherNormalizedPath.length && normalizedPath.startsWith(otherNormalizedPath)) {
            // Have to make sure we're actually a sub-directory of that other path.  For example,
            // if we have:  node_modules/mime-types, that's not subsumed by node_modules/mime
            const nextChar = normalizedPath.charAt(otherNormalizedPath.length);
            return nextChar === "/";
        }
    }

    return false;
}

/**
 * Searches for and returns the first directory path starting from a given
 * directory that contains the given file to find. Recursively searches up the
 * directory tree until it finds the file or returns `null` when it can't find
 * anything.
 * */
function searchUp(currentDir: string, fileToFind: string): string | null {
    if (fs.existsSync(upath.join(currentDir, fileToFind))) {
        return currentDir;
    }
    const parentDir = upath.resolve(currentDir, "..");
    if (currentDir === parentDir) {
        return null;
    }
    return searchUp(parentDir, fileToFind);
}

/**
 * Detects if we are in a Yarn/NPM workspace setup, and returns the root of the
 * workspace. If we are not in a workspace setup, it returns `null`.
 *
 * @internal
 */
export async function findWorkspaceRoot(startingPath: string): Promise<string | null> {
    const stat = fs.statSync(startingPath);
    if (!stat.isDirectory()) {
        startingPath = upath.dirname(startingPath);
    }
    const packageJSONDir = searchUp(startingPath, "package.json");
    if (packageJSONDir === null) {
        return null;
    }
    // We start at the location of the first `package.json` we find.
    let currentDir = packageJSONDir;
    let nextDir = upath.dirname(currentDir);
    while (currentDir !== nextDir) {
        const p = upath.join(currentDir, "package.json");
        if (!fs.existsSync(p)) {
            currentDir = nextDir;
            nextDir = upath.dirname(currentDir);
            continue;
        }
        const workspaces = parseWorkspaces(p);
        for (const workspace of workspaces) {
            const pattern = upath.join(currentDir, workspace, "package.json");
            const normalized = upath.normalizeTrim(upath.join(packageJSONDir, "package.json"));
            for await (const f of glob(pattern)) {
                if (upath.normalizeTrim(f) === normalized) {
                    return currentDir;
                }
            }
        }
        currentDir = nextDir;
        nextDir = upath.dirname(currentDir);
    }
    return null;
}

function parseWorkspaces(packageJsonPath: string): string[] {
    const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, "utf8"));
    if (packageJson.workspaces && Array.isArray(packageJson.workspaces)) {
        return packageJson.workspaces;
    }
    if (packageJson.workspaces?.packages && Array.isArray(packageJson.workspaces.packages)) {
        return packageJson.workspaces.packages;
    }
    return [];
}

/**
 * Computes the package folders that are transitively required by the root
 * `dependencies` node in the client's `package.json` file. Returns a map from
 * the path at which the package is included in the archive to the on-disk path
 * of the package's contents.
 *
 * The two paths differ when packages are symlinked, most notably by pnpm,
 * which lays out node_modules as symlinks into a versioned virtual store:
 *
 *     node_modules/foo -> node_modules/.pnpm/foo@1.0.0/node_modules/foo
 *     node_modules/.pnpm/foo@1.0.0/node_modules/bar -> node_modules/.pnpm/bar@2.0.0/node_modules/bar
 *
 * The archive instead recreates the physical layout npm would produce:
 * symlinks cannot be represented in the archive, store paths embed version
 * numbers and would change on every upgrade, and the serialized code
 * `require`s packages by name, which Node resolves from
 * `node_modules/<name>`. Virtual store paths are therefore flattened to
 * `node_modules/<name>`, and contents are read from the package's real
 * directory. This also covers symlinked workspace packages.
 */
async function allFoldersForPackages(
    includedPackages: Set<string>,
    excludedPackages: Set<string>,
    logResource: Resource | undefined,
): Promise<Map<string, string>> {
    // the working directory is the directory containing the package.json file
    let workingDir = searchUp(".", "package.json");
    if (workingDir === null) {
        // we couldn't find a directory containing package.json
        // searching up from the current directory
        throw new ResourceError("Failed to find package.json.", logResource);
    }
    workingDir = upath.resolve(workingDir);

    // This is the core starting point of the algorithm.  We read the
    // package.json information for this project, and then we start by walking
    // the .dependencies node in that package.  Importantly, we do not look at
    // things like .devDependencies or or .peerDependencies.  These are not
    // what are considered part of the final runtime configuration of the app
    // and should not be uploaded.
    const referencedPackages = new Set<string>(includedPackages);
    const packageJSON = computeDependenciesDirectlyFromPackageFile(upath.join(workingDir, "package.json"), logResource);
    if (packageJSON.dependencies) {
        for (const depName of Object.keys(packageJSON.dependencies)) {
            referencedPackages.add(depName);
        }
    }

    // Find the workspace root, fallback to current working directory if we are not in a workspaces setup.
    let workspaceRoot = (await findWorkspaceRoot(workingDir)) || workingDir;
    // Ensure workingDir is a relative path so we get relative paths in the
    // output. If we have absolute paths, AWS lambda might not find the
    // dependencies.
    workspaceRoot = upath.relative(upath.resolve("."), workspaceRoot);

    // Read package tree from the workspace root to ensure we can find all
    // packages in the workspace.  We then call addPackageAndDependenciesToSet
    // to recursively add all the dependencies of the referenced packages.
    const arb = new arborist.Arborist({ path: workspaceRoot });
    const root = await arb.loadActual();

    // package.json files can contain circularities.  For example es6-iterator depends
    // on es5-ext, which depends on es6-iterator, which depends on es5-ext:
    // https://github.com/medikoo/es6-iterator/blob/0eac672d3f4bb3ccc986bbd5b7ffc718a0822b74/package.json#L20
    // https://github.com/medikoo/es5-ext/blob/792c9051e5ad9d7671dd4e3957eee075107e9e43/package.json#L29
    //
    // So keep track of the paths we've looked and don't recurse if we hit something again.
    const seenPaths = new Set<string>();

    const packagePaths = new Map<string, string>();

    // Pre-claim the archive paths for the root's direct dependencies so that a transitive
    // dependency processed earlier in the loop below can never claim `node_modules/<name>`
    // with a different version of a package that is also a direct dependency.
    for (const pkg of referencedPackages) {
        if (excludedPackages.has(pkg) || pkg.startsWith("@pulumi")) {
            continue;
        }
        const child = findDependency(root, pkg);
        if (!child || child.package.pulumi) {
            continue;
        }
        const { archivePath, sourcePath } = packageArchiveEntry(child);
        if (!packagePaths.has(archivePath)) {
            packagePaths.set(archivePath, sourcePath);
        }
    }

    for (const pkg of referencedPackages) {
        addPackageAndDependenciesToMap(
            root,
            pkg,
            seenPaths,
            packagePaths,
            excludedPackages,
            /*dependentPath*/ undefined,
        );
    }

    return packagePaths;
}

function computeDependenciesDirectlyFromPackageFile(path: string, logResource: Resource | undefined): any {
    // read the package.json file in directly.  if any of these fail an error will be thrown
    // and bubbled back out to user.
    const contents = readFile();
    const data = parse();

    // 'normalize-package-data' can throw if 'version' isn't a valid string.  We don't care about
    // 'version' so just delete it.
    // https://github.com/npm/normalize-package-data/blob/df8ea05b8cd38531e8b70ac7906f420344f55bab/lib/fixer.js#L191
    data.version = undefined;

    // 'normalize-package-data' can throw if 'name' isn't a valid string.  We don't care about
    // 'name' so just delete it.
    // https://github.com/npm/normalize-package-data/blob/df8ea05b8cd38531e8b70ac7906f420344f55bab/lib/fixer.js#L211
    data.name = undefined;

    normalize(data);

    return data;

    function readFile() {
        try {
            return fs.readFileSync(path);
        } catch (err) {
            throw new ResourceError(
                `Error reading file '${path}' when computing package dependencies. ${err}`,
                logResource,
            );
        }
    }

    function parse() {
        try {
            return JSON.parse(contents.toString());
        } catch (err) {
            throw new ResourceError(
                `Error parsing file '${path}' when computing package dependencies. ${err}`,
                logResource,
            );
        }
    }
}

/**
 * Computes the path at which a package is included in the archive, and the
 * on-disk path its contents are read from.
 */
function packageArchiveEntry(child: arborist.Node | arborist.Link): { archivePath: string; sourcePath: string } {
    const nominalPath = upath.relative(upath.resolve("."), upath.resolve(child.path));
    const sourcePath = upath.relative(upath.resolve("."), upath.resolve(child.realpath));
    return { archivePath: packageArchivePath(nominalPath, child.name), sourcePath };
}

/**
 * Returns the path at which a package with the given nominal node_modules
 * location should be included in the archive.
 *
 * @internal exported for testing.
 */
export function packageArchivePath(nominalPath: string, packageName: string): string {
    if (nominalPath.split("/").includes(".pnpm")) {
        return `node_modules/${packageName}`;
    }
    return nominalPath;
}

/**
 * Adds all required dependencies for the requested package name from the given
 * root package into the map. It will recurse into all dependencies of the
 * package.
 */
function addPackageAndDependenciesToMap(
    root: arborist.Node,
    pkg: string,
    seenPaths: Set<string>,
    packagePaths: Map<string, string>,
    excludedPackages: Set<string>,
    dependentPath: string | undefined,
) {
    // Don't process this packages if it was in the set the user wants to exclude.
    if (excludedPackages.has(pkg)) {
        return;
    }

    const child = findDependency(root, pkg);
    if (!child) {
        console.warn(`Could not include required dependency '${pkg}' in '${upath.resolve(root.path)}'.`);
        return;
    }

    const { archivePath, sourcePath } = packageArchiveEntry(child);

    if (child.package.pulumi) {
        // This was a pulumi deployment-time package.  Check if it had a:
        //
        //    `pulumi: { runtimeDependencies: ... }`
        //
        // section.  In this case, we don't want to add this specific package, but we do want to
        // include all the runtime dependencies it says are necessary.
        if (seenPaths.has(sourcePath)) {
            return;
        }
        seenPaths.add(sourcePath);
        recurse(child.package.pulumi.runtimeDependencies, dependentPath);
        return;
    } else if (pkg.startsWith("@pulumi")) {
        // exclude it if it's an @pulumi package.  These packages are intended for deployment
        // time only and will only bloat up the serialized lambda package.  Note: this code can
        // be removed once all pulumi packages add a "pulumi" section to their package.json.
        return;
    }

    // Normal package. Determine where to place it in the archive, then transitively add all
    // of its dependencies.
    let childArchivePath = archivePath;
    const existing = packagePaths.get(childArchivePath);
    if (existing !== undefined && existing !== sourcePath && dependentPath !== undefined) {
        // Another version of this package already occupies this location in the archive.
        // Mirror npm's layout by nesting this copy under its dependent, so that Node's module
        // resolution picks the right version at runtime.
        childArchivePath = `${dependentPath}/node_modules/${child.name}`;
    }
    const nested = packagePaths.get(childArchivePath);
    if (nested === undefined) {
        packagePaths.set(childArchivePath, sourcePath);
    } else if (nested !== sourcePath) {
        log.warn(
            `Cannot include multiple versions of package '${child.name}' at '${childArchivePath}': ` +
                `keeping '${nested}', ignoring '${sourcePath}'.`,
        );
        return;
    }

    // Don't recurse into a package's dependencies if we've already done so.
    if (seenPaths.has(sourcePath)) {
        return;
    }
    seenPaths.add(sourcePath);
    recurse(child.package.dependencies, childArchivePath);

    return;

    function recurse(dependencies: any, parentArchivePath: string | undefined) {
        if (dependencies) {
            for (const dep of Object.keys(dependencies)) {
                let next: arborist.Node;
                if (child?.isLink) {
                    next = child.target;
                } else {
                    next = child!;
                }
                addPackageAndDependenciesToMap(
                    next!,
                    dep,
                    seenPaths,
                    packagePaths,
                    excludedPackages,
                    parentArchivePath,
                );
            }
        }
    }
}

/**
 * Searches the package tree starting at a root node (possibly a child) for a
 * match for the given name. It is assumed that the tree was correctly
 * constructed such that dependencies are resolved to compatible versions in the
 * closest available match starting at the provided root and walking up to the
 * head of the tree.
 */
function findDependency(
    root: arborist.Node | undefined | null,
    name: string,
): arborist.Node | arborist.Link | undefined {
    while (root) {
        for (const [childName, child] of root.children) {
            if (childName === name) {
                return child;
            }
        }
        if (root.parent) {
            root = root.parent;
        } else if (root.root !== root) {
            root = root.root; // jump up to the workspace root
        } else {
            break;
        }
    }

    return undefined;
}
