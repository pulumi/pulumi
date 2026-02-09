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

import { CodeBlockWriter, ParameterDeclarationStructure, Project, ReturnStatement, SourceFile, StructureKind } from "ts-morph";

import type { Command, Flag, Structure } from "./types";

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
    const options: string = pascalise(command) + "Options";

    const flags: Record<string, Flag> = { ...inherited, ...(structure.flags ?? {}) };

    source.addInterface({
        kind: StructureKind.Interface,
        name: createOptionsTypeName(breadcrumbs),
        docs: ["Options for the `" + command + "` command."],
        isExported: true,
        properties: Object.entries(flags).map(([name, flag]) => ({
            name: convertName(flag.name),
            type: convertType(flag.type, flag.repeatable ?? false),
            hasQuestionToken: true,
            docs: flag.description ? [flag.description] : undefined,
        })),
    });

    if (structure.type === "menu" && structure.commands) {
        for (const [name, child] of Object.entries(structure.commands)) {
            generateOptionsTypes(child, source, [...breadcrumbs, name]);
        }
    }
}

function generateCommands(
  structure: Structure,
  source: SourceFile,
  breadcrumbs: string[] = [],
): void {
  if (structure.type === "menu") {
    if (structure.commands) {
      for (const [name, child] of Object.entries(structure.commands)) {
        generateCommands(child, source, [...breadcrumbs, name]);
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
      const argument = specification.arguments[i];
      const hasQuestionToken = i >= (specification.requiredArguments ?? 0);
      const isRestParameter = i === specification.arguments.length - 1 && (specification.variadic ?? false);

      parameters.push({
        kind: StructureKind.Parameter,
        name: convertName(argument.name),
        type: convertType(argument.type ?? "string", isRestParameter),
        hasQuestionToken: hasQuestionToken && !isRestParameter, // TS doesn't allow optional rest parameters
        isRestParameter,
      });
    }
  }

  source.addFunction({
    name: convertName(breadcrumbs.join(" ")),
    isExported: true,
    // isAsync: true,
    parameters,
    statements: writer => generateBody(structure, writer, breadcrumbs),
    returnType: "string",
  });
}

function generateBody(structure: Structure, writer: CodeBlockWriter, breadcrumbs: string[]): void {
  writer.writeLine("const __arguments: string[] = [];");
  writer.blankLine();

  function pushArgument(name: string, isRestParameter: boolean): void {
    if (isRestParameter) {
      writer.writeLine(`__arguments.push(...${name});`);
    } else {
      writer.writeLine(`__arguments.push(${name});`);
    }
  }

  if (structure.type === "command" && structure.arguments) {
    const specification = structure.arguments;

    for (let i = 0; i < specification.arguments.length; i++) {
      const argument = specification.arguments[i];
      const hasQuestionToken = i >= (specification.requiredArguments ?? 0);
      const isRestParameter = i === specification.arguments.length - 1 && (specification.variadic ?? false);
      const name = convertName(argument.name);

      if (hasQuestionToken) {
        writer.writeLine(`if (${name} != null) {`);
        writer.indent(() => pushArgument(name, isRestParameter));
        writer.writeLine("}");
      } else {
        pushArgument(name, isRestParameter);
      }

      writer.blankLine();
    }
  }

  function pushOption(name: string, type: string, repeatable: boolean): void {
    const cleanName: string = convertName(name);

    if (repeatable) {
      writer.writeLine(`for (const __item of __options.${cleanName}) {`);

      writer.indent(() => {
        writer.writeLine(`__arguments.push('--${name}')`);

        if (type !== "boolean") {
          writer.writeLine(`__arguments.push('' + __item)`);
        }
      });

      writer.writeLine("}");
      return;
    }

    writer.writeLine(`__arguments.push('--${name}')`);

    if (type !== "boolean") {
      writer.writeLine(`__arguments.push('' + __options.${cleanName})`);
    }
  }

  for (const [name, flag] of Object.entries(structure.flags ?? {})) {
    writer.writeLine(`if (__options.${convertName(name)} != null) {`);
    writer.indent(() => pushOption(name, flag.type, flag.repeatable ?? false));
    writer.writeLine("}");

    writer.blankLine();
  }

  const command: string = "pulumi " + breadcrumbs.join(" ");
  writer.writeLine(`return '${command} ' + __arguments.join(' ');`);
}

function createOptionsTypeName(breadcrumbs: string[]): string {
  return "Pulumi" + pascalise(breadcrumbs.join(" ")) + "Options";
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

function convertName(name: string): string {
  const prefix = isReservedWord(name) ? "__" : "";
  return prefix + camelCase(name);
}

function isReservedWord(name: string): boolean {
  const reservedWords: string[] = [ "console", "import", "new", "package", "type" ];
  return reservedWords.includes(name);
}
