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

import * as fs from "fs";
import * as upath from "upath";

type Exports = string | {[key: string]: SubExports};
type SubExports = string | {[key: string]: SubExports} | null;

type PackageDefinition = {
    name: string;
    exports?: Exports;
};

function getPackageDefinition(path: string): PackageDefinition {
    const directories =  path.split(upath.sep);
    while(directories.length > 0) {
        const curPath = directories.join(upath.sep);
        const packageDefinitionPath = `${curPath}/package.json`;
        try {
            require.resolve(packageDefinitionPath);
            return require(packageDefinitionPath);
        } catch (e) {
            // no package.json found. check next
            directories.pop();
            continue;
        }
    }
    throw new Error(`no package.json found for ${path}`);
}

function getAllLeafStrings(objectOrPath: SubExports): string[] {
    if (objectOrPath === null) {
        return [];
    }
    if (typeof objectOrPath === "string") {
        return [objectOrPath];
    }
    const strings: string[] = [];
    for (const [_, value] of Object.entries(objectOrPath)) {
        const leaves = getAllLeafStrings(value)
        if (leaves === []) {
            // if there's an environment where this export does not work
            // don't allow requires from this match
            return [];
        }
        strings.push(...leaves);
    }
    return strings;
}

function patternKeyCompare(a: string, b: string) {
  const aPatternIndex = a.indexOf('*');
  const bPatternIndex = b.indexOf('*');
  const baseLenA = aPatternIndex === -1 ? a.length : aPatternIndex + 1;
  const baseLenB = bPatternIndex === -1 ? b.length : bPatternIndex + 1;
  if (baseLenA > baseLenB) return -1;
  if (baseLenB > baseLenA) return 1;
  if (aPatternIndex === -1) return 1;
  if (bPatternIndex === -1) return -1;
  if (a.length > b.length) return -1;
  if (b.length > a.length) return 1;
  return 0;
}
class WildcardMap {
    private map: {[srcPrefix: string]: string}
    private datamap: {[srcPrefix: string]: {
        srcSuffix: string;
        modPrefix: string;
        modSuffix: string;
    };};
    private blacklist: [string, string][];
    constructor() {
        this.datamap = {};
        this.map = {};
        this.blacklist = [];
    }
    setBlacklist(modPattern: string) {
        // we skip blacklisting packages
        // all of the paths are from the require cache
        // we just have to determine a valid path to import from
        const [prefix, suffix] = modPattern.split('*', 2) // there is undefined behavior on more than 1 '*' https://github.com/nodejs/node/blob/b191e66/lib/internal/modules/esm/resolve.js#L664
        this.blacklist.push([prefix, suffix || '']);
    }
    set(newLeaf: string, newModName: string) {
        if (newLeaf.includes("*")) {
            // wildcard match
            this.setWildcard(newLeaf, newModName);
            return;
        }

        this.map[newLeaf] = newModName;
    }
    setWildcard(srcPattern: string, modPattern: string) {
        // Assumption: to use a wildcard pattern, each side must be wildcarded
        const srcSplit = srcPattern.split("*"); // NodeJS doesn't error out when provided multiple '*'.
        const modSplit = modPattern.split("*");
        if (srcPattern.length > 2 || modSplit.length > 2) {
            throw new Error("multiple wildcards in single export target specification")
        }
        const [srcPrefix, srcSuffix] = srcSplit;
        const [modPrefix, modSuffix] = modSplit; // there is undefined behavior on more than 1 '*' https://github.com/nodejs/node/blob/b191e66/lib/internal/modules/esm/resolve.js#L664
        this.datamap[srcPrefix] = {
            modPrefix,
            modSuffix: (modSuffix || ''),
            srcSuffix: (srcSuffix || ''),
        };
    }
    get(srcName: string): string | undefined {
        if (this.map[srcName]) {
            return this.map[srcName];
        }
        for (const [blacklistPrefix, blacklistSuffix] of this.blacklist) {
            if ((upath.extname(srcName) ? srcName : srcName + upath.sep).startsWith(blacklistPrefix) && srcName.endsWith(blacklistSuffix)){
                return undefined;
            }
        }
        // this is a bit slow.
        const sortedKeys = Object.keys(this.datamap).sort(patternKeyCompare)
        for (const srcPrefix of sortedKeys) { // TODO sort in a short-circuitable manner
            const srcRule = this.datamap[srcPrefix]
            if (!srcName.startsWith(srcPrefix) || !srcName.endsWith(srcRule.srcSuffix)) {
                continue;
            }

            const srcSubpath = srcName.slice(srcPrefix.length, srcName.length-srcRule.srcSuffix.length);
            const result = srcRule.modPrefix + srcSubpath + srcRule.modSuffix;
            return result;
        }
        return undefined;
    }
}

class ModuleMap {
    readonly name: string;
    private wildcardMap: WildcardMap;
    constructor(packageDefinition: PackageDefinition) {
        this.name = packageDefinition.name;
        this.wildcardMap = new WildcardMap();
        const exports = packageDefinition.exports;

        if (exports === undefined) {
            return;
        }

        for (const [modName, objectOrPath] of Object.entries(exports)) {
            let newModName: string = packageDefinition.name + modName.substr(1);

            if (modName === "." || !modName.startsWith(".")) {
                newModName = packageDefinition.name;
            }
            const leaves = getAllLeafStrings(objectOrPath);
            for (const leaf of leaves) {

                const newLeaf = packageDefinition.name + leaf.substr(1);

                this.wildcardMap.set(newLeaf, newModName);
            }
            if (leaves.length === 0 && newModName.includes('*')) {
                // module not whitelisted
                this.wildcardMap.setBlacklist(newModName);
            }
        }
    }
    get(srcName: string) {
        return this.wildcardMap.get(srcName);
    }
}

export function getModuleFromPath(path: string, packageDefinition?: PackageDefinition) {
    if (packageDefinition === undefined) {
        packageDefinition = getPackageDefinition(path);
    }
    if (packageDefinition.exports === undefined) {
        return path;
    }
    const packageName = packageDefinition.name;
    if (typeof packageDefinition.exports === "string") {
        return packageName;
    }
    if (typeof packageDefinition.exports === "object") {
        const modMap = new ModuleMap(packageDefinition);
        const modulePath = modMap.get(path);
        return modulePath;
    }

    return path;
}
