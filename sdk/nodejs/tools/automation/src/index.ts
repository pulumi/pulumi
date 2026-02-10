// Copyright 2016-2026, Pulumi Corporation.
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
import * as path from "path";
import pascalise from "pascalcase";
import camelCase from "to-camel-case";

import {
    CodeBlockWriter,
    ParameterDeclarationStructure,
    Project,
    PropertySignatureStructure,
    ReturnStatement,
    SourceFile,
    StructureKind,
} from "ts-morph";

import type { Argument, Arguments, Flag, Structure } from "./types";

// The words we consider reserved in TypeScript.
const reservedWords: string[] = ["console", "import", "new", "package", "type"];

(function main(): void {
    if (!process.argv[2]) {
        throw new Error("Usage: npm start <path-to-specification.json>");
    }

    const specification: string = path.resolve(process.cwd(), process.argv[2]);
    const output: string = path.join(process.cwd(), "output");

    const spec: Structure = JSON.parse(fs.readFileSync(specification, "utf-8")) as Structure;
    fs.mkdirSync(output, { recursive: true });

    const index: string = path.join(output, "index.ts");
    const project: Project = new Project({});

    const source: SourceFile = project.createSourceFile(index, "", { overwrite: true });

    generateStaticDeclarations(source);
    generateOptionsTypes(spec, source);
    generateCommands(spec, source);

    project.saveSync();
})();

// Create imports, type declarations, and helper functions for the generated code.
function generateStaticDeclarations(source: SourceFile): void {
    source.addInterface({
        kind: StructureKind.Interface,
        name: "Output",
        docs: ["The output of a command."],
        properties: [
            {
                name: "stdout",
                type: "string",
            },
            {
                name: "stderr",
                type: "string",
            },
            {
                name: "exitCode",
                type: "number",
            },
        ],
        isExported: true,
    });
}

// Every command and menu may add some flags to the pool of available flags. This means that, as we
// descend the command tree, we need to collect all the flags that have been defined and add them to
// an options object.
function generateOptionsTypes(
    structure: Structure,
    source: SourceFile,
    breadcrumbs: string[] = [],
    inherited: Record<string, Flag> = {},
): void {
    const command: string = "pulumi " + breadcrumbs.join(" ");
    const options: Record<string, Flag> = { ...inherited, ...(structure.flags ?? {}) };

    source.addInterface({
        kind: StructureKind.Interface,
        name: createOptionsTypeName(breadcrumbs),
        docs: ["Options for the `" + command + "` command."],
        isExported: true,
        properties: Object.values(options).map(flagToPropertySignature),
    });

    if (structure.type === "menu" && structure.commands) {
        for (const [name, child] of Object.entries(structure.commands)) {
            generateOptionsTypes(child, source, [...breadcrumbs, name]);
        }
    }
}

// Generate the command functions for the command tree.
function generateCommands(structure: Structure, source: SourceFile, breadcrumbs: string[] = []): void {
    if (structure.type === "menu") {
        if (structure.commands) {
            for (const [name, subcommand] of Object.entries(structure.commands)) {
                generateCommands(subcommand, source, [...breadcrumbs, name]);
            }
        }

        if (!structure.executable) {
            return;
        }
    }

    const parameters: ParameterDeclarationStructure[] = [];
    parameters.push({
        kind: StructureKind.Parameter,
        name: "__options",
        type: createOptionsTypeName(breadcrumbs),
    });

    if (structure.type === "command" && structure.arguments) {
        const specification = structure.arguments;

        for (let i = 0; i < specification.arguments.length; i++) {
            const argument: Argument = specification.arguments[i];
            const optional: boolean = i >= (specification.requiredArguments ?? 0);
            const variadic: boolean = i === specification.arguments.length - 1 && (specification.variadic ?? false);

            parameters.push({
                kind: StructureKind.Parameter,
                name: convertName(argument.name),
                type: convertType(argument.type ?? "string", variadic),
                hasQuestionToken: optional && !variadic, // TS doesn't allow optional rest parameters
                isRestParameter: variadic,
            });
        }
    }

    source.addFunction({
        name: convertName(breadcrumbs.join(" ")),
        isExported: true,
        parameters,
        statements: (writer) => generateBody(structure, writer, breadcrumbs),
        returnType: "string",
    });
}

