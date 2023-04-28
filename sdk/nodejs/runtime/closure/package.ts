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

import * as upath from "upath";

type Exports = string | { [key: string]: SubExports };
type SubExports = string | { [key: string]: SubExports } | null;

type PackageDefinition = {
    name: string;
    exports?: Exports;
};

// TODO[issue] handle https://nodejs.org/api/packages.html#package-entry-points
//
// Warning: Introducing the "exports" field prevents consumers of a package from using
// any entry points that are not defined, including the package.json
// (e.g. require("your-package/package.json"). This will likely be a breaking change.

function getPackageDefinition(path: string): PackageDefinition | undefined {
    try {
        const directories = path.split(upath.sep);
        let last: string | undefined = undefined;
        let lastFullPath: string | undefined = undefined;
        while (directories.length > 0) {
            const curPath = directories.join(upath.sep);
            try {
                lastFullPath = require.resolve(curPath);
                last = curPath;
            } catch (e) {
                // current path is not a module
            }
            directories.pop();
        }
        if (last === undefined || lastFullPath === undefined) {
            throw new Error(`no package.json found for ${path}`);
        }
        const packageDefinitionAbsPath = lastFullPath.slice(0, lastFullPath.indexOf(last)) + last + "/package.json";
        return require(packageDefinitionAbsPath);
    } catch (err) {
        return undefined;
    }
}

// a module's implementations are leaves of the document tree.
function getAllLeafStrings(objectOrPath: SubExports, opts?: RequireOpts): string[] {
    if (objectOrPath === null) {
        // module blacklisted return no implementations
        return [];
    }
    if (typeof objectOrPath === "string") {
        return [objectOrPath];
    }
    const strings: string[] = [];
    for (const [key, value] of Object.entries(objectOrPath)) {
        if (opts && !opts.isRequire && key === "require") {
            continue;
        }
        if (opts && !opts.isImport && key === "import") {
            continue;
        }
        const leaves = getAllLeafStrings(value);
        if (leaves.length === 0) {
            // if there's an environment where this export does not work,
            // don't suggest requires from this match as a more preferable path may
            // match this file.
            return [];
        }
        strings.push(...leaves);
    }
    return strings;
}

// from https://github.com/nodejs/node/blob/b191e66ddf/lib/internal/modules/esm/resolve.js#L686
function patternKeyCompare(a: string, b: string) {
    const aPatternIndex = a.indexOf("*");
    const bPatternIndex = b.indexOf("*");
    const baseLenA = aPatternIndex === -1 ? a.length : aPatternIndex + 1;
    const baseLenB = bPatternIndex === -1 ? b.length : bPatternIndex + 1;
    if (baseLenA > baseLenB) {
        return -1;
    }
    if (baseLenB > baseLenA) {
        return 1;
    }
    if (aPatternIndex === -1) {
        return 1;
    }
    if (bPatternIndex === -1) {
        return -1;
    }
    if (a.length > b.length) {
        return -1;
    }
    if (b.length > a.length) {
        return 1;
    }
    return 0;
}

type SrcPrefix = string;
type Rule = [
    SrcPrefix,
    {
        modPrefix: string;
        modSuffix: string;
        srcSuffix: string;
    },
];

function makeRule(srcPattern: string, modPattern: string): Rule {
    const srcSplit = srcPattern.split("*"); // NodeJS doesn't error out when provided multiple '*'.
    const modSplit = modPattern.split("*");
    if (srcSplit.length > 2 || modSplit.length > 2) {
        // there is undefined behavior on more than 1 "*"
        // see https://github.com/nodejs/node/blob/b191e66ddf/lib/internal/modules/esm/resolve.js#L664
        throw new Error("multiple wildcards in single export target specification");
    }
    const [srcPrefix, srcSuffix] = srcSplit;
    const [modPrefix, modSuffix] = modSplit;
    return [
        srcPrefix,
        {
            modPrefix,
            modSuffix: modSuffix || "",
            srcSuffix: srcSuffix || "",
        },
    ];
}

