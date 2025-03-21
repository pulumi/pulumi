// Copyright 2025-2025, Pulumi Corporation.
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

// The typescript import is used for type-checking only. Do not reference it in the emitted code.
// Use `ts` instead to access typescript library functions.
import typescript from "typescript";
import * as path from "path";

// Use the TypeScript shim which allows us to fallback to a vendored version of
// TypeScript if the user has not installed it.
// TODO: we should consider requiring the user to install TypeScript and not
// rely on the shim. In any case, we should add tests for providers with
// different versions of TypeScript in their dependencies, to ensure the
// analyzer code is compatible with all of them.
const ts: typeof typescript = require("../../typescript-shim");

export type PropertyType = "string" | "integer" | "number" | "boolean" | "array" | "object";

export type PropertyDefinition = ({ type: PropertyType } | { $ref: string }) & {
    description?: string;
    optional?: boolean;
    plain?: boolean;
    additionalProperties?: PropertyDefinition;
    items?: PropertyDefinition;
};

export type ComponentDefinition = {
    name: string;
    description?: string;
    inputs: Record<string, PropertyDefinition>;
    outputs: Record<string, PropertyDefinition>;
};

export type TypeDefinition = {
    name: string;
    properties: Record<string, PropertyDefinition>;
    description?: string;
};

export type AnalyzeResult = {
    components: Record<string, ComponentDefinition>;
    typeDefinitions: Record<string, TypeDefinition>;
    packageReferences: Record<string, string>;
};

interface docNode {
    jsDoc?: typescript.JSDoc[];
}

export enum InputOutput {
    Neither = 0,
    Input = 1,
    Output = 2,
}

// Check if any of module exports is a Pulumi Component class.
export function hasPulumiComponents(moduleExports: any): boolean {
    if (!moduleExports) {
        return false;
    }

    for (const key in moduleExports) {
        // Filter unwanted properties from the prototype
        if (!Object.prototype.hasOwnProperty.call(moduleExports, key)) {
            continue;
        }

        const exportedItem = moduleExports[key];

        // Only check function/class items
        if (typeof exportedItem !== "function" || !exportedItem.prototype) {
            continue;
        }

        // Navigate through the prototype chain to find if any of the prototypes are a Pulumi component.
        let proto = exportedItem.prototype;
        while (proto?.__proto__) {
            proto = proto.__proto__;

            // Check if the constructor of this prototype is ComponentResource
            if (
                proto.constructor &&
                (proto.constructor.name === "ComponentResource" || proto.constructor.__pulumiComponentResource === true)
            ) {
                return true;
            }
        }
    }

    return false;
}

export class Analyzer {
    private dir: string;
    private packageJSON: Record<string, any>;
    private providerName: string;
    private checker: typescript.TypeChecker;
    private program: typescript.Program;
    private components: Record<string, ComponentDefinition> = {};
    private typeDefinitions: Record<string, TypeDefinition> = {};
    private packageReferences: Record<string, string> = {};
    private docStrings: Record<string, string> = {};
    private componentNames: Set<string>;
    private programFiles: Set<string>;

    constructor(dir: string, packageJSON: Record<string, any>, componentNames: Set<string>) {
        if (componentNames.size === 0) {
            throw new Error("componentNames cannot be empty - at least one component name must be provided");
        }
        this.dir = dir;
        this.packageJSON = packageJSON;
        this.providerName = packageJSON.name;
        const configPath = `${dir}/tsconfig.json`;
        const config = ts.readConfigFile(configPath, ts.sys.readFile);
        const parsedConfig = ts.parseJsonConfigFileContent(config.config, ts.sys, path.dirname(configPath));
        parsedConfig.options["strictNullChecks"] = true;
        this.program = ts.createProgram({
            rootNames: parsedConfig.fileNames,
            options: parsedConfig.options,
        });
        this.checker = this.program.getTypeChecker();
        this.componentNames = componentNames;
        this.programFiles = new Set(this.program.getSourceFiles().map((f) => f.fileName));
    }

