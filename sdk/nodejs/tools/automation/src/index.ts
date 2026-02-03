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

import { Project, SourceFile } from "ts-morph";

import type { Flag, Structure } from "./types";

function main(): void {
  const specPath = process.argv[2];
  if (!specPath) {
    console.error("Usage: npm start <path-to-specification.json>");
    process.exit(1);
  }

  const resolved = path.resolve(process.cwd(), specPath);
  let raw: string;
  try {
    raw = fs.readFileSync(resolved, "utf-8");
  } catch (err) {
    console.error(`Failed to read specification at ${resolved}:`, err);
    process.exit(1);
  }

  let spec: Structure;
  try {
    spec = JSON.parse(raw) as Structure;
  } catch (err) {
    console.error(`Failed to parse specification JSON:`, err);
    process.exit(1);
  }

  const outputDir = path.join(process.cwd(), "output");
  try {
    fs.mkdirSync(outputDir, { recursive: true });
  } catch (err) {
    console.error(`Failed to create output directory ${outputDir}:`, err);
    process.exit(1);
  }

  const indexPath = path.join(outputDir, "index.ts");
  try {
    const project = new Project({});
    const indexFile = project.createSourceFile(indexPath, "", { overwrite: true });

    generateOptionsTypes(spec, indexFile);

    project.saveSync();
  } catch (err) {
    console.error(`Failed to write ${indexPath}:`, err);
    process.exit(1);
  }
}

main();

// --- Code generation -------------------------------------------------------

function generateOptionsTypes(spec: Structure, file: SourceFile): void {
  const visited = new Set<string>();

  emitNode(spec, [], undefined);

  function emitNode(node: Structure, pathParts: string[], parentType: string | undefined): void {
    const typeName = optionsTypeNameFor(pathParts);
    if (visited.has(typeName)) {
      return;
    }
    visited.add(typeName);

    const flags = node.flags ?? {};
    const commandPath = commandPathFor(pathParts);

    const extendsClause = parentType ? ` extends ${parentType}` : "";
    const lines: string[] = [];
    lines.push(`/** Flags for the \`${commandPath}\` command */`);
    lines.push(`export interface ${typeName}${extendsClause} {`);

    const flagLines = flagProperties(flags);
    if (flagLines.length > 0) {
      for (const line of flagLines) {
        lines.push(line);
      }
    }

    lines.push("}");
    file.addStatements(lines.join("\n"));

    if (node.type === "menu" && node.commands) {
      for (const [name, child] of Object.entries(node.commands)) {
        emitNode(child, [...pathParts, name], typeName);
      }
    }
  }
}

function commandPathFor(pathParts: string[]): string {
  if (pathParts.length === 0) {
    return "pulumi";
  }
  return "pulumi " + pathParts.join(" ");
}

function optionsTypeNameFor(pathParts: string[]): string {
  const pascal = pathParts.map(toPascalCase).join("");
  return `Pulumi${pascal}Options`;
}

function toPascalCase(name: string): string {
  return name
    .split(/[^a-zA-Z0-9]+/)
    .filter((part) => part.length > 0)
    .map((part) => part[0]!.toUpperCase() + part.slice(1))
    .join("");
}

function flagProperties(flags: Record<string, Flag>): string[] {
  const lines: string[] = [];

  for (const [name, flag] of Object.entries(flags)) {
    if (flag.description && flag.description.trim() !== "") {
      lines.push(`  /** ${escapeJsDoc(flag.description.trim())} */`);
    }
    const tsType = flagTsType(flag);
    // Use a quoted property name so we don't have to worry about characters
    // that are invalid in identifiers (for example, hyphens).
    lines.push(`  "${name}"?: ${tsType};`);
  }

  return lines;
}

/** Escape text for use inside a JSDoc comment (avoid closing the comment). */
function escapeJsDoc(text: string): string {
  return text.replace(/\*\//g, "* /").replace(/\n/g, " ");
}

function flagTsType(flag: Flag): string {
  const baseType = (() => {
    switch (flag.type) {
      case "boolean":
        return "boolean";
      case "int":
        return "number";
      default:
        return "string";
    }
  })();

  if (flag.repeatable) {
    return `${baseType}[]`;
  }

  return baseType;
}

