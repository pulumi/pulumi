// Copyright 2026, Pulumi Corporation.
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
    ClassDeclaration,
    ClassDeclarationStructure,
    ParameterDeclarationStructure,
    Project,
    PropertySignatureStructure,
    SourceFile,
    StructureKind,
    WriterFunction,
} from "ts-morph";

import type { Argument, Arguments, Command, Flag, PresetValue, Structure } from "./types";

// Known collisions between the Pulumi CLI and the TypeScript keywords or globals.
const reservedWords: string[] = ["options", "package"];

/**
 * Strip omit/preset fields from a flag so that override information doesn't
 * leak from parent to child via inheritance. Each node re-introduces overrides
 * via its own spec flags.
 */
function baseFlag(flag: Flag): Flag {
    const { omit, preset, ...rest } = flag;
    return rest;
}

(function main(): void {
    if (!process.argv[2]) {
        throw new Error("Usage: npm start <path-to-specification.json> [path-to-boilerplate.ts] [output-dir]");
    }

    const specification: string = path.resolve(process.cwd(), process.argv[2]);
    const boilerplate: string = path.resolve(process.cwd(), process.argv[3] ?? path.join("boilerplate", "testing.ts"));
    const output: string = path.resolve(process.cwd(), process.argv[4] ?? "output");

    const spec: Structure = JSON.parse(fs.readFileSync(specification, "utf-8")) as Structure;
    fs.mkdirSync(output, { recursive: true });

    const index: string = path.join(output, "index.ts");
    const project: Project = new Project({});

    const source: SourceFile = project.createSourceFile(index, fs.readFileSync(boilerplate, "utf-8"), {
        overwrite: true,
    });

    const baseOptionsType = source.getTypeAlias("BaseOptions");

    if (!baseOptionsType) {
        throw new Error("Boilerplate must define a `BaseOptions` type.");
    }

    const container: ClassDeclaration | undefined = source.getClass("API");

    if (!container) {
        throw new Error("Boilerplate must define an `API` class.");
    }

    generateOptionsTypes(spec, source);
    generateCommands(spec, container, 'ReturnType<API["__run"]>');
    project.saveSync();
})();

/**
 * Every command and menu may add some flags to the pool of available flags.
 * This means that, as we descend the command tree, we need to collect all the
 * flags that have been defined and add them to an options object.
 * Flags with omit: true in the spec are excluded from the options type.
 */
function generateOptionsTypes(
    structure: Structure,
    source: SourceFile,
    breadcrumbs: string[] = [],
    inherited: Record<string, Flag> = {},
): void {
    const command: string = createCommandName(breadcrumbs);
    const allFlags: Record<string, Flag> = { ...inherited, ...(structure.flags ?? {}) };
    const visibleFlags = Object.values(allFlags).filter((f) => !f.omit);

    // Only emit options types for structures that will also have a corresponding
    // command method in the generated API. Non-executable menus (like the root
    // "pulumi" node) do not produce methods, so we skip generating their
    // options types to avoid orphaned interfaces such as `PulumiOptions`.
    const shouldEmitOptions =
        structure.type === "command" || (structure.type === "menu" && structure.executable === true);

    if (shouldEmitOptions) {
        source.addInterface({
            kind: StructureKind.Interface,
            name: createOptionsTypeName(breadcrumbs),
            extends: ["BaseOptions"],
            docs: ["Options for the `" + command + "` command."],
            isExported: true,
            properties: visibleFlags.map(flagToPropertySignature),
        });
    }

    if (structure.type === "menu" && structure.commands) {
        const childInherited = Object.fromEntries(Object.entries(allFlags).map(([k, v]) => [k, baseFlag(v)]));
        for (const [name, child] of Object.entries(structure.commands)) {
            generateOptionsTypes(child, source, [...breadcrumbs, name], childInherited);
        }
    }
}

/**
 * Generate the commands.
 * This creates the CLI invocation by combining all the flags and arguments into a shell command.
 */