    public analyze(): AnalyzeResult {
        // Find the entry point file
        const entryPoint = this.findProgramEntryPoint();
        // Track remaining files we need to process
        const filesToProcess: typescript.SourceFile[] = [entryPoint];
        // Track which files we've already processed
        const processedFiles = new Set<string>();
        // Keep track of remaining component names we're looking for
        const componentNames = new Set(this.componentNames);

        // Process files until we've found all components or run out of files
        while (filesToProcess.length > 0 && componentNames.size > 0) {
            const sourceFile = filesToProcess.shift()!;

            // Skip if already processed
            if (processedFiles.has(sourceFile.fileName)) {
                continue;
            }
            processedFiles.add(sourceFile.fileName);

            // Look for component declarations in this file
            sourceFile.forEachChild((node) => {
                if (ts.isClassDeclaration(node) && node.name && componentNames.has(node.name.text)) {
                    if (this.isPulumiComponent(node)) {
                        const component = this.analyzeComponent(node);
                        if (component) {
                            this.components[component.name] = component;
                            componentNames.delete(component.name);
                        }
                    }
                } else if ((ts.isClassDeclaration(node) || ts.isInterfaceDeclaration(node)) && node.name) {
                    // Collect documentation for types
                    const dNode = node as docNode;
                    if (dNode?.jsDoc && dNode.jsDoc.length > 0) {
                        const typeDocString = dNode.jsDoc.map((doc: typescript.JSDoc) => doc.comment).join("\n");
                        if (typeDocString) {
                            this.docStrings[node.name.text] = typeDocString;
                        }
                    }
                }
            });

            // If we still have components to find, follow imports from this file
            if (componentNames.size > 0) {
                filesToProcess.push(...this.collectImportedFiles(sourceFile));
            }
        }

        // Check if all components were found
        if (componentNames.size > 0) {
            throw new Error(`Failed to find the following components: ${Array.from(componentNames).join(", ")}. 
Please ensure these components are properly imported to your package's entry point.`);
        }

        return {
            components: this.components,
            typeDefinitions: this.typeDefinitions,
            packageReferences: this.packageReferences,
        };
    }

    private findProgramEntryPoint(): typescript.SourceFile {
        // Helper to convert JS paths to TS paths and resolve them
        const tryResolveSourceFile = (jsPath: string): typescript.SourceFile | undefined => {
            let tsPath = jsPath.replace(/\.js$/, ".ts");
            if (!path.isAbsolute(tsPath)) {
                tsPath = path.join(this.dir, tsPath);
            }
            const sourceFile = this.program.getSourceFile(tsPath);
            if (sourceFile) {
                return sourceFile;
            }
            return undefined;
        };

        // 1. Check package.json for exports field
        if (this.packageJSON.exports) {
            let entryPath: string | undefined;

            if (typeof this.packageJSON.exports === "string") {
                entryPath = this.packageJSON.exports;
            } else if (typeof this.packageJSON.exports === "object") {
                const mainExport = this.packageJSON.exports["."];
                if (typeof mainExport === "string") {
                    entryPath = mainExport;
                } else if (typeof mainExport === "object") {
                    entryPath = mainExport.default || mainExport.require || mainExport.import;
                }
            }

            if (entryPath) {
                const sourceFile = tryResolveSourceFile(entryPath);
                if (sourceFile) {
                    return sourceFile;
                }
            }
        }

        // 2. Check package.json for main field
        if (this.packageJSON.main) {
            const sourceFile = tryResolveSourceFile(this.packageJSON.main);
            if (sourceFile) {
                return sourceFile;
            }
        }

        // 3. Default to index.ts in root or src directory
        const defaultPaths = ["index.ts", "src/index.ts"];
        for (const relativePath of defaultPaths) {
            const fullPath = path.join(this.dir, relativePath);
            const sourceFile = this.program.getSourceFile(fullPath);
            if (sourceFile) {
                return sourceFile;
            }
        }

        throw new Error(
            `No entry points found in ${this.dir}. Expected either 'exports' or 'main' in package.json, or an index.ts file in root or src directory.`,
        );
    }