// Generate the body of the command function.
function generateBody(structure: Structure, writer: CodeBlockWriter, breadcrumbs: string[]): void {
    writer.writeLine("const __arguments: string[] = [];");
    writer.blankLine();

    function pushArgument(argument: Argument, index: number, spec: Arguments): void {
        const variadic: boolean = index === spec.arguments.length - 1 && (spec.variadic ?? false);
        const required: number = spec.requiredArguments ?? 0;
        const optional: boolean = index >= required;
        const name: string = convertName(argument.name);

        if (optional) {
            writer.writeLine(`if (${name} != null) {`);
            writer.indent(() => {
                if (variadic) {
                    writer.writeLine(`__arguments.push(...${name});`);
                } else {
                    writer.writeLine(`__arguments.push(${name});`);
                }
            });
            writer.writeLine("}");
        }

        const push = (): void => {
            if (variadic) {
                writer.writeLine(`__arguments.push(...${name});`);
            } else {
                writer.writeLine(`__arguments.push(${name});`);
            }
        };

        if (optional) {
            writer.writeLine(`if (${name} != null) {`);
            writer.indent(push);
            writer.writeLine("}");
        } else {
            push();
        }
        writer.blankLine();
    }

    function pushOption(name: string, flag: Flag): void {
        const { type, repeatable = false } = flag;

        const push = (reference: string): void => {
            // Boolean flags are pushed without value when true, and ignored when false.
            if (type !== "boolean") {
                writer.writeLine(`if (${reference}) {`);
                writer.indent(() => {
                    writer.writeLine(`__arguments.push('--${name}');`);
                });
                writer.writeLine("}");
            } else {
                writer.writeLine(`__arguments.push('--${name}', ${reference});`);
            }
        };

        const converted: string = convertName(name);
        writer.writeLine(`if (__options.${converted} != null) {`);
        writer.indent(() => {
            if (repeatable) {
                writer.writeLine(`for (const __item of __options.${converted}) {`);
                writer.indent(() => push("__item"));
                writer.writeLine("}");
            } else {
                push(`__options.${converted}`);
            }
        });

        writer.writeLine("}");
        writer.blankLine();
    }

    if (structure.type === "command" && structure.arguments) {
        const specification = structure.arguments;
        for (let i = 0; i < specification.arguments.length; i++) {
            pushArgument(specification.arguments[i], i, specification);
        }
    }

    for (const [name, flag] of Object.entries(structure.flags ?? {})) {
        pushOption(name, flag);
    }

    const command: string = ["pulumi", ...breadcrumbs].join(" ");
    writer.writeLine(`return '${command} ' + __arguments.join(' ');`);
}

// Options types are pascal-cased versions of the command breadcrumbs, prefixed
// with "Pulumi" and suffixed with "Options", like "PulumiAboutEnvOptions".
function createOptionsTypeName(breadcrumbs: string[]): string {
    return pascalise(["pulumi", ...breadcrumbs, "options"].join(" "));
}

// Create property signatures for the options object.
function flagToPropertySignature(flag: Flag): PropertySignatureStructure {
    return {
        name: convertName(flag.name),
        kind: StructureKind.PropertySignature,
        type: convertType(flag.type, flag.repeatable ?? false),
        hasQuestionToken: true,
        docs: flag.description ? [flag.description] : undefined,
    };
}

// The type system of flags is effectively just the type system of Go, so we need to find appropriate
// approximations of the types for TypeScript.
function convertType(type: string, repeatable: boolean): string {
    let base: string = "";

    switch (type) {
        case "string":
            base = "string";
            break;

        case "boolean":
            base = "boolean";
            break;

        case "int":
            base = "number";
            break;

        default:
            throw new Error("Unknown type: " + type);
    }

    return repeatable ? base + "[]" : base;
}

// Names are camel-cased, with a double underscore prefix if they're reserved words.
function convertName(name: string): string {
    const prefix: string = reservedWords.includes(name) ? "__" : "";
    return prefix + camelCase(name);
}
