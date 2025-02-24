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
import { ComponentResource } from "../../resource";

// Use the TypeScript shim which allows us to fallback to a vendored version of
// TypeScript if the user has not installed it.
// TODO: we should consider requiring the user to install TypeScript and not
// rely on the shim. In any case, we should add tests for providers with
// different versions of TypeScript in their dependencies, to ensure the
// analyzer code is compatible with all of them.
const ts: typeof typescript = require("../../typescript-shim");

export type PropertyType = "string" | "integer" | "number" | "boolean" | "array" | "object";

export type PropertyDefinition = ({ type: PropertyType } | { $ref: string }) & {
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
};

export type AnalyzeResult = {
    components: Record<string, ComponentDefinition>;
    typeDefinitions: Record<string, TypeDefinition>;
};

interface docNode extends typescript.Node {
    jsDoc?: typescript.JSDoc[];
}

enum InputOutput {
    Neither = 0,
    Input = 1,
    Output = 2,
}

export class Analyzer {
    private path: string;
    private providerName: string;
    private checker: typescript.TypeChecker;
    private program: typescript.Program;
    private components: Record<string, ComponentDefinition> = {};
    private typeDefinitions: Record<string, TypeDefinition> = {};

    constructor(dir: string, providerName: string) {
        const configPath = `${dir}/tsconfig.json`;
        const config = ts.readConfigFile(configPath, ts.sys.readFile);
        const parsedConfig = ts.parseJsonConfigFileContent(config.config, ts.sys, path.dirname(configPath));
        this.path = dir;
        this.providerName = providerName;
        const options = parsedConfig.options;
        parsedConfig.options["strictNullChecks"] = true;
        this.program = ts.createProgram({
            rootNames: parsedConfig.fileNames,
            options: parsedConfig.options,
        });
        this.checker = this.program.getTypeChecker();
    }

    public analyze(): AnalyzeResult {
        const sourceFiles = this.program.getSourceFiles();
        for (const sourceFile of sourceFiles) {
            if (sourceFile.fileName.includes("node_modules") || sourceFile.fileName.endsWith(".d.ts")) {
                continue;
            }
            this.analyseFile(sourceFile);
        }
        return {
            components: this.components,
            typeDefinitions: this.typeDefinitions,
        };
    }

    public async findComponent(name: string): Promise<typeof ComponentResource> {
        const sourceFiles = this.program.getSourceFiles();
        for (const sourceFile of sourceFiles) {
            if (sourceFile.fileName.includes("node_modules") || sourceFile.fileName.endsWith(".d.ts")) {
                continue;
            }
            for (const node of sourceFile.statements) {
                if (ts.isClassDeclaration(node) && this.isPulumiComponent(node) && node.name) {
                    if (ts.isClassDeclaration(node) && this.isPulumiComponent(node) && node.name?.text === name) {
                        try {
                            const module = await import(sourceFile.fileName);
                            return module[name];
                        } catch (e) {
                            throw new Error(`Failed to import component '${name}': ${e}`);
                        }
                    }
                }
            }
        }
        throw new Error(`Component '${name}' not found`);
    }

    private analyseFile(sourceFile: typescript.SourceFile) {
        // We intentionally visit only the top-level nodes, because we only
        // support components defined at the top-level. We have no way to
        // instantiate components defined inside functions or methods.
        sourceFile.forEachChild((node) => {
            if (ts.isClassDeclaration(node) && this.isPulumiComponent(node) && node.name) {
                const component = this.analyzeComponent(node);
                this.components[component.name] = component;
            }
        });
    }

    private analyzeComponent(node: typescript.ClassDeclaration): ComponentDefinition {
        const componentName = node.name?.text;

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
            inputs = this.analyzeSymbols(symbolTableToSymbols(argsSymbol.members), argsParam);
        }

        let outputs: Record<string, PropertyDefinition> = {};
        const classType = this.checker.getTypeAtLocation(node);
        const classSymbol = classType.getSymbol();
        if (classSymbol?.members) {
            outputs = this.analyzeSymbols(symbolTableToSymbols(classSymbol.members), node);
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
        symbols: typescript.Symbol[],
        location: typescript.Node,
    ): Record<string, PropertyDefinition> {
        const properties: Record<string, PropertyDefinition> = {};
        symbols.forEach((member) => {
            if (!isPropertyDeclaration(member)) {
                return;
            }
            const name = member.escapedName as string;
            properties[name] = this.analyzeSymbol(member, location);
        });
        return properties;
    }

    private analyzeSymbol(symbol: typescript.Symbol, location: typescript.Node): PropertyDefinition {
        // Check if the property is optional, e.g.: myProp?: string; This is
        // defined on the symbol, not the type.
        const propType = this.checker.getTypeOfSymbolAtLocation(symbol, location);
        const optional = isOptional(symbol);
        return this.analyzeType(propType, location, optional);
    }