    private collectImportedFiles(sourceFile: typescript.SourceFile): typescript.SourceFile[] {
        const importedFiles: typescript.SourceFile[] = [];

        // Find all import declarations
        sourceFile.forEachChild((node) => {
            if (!ts.isImportDeclaration(node)) {
                return;
            }

            // Get the module specifier (the string in the import)
            const moduleSpecifier = node.moduleSpecifier;
            if (!ts.isStringLiteral(moduleSpecifier)) {
                return;
            }
            const importPath = moduleSpecifier.text;

            // Resolve the import path relative to the current file
            const resolvedModule = ts.resolveModuleName(
                importPath,
                sourceFile.fileName,
                this.program.getCompilerOptions(),
                ts.sys,
            );
            if (!resolvedModule.resolvedModule) {
                return;
            }

            // Find the source file for this import
            const resolvedFileName = resolvedModule.resolvedModule.resolvedFileName;
            const importedFile = this.program.getSourceFile(resolvedFileName);
            if (importedFile && this.programFiles.has(importedFile.fileName)) {
                importedFiles.push(importedFile);
            }
        });

        return importedFiles;
    }

    private analyzeComponent(node: typescript.ClassDeclaration): ComponentDefinition {
        const componentName = node.name?.text!;

        // We expect exactly 1 constructor, and it must have and 'args'
        // parameter that has an interface type.
        const constructors = node.members.filter((member: typescript.ClassElement) =>
            ts.isConstructorDeclaration(member),
        ) as typescript.ConstructorDeclaration[];
        if (constructors.length !== 1) {
            throw new Error(`Component '${componentName}' must have exactly one constructor`);
        }
        const argsParam = constructors?.[0].parameters.find((param: typescript.ParameterDeclaration) => {
            return ts.isIdentifier(param.name) && param.name.escapedText === "args";
        });
        if (!argsParam) {
            throw new Error(`Component '${componentName}' constructor must have an 'args' parameter`);
        }
        if (!argsParam.type) {
            throw new Error(`Component '${componentName}' constructor 'args' parameter must have a type`);
        }
        const args = this.checker.getTypeAtLocation(argsParam.type);
        const argsSymbol = args.getSymbol();
        if (!argsSymbol || !isInterface(argsSymbol)) {
            throw new Error(`Component '${componentName}' constructor 'args' parameter must be an interface`);
        }

        let inputs: Record<string, PropertyDefinition> = {};
        if (argsSymbol.members) {
            inputs = this.analyzeSymbols(
                { component: componentName, inputOutput: InputOutput.Input, typeName: argsSymbol.getName() },
                symbolTableToSymbols(argsSymbol.members),
                argsParam,
            );
        }

        let outputs: Record<string, PropertyDefinition> = {};
        const classType = this.checker.getTypeAtLocation(node);
        const classSymbol = classType.getSymbol();
        if (classSymbol?.members) {
            outputs = this.analyzeSymbols(
                { component: componentName, inputOutput: InputOutput.Output },
                symbolTableToSymbols(classSymbol.members),
                node,
            );
        }

        const definition: ComponentDefinition = {
            name: componentName!,
            inputs: inputs,
            outputs: outputs,
        };

        const dNode = node as docNode;
        if (dNode.jsDoc && dNode.jsDoc.length > 0) {
            definition.description = dNode.jsDoc.map((doc: typescript.JSDoc) => doc.comment).join("\n");
        }

        return definition;
    }

    private isPulumiComponent(node: typescript.ClassDeclaration): boolean {
        if (!node.heritageClauses) {
            return false;
        }

        return node.heritageClauses.some((clause) => {
            return clause.types.some((clauseNode) => {
                const type = this.checker.getTypeAtLocation(clauseNode);
                const symbol = type.getSymbol();
                const matchesName = symbol?.escapedName === "ComponentResource";
                const sourceFile = symbol?.declarations?.[0].getSourceFile();
                const matchesSourceFile =
                    sourceFile?.fileName.endsWith("resource.ts") || sourceFile?.fileName.endsWith("resource.d.ts");
                return matchesName && matchesSourceFile;
            });
        });
    }

    private analyzeSymbols(
        context: { component: string; inputOutput: InputOutput; typeName?: string },
        symbols: typescript.Symbol[],
        location: typescript.Node,
    ): Record<string, PropertyDefinition> {
        const properties: Record<string, PropertyDefinition> = {};
        symbols.forEach((member) => {
            if (!isPropertyDeclaration(member)) {
                return;
            }
            const name = member.escapedName as string;
            properties[name] = this.analyzeSymbol({ ...context, property: name }, member, location);
        });
        return properties;
    }

