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
import yargs from "yargs";
import { hideBin } from "yargs/helpers";

import {
    CodeBlockWriter,
    ParameterDeclarationStructure,
    Project,
    PropertySignatureStructure,
    ReturnStatement,
    SourceFile,
    StructureKind,
    VariableDeclarationKind,
} from "ts-morph";

import type { Argument, Arguments, Flag, Structure } from "./types";

// The words we consider reserved in TypeScript.
const reservedWords: string[] = ["console", "import", "new", "package", "type"];

// CLI interface options.
interface Options {
    _: string[];
    boilerplate: string;
    output: string;
    result: string;
}

(function main(): void {
    const argv: Options = yargs(hideBin(process.argv))
        .usage("Usage: $0 <path-to-specification.json> [options]")
        .option("boilerplate", {
            alias: "b",
            type: "string",
            describe: "Path to the boilerplate TypeScript file.",
            default: path.join(process.cwd(), "src", "boilerplate.ts"),
        })
        .option("output", {
            alias: "o",
            type: "string",
            describe: "Output directory for generated files.",
            default: path.join(process.cwd(), "output"),
        })
        .option("result", {
            alias: "r",
            type: "string",
            describe: "The type of the command results.",
            default: "Promise<Output>",
        })
        .demand(1, "Path to specification JSON is required.")
        .strict()
        .help()
        .parseSync() as Options;

    const [pathToSpecification] = argv._;
    if (typeof pathToSpecification !== "string") {
        throw new Error("Specification path must be a string.");
    }

    const specification: string = path.resolve(process.cwd(), pathToSpecification);
    const boilerplate: string = path.resolve(process.cwd(), argv.boilerplate);
    const output: string = path.resolve(process.cwd(), argv.output);

    const spec: Structure = JSON.parse(fs.readFileSync(specification, "utf-8")) as Structure;
    fs.mkdirSync(output, { recursive: true });

    const index: string = path.join(output, "index.ts");
    const project: Project = new Project({});

    const content: string = fs.readFileSync(boilerplate, "utf-8");
    const source: SourceFile = project.createSourceFile(index, content, { overwrite: true });

    generateOptionsTypes(spec, source);
    generateCommands(spec, source, argv.result);

    project.saveSync();
})();

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
function generateCommands(
    structure: Structure,
    source: SourceFile,
    returnType: string,
    breadcrumbs: string[] = [],
): void {
    if (structure.type === "menu") {
        if (structure.commands) {
            for (const [name, subcommand] of Object.entries(structure.commands)) {
                generateCommands(subcommand, source, returnType, [...breadcrumbs, name]);
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
        returnType,
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

        const push = (reference: string): void => {
            writer.writeLine(`__arguments.push('' + ${reference});`);
        };

        if (optional) {
            writer.writeLine(`if (${name} != null) {`);
            writer.indent(() => {
                if (variadic) {
                    writer.writeLine(`for (const __item of ${name}) {`);
                    writer.indent(() => push("__item"));
                    writer.writeLine("}");
                } else {
                    push(name);
                }
            });
            writer.writeLine("}");
        } else {
            push(name);
        }

        writer.blankLine();
    }

    function pushOption(name: string, flag: Flag): void {
        const { type, repeatable = false } = flag;

        const push = (reference: string): void => {
            // Boolean flags are pushed without value when true, and ignored when false.
            if (type === "boolean") {
                writer.writeLine(`if (${reference}) {`);
                writer.indent(() => {
                    writer.writeLine(`__arguments.push('--${name}');`);
                });
                writer.writeLine("}");
            } else {
                writer.writeLine(`__arguments.push('--${name}', '' + ${reference});`);
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
    writer.writeLine(`return __run('${command} ' + __arguments.join(' '));`);
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
