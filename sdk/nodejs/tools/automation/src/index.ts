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

import { Project, SourceFile, StructureKind } from "ts-morph";

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
  generateOptionsTypes(spec, source);

  project.saveSync();
})();

function generateOptionsTypes(
  structure: Structure,
  source: SourceFile,
  breadcrumbs: string[] = [],
  inherited: Record<string, Flag> = {}
): void {
  const command: string = "pulumi " + breadcrumbs.join(" ");
  const options: string = pascalise(command) + "Options";

  const flags: Record<string, Flag> = { ...inherited, ...structure.flags ?? {} };

  source.addInterface({
    kind: StructureKind.Interface,
    name: pascalise(command) + "Options",
    docs: ["Options for the `" + command + "` command."],
    isExported: true,
    properties: Object.entries(flags).map(([name, flag]) => ({
      name: camelCase(flag.name),
      type: convertType(flag.type, flag.repeatable ?? false),
      docs: flag.description ? [flag.description] : undefined,
    })),
  });

  if (structure.type === "menu" && structure.commands) {
    for (const [name, child] of Object.entries(structure.commands)) {
      generateOptionsTypes(child, source, [...breadcrumbs, name]);
    }
  }
}

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
