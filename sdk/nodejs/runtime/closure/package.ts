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
        strings.push(...getAllLeafStrings(value));
    }
    return strings;
}

class WildcardMap {
    private datamap: {[srcPrefix: string]: {
        srcSuffix: string;
        modPrefix: string;
        modSuffix: string;
    };};
    constructor() {
        this.datamap = {};
    }
    set(srcPattern: string, modPattern: string) {
        // Assumption: to use a wildcard pattern, each side must be wildcarded
        const [srcPrefix, srcSuffix] = srcPattern.split("*");
        const [modPrefix, modSuffix] = modPattern.split("*"); // modules don't have tail matches
        this.datamap[srcPrefix] = {
            srcSuffix,
            modPrefix,
            modSuffix,
        };
    }
    get(srcName: string) {
        // this is a bit slow.
        for (const [srcPrefix, srcRule] of Object.entries(this.datamap)) { // TODO sort in a short-circuitable manner
            if (!srcName.startsWith(srcPrefix)) {
                continue;
            }
            const [_, srcSuffix] = srcName.split(srcPrefix);
            if (!srcSuffix.endsWith(srcRule.srcSuffix)) {
                // TODO re-check spec
                // improper filetype?
                //return undefined;
            }
            if (srcSuffix.endsWith(srcRule.srcSuffix)) {
                const modSuffix = srcSuffix.substring(0, srcSuffix.lastIndexOf(srcRule.srcSuffix));
                return srcRule.modPrefix + modSuffix;
            }
            return srcRule.modPrefix + srcSuffix;
        }
        return undefined;
    }
}

class ModuleMap {
    readonly name: string;
    private map: {[key: string]: string} = {};
    private wildcardMap: WildcardMap;
    constructor(packageDefinition: PackageDefinition) {
        this.name = packageDefinition.name;
        this.map = {};
        this.wildcardMap = new WildcardMap();
        const exports = packageDefinition.exports;

        if (exports === undefined) {
            return;
        }

        for (const [modName, objectOrPath] of Object.entries(exports)) {
            const leaves = getAllLeafStrings(objectOrPath);
            if (leaves === []) {
                // module not whitelisted
            }
            for (const leaf of leaves) {
                let newModName: string = packageDefinition.name + modName.substr(1);

                if (modName === "." || !modName.startsWith(".")) {
                    newModName = packageDefinition.name;
                }

                const newLeaf = packageDefinition.name + leaf.substr(1);

                if (newLeaf.includes("*")) {
                    // wildcard match
                    this.wildcardMap.set(newLeaf, newModName);
                    continue;
                }

                this.map[newLeaf] = newModName;
            }
        }
    }
    get(srcName: string) {
        return this.map[srcName] ?  this.map[srcName] : this.wildcardMap.get(srcName);
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