    private analyzeSymbol(
        context: { component: string; property: string; inputOutput: InputOutput; typeName?: string },
        symbol: typescript.Symbol,
        location: typescript.Node,
    ): PropertyDefinition {
        // Check if the property is optional, e.g.: myProp?: string; This is
        // defined on the symbol, not the type.
        const propType = this.checker.getTypeOfSymbolAtLocation(symbol, location);
        const optional = isOptional(symbol);
        const dNode = symbol.valueDeclaration as docNode;
        let docString: string | undefined = undefined;
        if (dNode?.jsDoc && dNode.jsDoc.length > 0) {
            docString = dNode.jsDoc.map((doc: typescript.JSDoc) => doc.comment).join("\n");
        }
        return this.analyzeType(
            { ...context, inputOutput: InputOutput.Neither },
            propType,
            location,
            optional,
            docString,
        );
    }

    private analyzeType(
        context: { component: string; property: string; inputOutput: InputOutput; typeName?: string },
        type: typescript.Type,
        location: typescript.Node,
        optional: boolean = false,
        docString: string | undefined = undefined,
    ): PropertyDefinition {
        if (isSimpleType(type)) {
            const prop: PropertyDefinition = { type: tsTypeToPropertyType(type) };
            if (optional) {
                prop.optional = true;
            }
            if (context.inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }
            if (docString) {
                prop.description = docString;
            }
            return prop;
        } else if (isInput(type)) {
            // Grab the promise type from the `T | Promise<T> | OutputInstance<T>`
            // union, and get the type reference `T` from there. With that we
            // can recursively analyze the type, passing through the optional
            // flag. The type can now not be plain anymore, since it's in an
            // input.
            const base = (type as typescript.UnionType)?.types?.find(isPromise);
            if (!base) {
                // unreachable due to the isInput check
                throw new Error(
                    `Input type union must include a Promise: ${this.formatErrorContext(context)} has type '${this.checker.typeToString(type)}'`,
                );
            }
            const innerType = this.unwrapTypeReference(context, base);
            return this.analyzeType(
                { ...context, inputOutput: InputOutput.Input },
                innerType,
                location,
                optional,
                docString,
            );
        } else if (isOutput(type)) {
            type = unwrapOutputIntersection(type);
            // Grab the inner type of the OutputInstance<T> type, and then
            // recurse, passing through the optional flag. The type can now not
            // be plain anymore, since it's wrapped in an output.
            const innerType = this.unwrapTypeReference(context, type);
            return this.analyzeType(
                { ...context, inputOutput: InputOutput.Output },
                innerType,
                location,
                optional,
                docString,
            );
        } else if (isAny(type)) {
            const $ref = "pulumi.json#/Any";
            const prop: PropertyDefinition = { $ref };
            if (optional) {
                prop.optional = true;
            }
            if (docString) {
                prop.description = docString;
            }
            return prop;
        } else if (isAsset(type)) {
            const $ref = "pulumi.json#/Asset";
            const prop: PropertyDefinition = { $ref };
            if (optional) {
                prop.optional = true;
            }
            if (context.inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }
            if (docString) {
                prop.description = docString;
            }
            return prop;
        } else if (isArchive(type)) {
            const $ref = "pulumi.json#/Archive";
            const prop: PropertyDefinition = { $ref };
            if (optional) {
                prop.optional = true;
            }
            if (context.inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }
            if (docString) {
                prop.description = docString;
            }
            return prop;
        } else if (isResourceReference(type, this.checker)) {
            const { packageName, packageVersion, pulumiType } = this.getResourceType(context, type);
            const $ref = `/${packageName}/v${packageVersion}/schema.json#/resources/${pulumiType.replace("/", "%2F")}`;
            this.packageReferences[packageName] = packageVersion;

            const prop: PropertyDefinition = { $ref };
            if (optional) {
                prop.optional = true;
            }
            if (context.inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }
            if (docString) {
                prop.description = docString;
            }
            return prop;
        } else if (type.isClassOrInterface()) {
            // This is a complex type, create a typedef and then reference it in
            // the PropertyDefinition.
            const name = type.getSymbol()?.escapedName as string | undefined;
            if (!name) {
                throw new Error(
                    `Class or interface has no name: ${this.formatErrorContext(context)} has type '${this.checker.typeToString(type)}'`,
                );
            }
            if (this.typeDefinitions[name]) {
                // Type already exists, just reference it and we're done.
                const refProp: PropertyDefinition = { $ref: `#/types/${this.providerName}:index:${name}` };
                if (optional) {
                    refProp.optional = true;
                }
                if (context.inputOutput === InputOutput.Neither) {
                    refProp.plain = true;
                }
                return refProp;
            }
            // Immediately add an empty type definition, so that it can be
            // referenced recursively, then analyze the properties.
            this.typeDefinitions[name] = { name, properties: {} };
            if (this.docStrings[name]) {
                this.typeDefinitions[name].description = this.docStrings[name];
            }
            const typeContext = { ...context, typeName: name };
            const properties = this.analyzeSymbols(typeContext, type.getProperties(), location);
            this.typeDefinitions[name].properties = properties;
            const $ref = `#/types/${this.providerName}:index:${name}`;
            const prop: PropertyDefinition = { $ref };
            if (optional) {
                prop.optional = true;
            }
            if (context.inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }
            if (docString) {
                prop.description = docString;
            }
            return prop;
        } else if (isArrayType(type)) {
            const prop: PropertyDefinition = { type: "array" };
            if (optional) {
                prop.optional = true;
            }
            if (context.inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }

            const typeArguments = (type as typescript.TypeReference).typeArguments;
            if (!typeArguments || typeArguments.length !== 1) {
                throw new Error(
                    `Expected exactly one type argument in '${this.checker.typeToString(type)}': ${this.formatErrorContext(context)} has ${typeArguments?.length || 0} type arguments`,
                );
            }

            const innerType = typeArguments[0];
            prop.items = this.analyzeType(
                {
                    ...context,
                    property: `${context.property}[]`,
                    inputOutput: context.inputOutput === InputOutput.Output ? InputOutput.Output : InputOutput.Neither,
                },
                innerType,
                location,
                false /* optional */,
            );
            if (docString) {
                prop.description = docString;
            }
            return prop;
        } else if (isMapType(type, this.checker)) {
            const prop: PropertyDefinition = { type: "object" };
            if (optional) {
                prop.optional = true;
            }
            if (context.inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }

            // We got { [key: string]: <indexInfo.type> }
            const indexInfo = this.checker.getIndexInfoOfType(type, ts.IndexKind.String);
            if (!indexInfo) {
                // We can't actually get here because isMapType checks for indexInfo
                throw new Error(`Map type has no index info`);
            }
            if (docString) {
                prop.description = docString;
            }
            prop.additionalProperties = this.analyzeType(
                {
                    ...context,
                    property: `${context.property} values`,
                },
                indexInfo.type,
                location,
                false,
            );
            return prop;
        } else if (isOptionalType(type, this.checker)) {
            const unionType = type as typescript.UnionType;
            const nonUndefinedType = unionType.types.find((t) => !(t.flags & ts.TypeFlags.Undefined));
            if (!nonUndefinedType) {
                throw new Error(
                    `Expected exactly one type to not be undefined: ${this.formatErrorContext(context)} has type '${this.checker.typeToString(type)}'`,
                );
            }
            return this.analyzeType(context, nonUndefinedType, location, true, docString);
        } else if (type.isUnion()) {
            throw new Error(
                `Union types are not supported for ${this.formatErrorContext(context)}: type '${this.checker.typeToString(type)}'`,
            );
        } else if (type.isIntersection()) {
            throw new Error(
                `Intersection types are not supported for ${this.formatErrorContext(context)}: type '${this.checker.typeToString(type)}'`,
            );
        }

        throw new Error(
            `Unsupported type for ${this.formatErrorContext(context)}: type '${this.checker.typeToString(type)}'`,
        );
    }