function generateCommands(
    structure: Structure,
    container: ClassDeclaration,
    returnType: string,
    breadcrumbs: string[] = [],
    inherited: Record<string, Flag> = {},
): void {
    const allFlags: Record<string, Flag> = { ...inherited, ...(structure.flags ?? {}) };

    if (structure.type === "menu" && structure.commands) {
        const childInherited = Object.fromEntries(Object.entries(allFlags).map(([k, v]) => [k, baseFlag(v)]));
        for (const [name, child] of Object.entries(structure.commands)) {
            generateCommands(child, container, returnType, [...breadcrumbs, name], childInherited);
        }
    }

    if (structure.type === "menu" && !structure.executable) {
        return;
    }

    const parameters: ParameterDeclarationStructure[] = [];
    parameters.push({
        kind: StructureKind.Parameter,
        name: "options",
        type: createOptionsTypeName(breadcrumbs),
    });

    if (structure.type === "command" && structure.arguments) {
        const specification: Arguments = structure.arguments;

        for (let i = 0; i < specification.arguments.length; i++) {
            const argument: Argument = specification.arguments[i];

            const optional: boolean = i >= (specification.requiredArguments ?? 0);
            const variadic: boolean = i === specification.arguments.length - 1 && !!specification.variadic;

            parameters.push({
                kind: StructureKind.Parameter,
                name: sanitiseValueName(argument.name),
                type: convertType(argument.type ?? "string", variadic),
                hasQuestionToken: optional && !variadic,
                isRestParameter: variadic,
            });
        }
    }

    container.addMethod({
        name: sanitiseValueName(breadcrumbs.join("_")),
        parameters,
        statements: generateBody(structure, breadcrumbs, allFlags),
        returnType,
    });
}

/**
 * Emit code that pushes a preset flag value onto __flags.
 * When the flag is not omitted, wrap in a condition so we only add the preset
 * when the user did not provide the option (options.<optName> == null).
 */
function emitPresetFlag(
    writer: { writeLine: (s: string) => void; indent: (fn: () => void) => void },
    flag: Flag,
): void {
    if (flag.preset === undefined) {
        return;
    }
    const wrapCondition = !flag.omit;
    const value: PresetValue = flag.preset;

    function emit(): void {
        if (typeof value === "boolean") {
            if (value) {
                writer.writeLine(`__flags.push("--${flag.name}");`);
            }
            return;
        }
        if (typeof value === "string" || typeof value === "number") {
            writer.writeLine(`__flags.push("--${flag.name}", "" + ${JSON.stringify(value)});`);
            return;
        }
        if (Array.isArray(value)) {
            writer.writeLine(`for (const __preset of ${JSON.stringify(value)}) {`);
            writer.indent(() => writer.writeLine(`__flags.push("--${flag.name}", __preset);`));
            writer.writeLine("}");
            return;
        }
    }

    if (wrapCondition) {
        writer.writeLine(`if (options.${sanitiseValueName(flag.name)} == null) {`);
        writer.indent(emit);
        writer.writeLine(`}`);
    } else {
        emit();
    }
}