class WildcardMap {
    private map: { [srcPrefix: string]: string };
    private rules: Rule[];
    constructor(matches: [string, string[]][]) {
        this.map = {};
        const rules: Rule[] = [];
        for (const [match, srcPaths] of matches) {
            for (const srcPath of srcPaths) {
                if (srcPath.includes("*")) {
                    // wildcard match
                    rules.push(makeRule(srcPath, match));
                    continue;
                }
                this.map[srcPath] = match;
            }
        }
        this.rules = rules.sort((a, b) => patternKeyCompare(a[0], b[0]));
    }
    get(srcName: string): string | undefined {
        if (this.map[srcName]) {
            return this.map[srcName];
        }
        for (const [srcPrefix, srcRule] of this.rules) {
            if (!srcName.startsWith(srcPrefix) || !srcName.endsWith(srcRule.srcSuffix)) {
                continue;
            }

            const srcSubpath = srcName.slice(srcPrefix.length, srcName.length - srcRule.srcSuffix.length);
            const result = srcRule.modPrefix + srcSubpath + srcRule.modSuffix;
            return result;
        }
        return undefined;
    }
}

function isConditionalSugar(exports: Exports, name: string) {
    // exports sugar does not handle mixing ["./path/to/module"] path keys
    // and ["default"|"require"|"import"] conditional keys
    // details https://github.com/nodejs/node/blob/b191e66ddf/lib/internal/modules/esm/resolve.js#L593
    let isSugar = false;
    for (const key of Object.keys(exports)) {
        if (isSugar && key.startsWith(".")) {
            throw new Error(
                `${name}:package.json "exports" cannot contain some keys starting with "." and some not.` +
                    " The exports object must either be an object of package subpath keys" +
                    " or an object of main entry condition name keys only.",
            );
        }
        if (!key.startsWith(".")) {
            isSugar = true;
            continue;
        }
    }
    return isSugar;
}

class ModuleMap {
    readonly name: string;
    private wildcardMap: WildcardMap;
    constructor(name: string, exports: Exports, opts?: RequireOpts) {
        this.name = name;

        if (isConditionalSugar(exports, name)) {
            // the exports keys are not paths meaning it is an exports sugar we need to simplify
            exports = { ".": exports };
        }

        const rules: [string, string[]][] = [];
        for (const [modPath, objectOrPath] of Object.entries(exports)) {
            const modName: string = name + modPath.slice(1);
            const leaves = getAllLeafStrings(objectOrPath, opts);
            rules.push([modName, leaves.map((leaf) => name + leaf.slice(1))]);
        }
        this.wildcardMap = new WildcardMap(rules);
    }
    get(srcName: string) {
        const modPath = this.wildcardMap.get(srcName);
        if (modPath === undefined) {
            throw new Error(`package.json export path for "${srcName}" not found`);
        }
        return modPath;
    }
}

type RequireOpts = {
    isRequire?: boolean;
    isImport?: boolean;
};

/*
    We need to resolve from a source file path to a valid module export.

    Exports to source file is a many-to-one relationship. Reversing this is a one-to-many relationship.
    Any of the initial exports are aliases to the same module and
    we assume to be semantically equivalent. This makes it a one-to-any relationship.
    for example,
    <./package.json>
        "exports": {
            "./foo.js": "./lib/index.js",
            "./bar.js": "./lib/index.js",
        }
    we will resolve ./lib/index.js into either ./foo.js or ./bar.js
    a module can resolve into many files conditionally, but aliases are treated as equivalent.

    Due to null specifiers for modules and this one-to-many relationship, we assume that anything with a
    null specifier may be unreachable on a different platform and opt for a different alias to cover it if
    it exists.

    Exports ending in "/" will be deprecated by node.


    For more details https://nodejs.org/api/esm.html#resolution-algorithm
*/

export function getModuleFromPath(
    path: string,
    packageDefinition?: PackageDefinition,
    opts: RequireOpts = { isRequire: true },
) {
    packageDefinition = packageDefinition || getPackageDefinition(path);
    if (packageDefinition === undefined) {
        return path;
    }
    if (packageDefinition.exports === undefined) {
        return path;
    }
    if (typeof packageDefinition.exports === "string") {
        return packageDefinition.name;
    }
    if (typeof packageDefinition.exports === "object") {
        const modMap = new ModuleMap(packageDefinition.name, packageDefinition.exports, opts);
        const modulePath = modMap.get(path);
        return modulePath;
    }
    return path;
}