    unwrapTypeReference(
        context: { component: string; property: string; inputOutput: InputOutput; typeName?: string },
        type: typescript.Type,
    ): typescript.Type {
        let typeArguments = (type as typescript.TypeReference).typeArguments;
        if (!typeArguments) {
            typeArguments = (type as typescript.TypeReference).aliasTypeArguments;
        }
        if (!typeArguments || typeArguments.length !== 1) {
            throw new Error(
                `Expected exactly one type argument in '${this.checker.typeToString(type)}': ${this.formatErrorContext(context)} has ${typeArguments?.length || 0} type arguments`,
            );
        }
        const innerType = typeArguments[0];
        return innerType;
    }

    private formatErrorContext(context: {
        component: string;
        property?: string;
        inputOutput?: InputOutput;
        typeName?: string;
    }): string {
        const parts: string[] = [];
        parts.push(`component '${context.component}'`);

        if (context.property) {
            let propType = "property";
            if (context.inputOutput !== undefined) {
                if (context.inputOutput === InputOutput.Input) {
                    propType = "input";
                } else if (context.inputOutput === InputOutput.Output) {
                    propType = "output";
                }
            }
            parts.push(propType);

            let propName = context.property;
            if (context.typeName) {
                propName = `${context.typeName}.${propName}`;
            }
            parts.push(`'${propName}'`);
        }

        return parts.join(" ");
    }