    private analyzeType(
        type: typescript.Type,
        location: typescript.Node,
        optional: boolean = false,
        inputOutput: InputOutput = InputOutput.Neither,
    ): PropertyDefinition {
        if (isSimpleType(type)) {
            const prop: PropertyDefinition = { type: tsTypeToPropertyType(type) };
            if (optional) {
                prop.optional = true;
            }
            if (inputOutput === InputOutput.Neither) {
                prop.plain = true;
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
                throw new Error(`Input type union must include a Promise, got '${this.checker.typeToString(type)}'`);
            }
            const innerType = this.unwrapTypeReference(base);
            return this.analyzeType(innerType, location, optional, InputOutput.Input);
        } else if (isOutput(type)) {
            type = unwrapOutputIntersection(type);
            // Grab the inner type of the OutputInstance<T> type, and then
            // recurse, passing through the optional flag. The type can now not
            // be plain anymore, since it's wrapped in an output.
            const innerType = this.unwrapTypeReference(type);
            return this.analyzeType(innerType, location, optional, InputOutput.Output);
        } else if (isAsset(type)) {
            const $ref = "pulumi.json#/Asset";
            const prop: PropertyDefinition = { $ref };
            if (optional) {
                prop.optional = true;
            }
            if (inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }
            return prop;
        } else if (isArchive(type)) {
            const $ref = "pulumi.json#/Archive";
            const prop: PropertyDefinition = { $ref };
            if (optional) {
                prop.optional = true;
            }
            if (inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }
            return prop;
        } else if (type.isClassOrInterface()) {
            // This is a complex type, create a typedef and then reference it in
            // the PropertyDefinition.
            const name = type.getSymbol()?.escapedName as string | undefined;
            if (!name) {
                throw new Error(`Class or interface '${this.checker.typeToString(type)}}' has no name`);
            }
            if (this.typeDefinitions[name]) {
                // Type already exists, just reference it and we're done.
                const refProp: PropertyDefinition = { $ref: `#/types/${this.providerName}:index:${name}` };
                if (optional) {
                    refProp.optional = true;
                }
                if (inputOutput === InputOutput.Neither) {
                    refProp.plain = true;
                }
                return refProp;
            }
            // Immediately add an empty type definition, so that it can be
            // referenced recursively, then analyze the properties.
            this.typeDefinitions[name] = { name, properties: {} };
            const properties = this.analyzeSymbols(type.getProperties(), location);
            this.typeDefinitions[name].properties = properties;
            const $ref = `#/types/${this.providerName}:index:${name}`;
            const prop: PropertyDefinition = { $ref };
            if (optional) {
                prop.optional = true;
            }
            if (inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }
            return prop;
        } else if (isArrayType(type)) {
            const prop: PropertyDefinition = { type: "array" };
            if (optional) {
                prop.optional = true;
            }
            if (inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }

            const typeArguments = (type as typescript.TypeReference).typeArguments;
            if (!typeArguments || typeArguments.length !== 1) {
                throw new Error(
                    `Expected exactly one type argument in '${this.checker.typeToString(type)}', got '${typeArguments?.length}'`,
                );
            }

            const innerType = typeArguments[0];
            prop.items = this.analyzeType(innerType, location, false /* optional */, InputOutput.Neither);
            return prop;
        } else if (isMapType(type, this.checker)) {
            const prop: PropertyDefinition = { type: "object" };
            if (optional) {
                prop.optional = true;
            }
            if (inputOutput === InputOutput.Neither) {
                prop.plain = true;
            }

            // We got { [key: string]: <indexInfo.type> }
            const indexInfo = this.checker.getIndexInfoOfType(type, ts.IndexKind.String);
            if (!indexInfo) {
                // We can't actually get here because isMapType checks for indexInfo
                throw new Error(`Map type has no index info`);
            }
            prop.additionalProperties = this.analyzeType(indexInfo.type, location, false /* optional */, inputOutput);
            return prop;
        } else if (isOptionalType(type, this.checker)) {
            const unionType = type as typescript.UnionType;
            const nonUndefinedType = unionType.types.find((t) => !(t.flags & ts.TypeFlags.Undefined));
            if (!nonUndefinedType) {
                throw new Error(
                    `Expected exactly one type to not be undefined in '${this.checker.typeToString(type)}'`,
                );
            }
            return this.analyzeType(nonUndefinedType, location, true, inputOutput);
        } else if (type.isUnion()) {
            throw new Error(`Union types are not supported, got '${this.checker.typeToString(type)}'`);
        } else if (type.isIntersection()) {
            throw new Error(`Intersection types are not supported, got '${this.checker.typeToString(type)}'`);
        }

        throw new Error(`Unsupported type '${this.checker.typeToString(type)}'`);
    }

    unwrapTypeReference(type: typescript.Type): typescript.Type {
        let typeArguments = (type as typescript.TypeReference).typeArguments;
        if (!typeArguments) {
            typeArguments = (type as typescript.TypeReference).aliasTypeArguments;
        }
        if (!typeArguments || typeArguments.length !== 1) {
            throw new Error(
                `Expected exactly one type argument in '${this.checker.typeToString(type)}', got '${typeArguments?.length}'`,
            );
        }
        const innerType = typeArguments[0];
        return innerType;
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