/** Generate the body of the commands. */
function generateBody(structure: Structure, breadcrumbs: string[], allFlags: Record<string, Flag>): WriterFunction {
    return (writer) => {
        writer.writeLine("const __final: string[] = [];");
        for (const breadcrumb of breadcrumbs) {
            writer.writeLine(`__final.push("${breadcrumb}");`);
        }
        writer.blankLine();

        /**
         * Flags can be repeatable or unique, and boolean or not boolean.
         * If they're repeatable, we need to loop through the array to add them to the command.
         * If they're boolean, we don't need to add a value after the flag.
         */
        function option(flag: Flag, override: string = ""): void {
            const name: string = override ? override : "options." + sanitiseValueName(flag.name);

            if (flag.repeatable) {
                writer.writeLine(`for (const __item of ${name} ?? []) {`);
                writer.indent(() => option({ ...flag, repeatable: false }, "__item"));
                writer.writeLine("}");
            } else if (flag.type === "boolean") {
                writer.writeLine(`if (${name}) {`);
                writer.indent(() => writer.writeLine(`__flags.push("--${flag.name}");`));
                writer.writeLine("}");
            } else if (flag.required === true) {
                writer.writeLine(`__flags.push("--${flag.name}", "" + ${name});`);
            } else {
                writer.writeLine(`if (${name} != null) {`);
                writer.indent(() => writer.writeLine(`__flags.push("--${flag.name}", "" + ${name});`));
                writer.writeLine("}");
            }

            // Skip trailing blank line for recursive calls (inside a for loop)
            // to avoid biome formatting violations from empty lines before closing braces.
            if (!override) {
                writer.blankLine();
            }
        }

        /**
         * Arguments can be repeatable or unique, and optional or required.
         * If they're repeatable, we need to loop through the array to add them to the command.
         * If they're variadic, we need to concatenate the variadic arguments to the final array.
         */
        function argument(specification: Argument, variadic: boolean, optional: boolean, override: string = ""): void {
            const name: string = override ? override : sanitiseValueName(specification.name);

            if (optional) {
                writer.writeLine(`if (${name} != null) {`);
                writer.indent(() => argument(specification, variadic, false, override));
                writer.writeLine("}");

                return;
            } else if (variadic) {
                writer.writeLine(`for (const __item of ${name} ?? []) {`);
                writer.indent(() => argument(specification, false, false, "__item"));
                writer.writeLine("}");
            } else {
                writer.writeLine(`__arguments.push("" + ${name});`);
            }
        }

        writer.writeLine("const __flags: string[] = [];");
        writer.blankLine();

        // Preset flags (sorted by name for determinism).
        const presetFlags = Object.values(allFlags)
            .filter((f) => f.preset !== undefined)
            .sort((a, b) => a.name.localeCompare(b.name));
        for (const flag of presetFlags) {
            emitPresetFlag(writer, flag);
        }
        if (presetFlags.length > 0) {
            writer.blankLine();
        }

        // Flags from options (only those not omitted).
        const optionFlags = Object.values(allFlags).filter((f) => !f.omit);
        optionFlags.forEach((flag) => option(flag));

        writer.writeLine("__final.push(...__flags);");
        writer.blankLine();

        writer.writeLine("const __arguments: string[] = [];");
        writer.blankLine();

        if (structure.type === "command" && structure.arguments) {
            const specification: Arguments = structure.arguments;
            const variadic: boolean = specification.variadic ?? false;
            const required: number = specification.requiredArguments ?? 0;

            for (let i = 0; i < specification.arguments.length; i++) {
                const optional: boolean = i >= required;
                argument(specification.arguments[i], variadic, optional);
            }
        }

        writer.writeLine("if (__arguments.length > 0) {");

        writer.indent(() => {
            writer.writeLine('__final.push("--");');
            writer.writeLine("__final.push(...__arguments);");
        });

        writer.writeLine("}");
        writer.blankLine();

        writer.writeLine("return this.__run(options, __final);");
    };
}

/**
 * The type system of flags is effectively just the type system of Go,
 * so we need to find appropriate approximations of the types for TypeScript.
 */
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

/** Convert a list of subcommand breadcrumbs into the unconfigured CLI command. */
function createCommandName(breadcrumbs: string[]): string {
    return "pulumi " + breadcrumbs.join(" ");
}

/** Convert a flag or argument name into a valid TypeScript property name. */
function sanitiseValueName(name: string): string {
    const suffix: string = reservedWords.includes(name) ? "_" : "";
    return camelCase(name) + suffix;
}

/** Convert a list of subcommand breadcrumbs into the options type name. */
function createOptionsTypeName(breadcrumbs: string[]): string {
    const command: string = "pulumi " + breadcrumbs.join(" ");
    return pascalise(command) + "Options";
}

/** Convert a flag into a property signature for the options type. */
function flagToPropertySignature(flag: Flag): PropertySignatureStructure {
    return {
        name: sanitiseValueName(flag.name),
        kind: StructureKind.PropertySignature,
        type: convertType(flag.type, flag.repeatable ?? false),
        hasQuestionToken: flag.required === true ? false : true,
        docs: flag.description ? [flag.description] : undefined,
    };
}