    /**
     * Gets the Pulumi resource type information for a resource reference.
     * A strong assumption is that the referenced resource class is in a package installed to node_modules
     * and contains a standard Pulumi-generated SDK compiled into JavaScript. To find the resource type token,
     * the function will attempt to find the JavaScript module file that contains the resource class, and then
     * extract the type from the __pulumiType property of the resource class. To find the package version,
     * the function will attempt to read the package.json file in the root directory of the referenced package.
     * @returns Object containing packageName, packageVersion, and pulumiType token
     * @throws Error if the resource type cannot be determined with detailed context information
     */
    private getResourceType(
        context: { component: string; property: string; inputOutput: InputOutput; typeName?: string },
        type: typescript.Type,
    ): {
        packageName: string;
        packageVersion: string;
        pulumiType: string;
    } {
        const symbol = type.getSymbol();
        if (!symbol) {
            throw new Error(
                `Cannot determine resource type: source (symbol) not found for type '${this.checker.typeToString(type)}' for ${this.formatErrorContext(context)}`,
            );
        }

        // Try to find the declaration of the class
        const declaration = symbol.declarations?.[0];
        if (!declaration) {
            throw new Error(
                `Cannot determine resource type: source (declaration) not found for symbol '${symbol.name}' for ${this.formatErrorContext(context)}`,
            );
        }

        // Find its declaration source file.
        const sourceFile = declaration.getSourceFile();
        if (!sourceFile) {
            throw new Error(
                `Cannot determine resource type: source file not found for declaration of '${symbol.name}' for ${this.formatErrorContext(context)}`,
            );
        }

        // Find the actual implementation file - use the TypeScript file directly if it's not a .d.ts file
        let implPath = sourceFile.fileName;
        if (implPath.endsWith(".d.ts")) {
            // For declaration files, look for the corresponding .js file
            implPath = implPath.replace(/\.d\.ts$/, ".js");
        }

        if (!ts.sys.fileExists(implPath)) {
            throw new Error(
                `Cannot determine resource type: source file not found at '${implPath}' for '${symbol.name}' for ${this.formatErrorContext(context)}`,
            );
        }

        // Load the module.
        const module = require(implPath);
        if (!module) {
            throw new Error(`Failed to load module from '${implPath}' for ${this.formatErrorContext(context)}`);
        }

        // Find the resource class.
        const resourceClass = module[symbol.name];
        if (!resourceClass) {
            throw new Error(
                `Resource class '${symbol.name}' not found in module '${implPath}' for ${this.formatErrorContext(context)}`,
            );
        }

        // Find the __pulumiType property.
        const pulumiType = resourceClass.__pulumiType;
        if (!pulumiType) {
            throw new Error(
                `Could not determine __pulumiType for resource class '${symbol.name}' in '${implPath}' for ${this.formatErrorContext(context)}`,
            );
        }

        // Extract the package name and pulumi type from the __pulumiType property.
        const packageName = pulumiType.split(":")[0];

        // Extract package name from the path.
        const packageMatch = implPath.match(/node_modules\/((@[^/]+\/)?[^/]+)/);
        if (!packageMatch || packageMatch.length < 2) {
            throw new Error(
                `Cannot determine resource type: package name not found for '${symbol.name}' for ${this.formatErrorContext(context)}`,
            );
        }

        const npmPackageName = packageMatch[1];
        // We only support @pulumi/foo packages for resource references for now, so that we know exactly how to build the list
        // of dependencies based on a package name.
        if (!npmPackageName.startsWith("@pulumi/")) {
            throw new Error(
                `Cannot determine resource type: only @pulumi packages are supported for resource references '${symbol.name}' for ${this.formatErrorContext(context)}. Found ${npmPackageName}`,
            );
        }

        // Find package.json to get the version
        const packageJsonPath = path.resolve(
            implPath.substring(0, implPath.indexOf(npmPackageName) + npmPackageName.length),
            "package.json",
        );

        if (!ts.sys.fileExists(packageJsonPath)) {
            throw new Error(
                `Cannot determine resource type: package.json not found for '${symbol.name}' for ${this.formatErrorContext(context)}`,
            );
        }

        // Read the package version from the package.json file.
        const packageJson = JSON.parse(ts.sys.readFile(packageJsonPath)!);
        let packageVersion = packageJson.version;
        if (packageVersion.startsWith("v")) {
            packageVersion = packageVersion.slice(1);
        }

        return {
            packageName,
            pulumiType,
            packageVersion,
        };
    }
}

