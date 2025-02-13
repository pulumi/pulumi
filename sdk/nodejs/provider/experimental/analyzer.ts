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

export type ComponentDefinition = {
    name: string;
};

export type TypeDefinition = {
    name: string;
};

export type AnalyzeResult = {
    components: Record<string, ComponentDefinition>;
    typeDefinitons: Record<string, TypeDefinition>;
};

export class Analyzer {
    private path: string;
    private checker: typescript.TypeChecker;
    private program: typescript.Program;
    private components: Record<string, ComponentDefinition> = {};
    private typeDefinitons: Record<string, TypeDefinition> = {};

    constructor(dir: string) {
        const configPath = `${dir}/tsconfig.json`;
        const config = ts.readConfigFile(configPath, ts.sys.readFile);
        const parsedConfig = ts.parseJsonConfigFileContent(config.config, ts.sys, path.dirname(configPath));
        this.path = dir;
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
            typeDefinitons: this.typeDefinitons,
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
                const componentName = node.name.text;
                this.components[componentName] = {
                    name: componentName,
                };
            }
        });
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
}
