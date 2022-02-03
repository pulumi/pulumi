import * as fs from "fs";
import * as upath from "upath";

type Exports = string | {[key: string]: SubExports}
type SubExports = string | {[key: string]: SubExports} | null

type PackageDefinition = {
    name: string,
    exports?: Exports,
}

function getPackageDefinition(path: string): PackageDefinition {
    const directories =  path.split(upath.sep);
    while(directories.length > 0) {
        let curPath = directories.join(upath.sep);
        let packageDefinitionPath = `${curPath}/package.json`;
        try {
            require.resolve(packageDefinitionPath);
            return require(packageDefinitionPath)
        } catch (e) {
            
        }
        directories.pop()
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
    for (const [key, value] of Object.entries(objectOrPath)) {
        strings.push(...getAllLeafStrings(value));
    }
    return strings
}

class WildcardMap {
    private datamap: {[srcPrefix: string]: {
        srcSuffix: string;
        modPrefix: string;
        modSuffix: string;
    }}
    constructor() {
        this.datamap = {};
    }
    set(srcPattern: string, modPattern: string) {
        // Assumption: to use a wildcard pattern, each side must be wildcarded
        const [srcPrefix, srcSuffix] = srcPattern.split('*');
        const [modPrefix, modSuffix] = modPattern.split('*'); // modules don't have tail matches
        this.datamap[srcPrefix] = {
            srcSuffix,
            modPrefix,
            modSuffix,
        }
        return
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
                const modSuffix = srcSuffix.substring(0, srcSuffix.lastIndexOf(srcRule.srcSuffix))
                return srcRule.modPrefix + modSuffix;
            }
            return srcRule.modPrefix + srcSuffix;
        }
        return undefined;
    }
}

class ModuleMap {
    private map: {[key: string]: string} = {};
    private wildcardMap: WildcardMap;
    constructor(packageDefinition: PackageDefinition) {
        this.map = {};
        this.wildcardMap = new WildcardMap();
        const exports = packageDefinition.exports;

        if (exports === undefined) {
            return;
        }

        for (let [modName, objectOrPath] of Object.entries(exports)) {
            const leaves = getAllLeafStrings(objectOrPath);
            if (leaves === []) {
                // module not whitelisted
            }
            for (let leaf of leaves) {
                if (!modName.startsWith('.')) {
                    modName = '.'
                }
                modName = packageDefinition.name + modName.substr(1)
                leaf = packageDefinition.name + leaf.substr(1)

                if (leaf.includes('*')) {
                    // wildcard match
                    this.wildcardMap.set(leaf, modName);
                    continue
                }

                this.map[leaf] = modName;
            }
        }

    }
    get(srcName: string) {
        return this.map[srcName] ?  this.map[srcName] : this.wildcardMap.get(srcName)
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
    console.log(typeof packageDefinition.exports)
    if (typeof packageDefinition.exports === "object") {
        const modMap = new ModuleMap(packageDefinition)
        const modulePath = modMap.get(path);
        return modulePath
    }

    return path;
}