function isOptional(symbol: typescript.Symbol): boolean {
    return (symbol.flags & ts.SymbolFlags.Optional) === ts.SymbolFlags.Optional;
}

function isOptionalType(type: typescript.Type, checker: typescript.TypeChecker): boolean {
    if (!(type.flags & ts.TypeFlags.Union)) {
        return false;
    }

    const unionType = type as typescript.UnionType;
    // We only support union types with two types, one of which must be undefined
    if (!unionType.types || unionType.types.length !== 2) {
        return false;
    }

    // Check if one of the types in the union is undefined
    return unionType.types.some(
        (t) => t.flags & ts.TypeFlags.Undefined || t.flags & ts.TypeFlags.Void, // Also check for void in some cases
    );
}

function isInterface(symbol: typescript.Symbol): boolean {
    return (symbol.flags & ts.SymbolFlags.Interface) === ts.SymbolFlags.Interface;
}

function isPropertyDeclaration(symbol: typescript.Symbol): boolean {
    return (symbol.flags & ts.SymbolFlags.Property) === ts.SymbolFlags.Property;
}

function isNumber(type: typescript.Type): boolean {
    return (type.flags & ts.TypeFlags.Number) === ts.TypeFlags.Number;
}

function isString(type: typescript.Type): boolean {
    return (type.flags & ts.TypeFlags.String) === ts.TypeFlags.String;
}

function isBoolean(type: typescript.Type): boolean {
    return (type.flags & ts.TypeFlags.Boolean) === ts.TypeFlags.Boolean;
}

function isAny(type: typescript.Type): boolean {
    return (type.flags & ts.TypeFlags.Any) === ts.TypeFlags.Any;
}

function isSimpleType(type: typescript.Type): boolean {
    return isNumber(type) || isString(type) || isBoolean(type);
}

function isMapType(type: typescript.Type, checker: typescript.TypeChecker): boolean {
    const indexInfo = checker.getIndexInfoOfType(type, ts.IndexKind.String);
    return indexInfo !== undefined;
}

function isArrayType(type: typescript.Type): boolean {
    return (type.flags & ts.TypeFlags.Object) === ts.TypeFlags.Object && type.getSymbol()?.escapedName === "Array";
}

function isPromise(type: typescript.Type): boolean {
    if (!(type.flags & ts.TypeFlags.Object)) {
        return false;
    }
    const symbol = (type as typescript.ObjectType).symbol;
    if (!symbol) {
        return false;
    }
    return symbol.name === "Promise";
}

function isOutput(type: typescript.Type): boolean {
    // In sdk/nodejs/output.ts we define Output as:
    //
    //   export type Output<T> = OutputInstance<T> & Lifted<T>;
    //
    // Depending on T, we might have an OutputInstance<T> because Lifted<T>
    // does not add anything to the resulting type, or we get the
    // intersection. In the latter case, we want to find the
    // OutputInstance<T> within the intersection.
    if (type.isIntersection()) {
        for (const t of type.types) {
            if (isOutput(t)) {
                return true;
            }
        }
    }
    let symbol = type.getSymbol();
    if (!symbol) {
        symbol = type.aliasSymbol;
    }
    const matchesName = symbol?.escapedName === "OutputInstance" || symbol?.escapedName === "Output";
    const sourceFile = symbol?.declarations?.[0].getSourceFile();
    const matchesSourceFile =
        sourceFile?.fileName.endsWith("output.ts") || sourceFile?.fileName.endsWith("output.d.ts");
    return !!matchesName && !!matchesSourceFile;
}

function isAsset(type: typescript.Type): boolean {
    const symbol = type.getSymbol();
    const matchesName = symbol?.escapedName === "Asset";
    const sourceFile = symbol?.declarations?.[0].getSourceFile();
    const matchesSourceFile = sourceFile?.fileName.endsWith("asset.ts") || sourceFile?.fileName.endsWith("asset.d.ts");
    return !!matchesName && !!matchesSourceFile;
}

function isArchive(type: typescript.Type): boolean {
    const symbol = type.getSymbol();
    const matchesName = symbol?.escapedName === "Archive";
    const sourceFile = symbol?.declarations?.[0].getSourceFile();
    const matchesSourceFile =
        sourceFile?.fileName.endsWith("archive.ts") || sourceFile?.fileName.endsWith("archive.d.ts");
    return !!matchesName && !!matchesSourceFile;
}

function unwrapOutputIntersection(type: typescript.Type): typescript.Type {
    // Output<T> is an intersection type `OutputInstance<T> & Lifted<T>`, and
    // we want to find the `OutputInstance<T>` within the intersection for
    // further analysis.
    // Depending on `T`, TypeScript sometimes infers Output<T> directly as
    // `OutputInstance<T>`, dropping the `Lifted<T>` part.
    if (type.isIntersection()) {
        for (const t of type.types) {
            if (isOutput(t)) {
                return t;
            }
        }
    }
    return type;
}

/**
 * An input type is a union of Output<T>, Promise<T>, and T.
 */
function isInput(type: typescript.Type): boolean {
    if (!type.isUnion()) {
        return false;
    }
    let hasOutput = false;
    let hasPromise = false;
    let hasOther = false;
    for (const t of type.types) {
        if (isOutput(t)) {
            hasOutput = true;
        } else if (isPromise(t)) {
            hasPromise = true;
        } else {
            hasOther = true;
        }
    }
    return hasOutput && hasPromise && hasOther;
}

function isResourceReference(type: typescript.Type, checker: typescript.TypeChecker): boolean {
    if (!type.isClass()) {
        return false;
    }
    return checker.getBaseTypes(type as typescript.InterfaceType).some((baseType) => {
        const symbol = baseType.getSymbol();
        const matchesName =
            symbol?.escapedName === "CustomResource" ||
            symbol?.escapedName === "ComponentResource" ||
            symbol?.escapedName === "Resource";

        const sourceFile = symbol?.declarations?.[0].getSourceFile();
        const matchesSourceFile =
            sourceFile?.fileName.endsWith("resource.ts") || sourceFile?.fileName.endsWith("resource.d.ts");
        return (matchesName && matchesSourceFile) || isResourceReference(baseType, checker);
    });
}

function tsTypeToPropertyType(type: typescript.Type): PropertyType {
    if (isNumber(type)) {
        return "number";
    } else if (isString(type)) {
        return "string";
    } else if (isBoolean(type)) {
        return "boolean";
    }

    throw new Error(`Unsupported type '${type.symbol?.name}'`);
}

function symbolTableToSymbols(table: typescript.SymbolTable): typescript.Symbol[] {
    const symbols: typescript.Symbol[] = [];
    table.forEach((symbol) => {
        symbols.push(symbol);
    });
    return symbols;
}
