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
import * as url from "url";
import * as sourceproto from "../proto/source_pb";
import type { SourcePosition } from "./resource";

interface CallSite {
    file?: string;
    moduleFile?: string;
    line?: number;
    column?: number;
}

interface StackFrame extends SourcePosition {
    module?: string;
    origin?: "pulumi" | "runtime";
}

const packageNames = new Map<string, string | undefined>();

function getPackageName(file: string): string | undefined {
    const directory = path.dirname(file);
    if (packageNames.has(directory)) {
        return packageNames.get(directory);
    }
    let name: string | undefined;
    try {
        const manifest = JSON.parse(fs.readFileSync(path.join(directory, "package.json"), "utf8"));
        name = typeof manifest.name === "string" ? manifest.name : undefined;
    } catch (error) {
        // An unreadable or invalid manifest is unclassified, rather than inheriting an enclosing package's identity.
        if ((error as NodeJS.ErrnoException).code === "ENOENT" && path.dirname(directory) !== directory) {
            name = getPackageName(directory);
        }
    }
    packageNames.set(directory, name);
    return name;
}

const framePositionRegExp = /^\s*at .* \((?<file>.*):(?<line>[0-9]+):(?<column>[0-9]+)\)$/;
const anonymousFramePositionRegExp = /^\s*at (?:async )?(?<file>.*):(?<line>[0-9]+):(?<column>[0-9]+)$/;

/**
 * Returns the list of callsites for the current stack. The top of the stack is at index 0.
 *
 * Some callsites may not contain position information. It is up to the caller to detect this situation.
 */
function callsites(): CallSite[] {
    // Keep the installed formatter's source-mapped locations, including mappings supplied by ts-node.
    // Raw V8 locations are used separately for package identity: source maps can point outside a package.
    // The formatted stack trace will look like:
    //
    //     Error
    //         at foo (file:line:column)
    //         at bar (file:line:column)
    //         at baz (file:line:column)
    //
    // We process this by splitting it into lines and parsing the position information out of each line.
    const stackTraceLimit = Error.stackTraceLimit;
    try {
        Error.stackTraceLimit = Number.POSITIVE_INFINITY;

        const stackTrace = new Error().stack;
        const lines = stackTrace?.split("\n").slice(1) || [];
        const prepare = Error.prepareStackTrace;
        let moduleFiles: (string | null)[];
        try {
            Error.prepareStackTrace = (_, sites) => sites;
            // Capture at the same depth without the formatter. Only this helper's own line differs.
            const frames = new Error().stack as unknown as NodeJS.CallSite[];
            moduleFiles = frames.map((frame) => frame.getFileName());
        } finally {
            Error.prepareStackTrace = prepare;
        }
        // Custom formatters can omit frames; avoid assigning identities to different frames in that case.
        const aligned = moduleFiles.length === lines.length;
        return lines.map((frame, index) => {
            const moduleFile = aligned ? moduleFiles[index] || undefined : undefined;
            const { file, line, column } =
                (framePositionRegExp.exec(frame) || anonymousFramePositionRegExp.exec(frame))?.groups || {};
            if (!file || !line || !column) {
                return { moduleFile };
            }
            const lineNum = parseInt(line, 10);
            const colNum = parseInt(column, 10);
            if (Number.isNaN(lineNum) || Number.isNaN(colNum)) {
                return { moduleFile };
            }
            return { file, moduleFile, line: lineNum, column: colNum };
        });
    } finally {
        Error.stackTraceLimit = stackTraceLimit;
    }
}

/**
 * Captures its caller's stack, omitting skip additional caller frames.
 * The callsites and getStackTrace helpers are always omitted.
 *
 * @internal
 */
export function getStackTrace(skip: number): (StackFrame | undefined)[] {
    skip += 2;
    let stack = callsites();
    if (stack.length < skip) {
        return [];
    }
    stack = stack.slice(skip);

    return stack.map((frame) => {
        const { file, moduleFile, line, column } = frame;
        const identityFile = moduleFile || file;
        if (identityFile?.startsWith("node:")) {
            return { uri: identityFile, line: line || 0, column: column || 0, module: identityFile, origin: "runtime" };
        }
        if (!file || !line || !column) {
            return undefined;
        }
        let filename: string;
        let moduleFilename: string;
        try {
            filename = file.startsWith("file:") ? url.fileURLToPath(file) : file;
            moduleFilename = identityFile?.startsWith("file:")
                ? url.fileURLToPath(identityFile)
                : identityFile || filename;
        } catch {
            return undefined;
        }
        if (!path.isAbsolute(filename)) {
            return undefined;
        }
        const module = getPackageName(moduleFilename);
        // ts-node's compile-and-load hooks support TypeScript execution alongside Node's built-in loader.
        return {
            uri: url.pathToFileURL(filename).toString(),
            line,
            column,
            module,
            origin: module === "@pulumi/pulumi" ? "pulumi" : module === "ts-node" ? "runtime" : undefined,
        };
    });
}

/** @internal */
export function marshalSourcePosition(sourcePosition?: SourcePosition) {
    if (sourcePosition === undefined || sourcePosition.uri.startsWith("node:")) {
        return undefined;
    }
    const pos = new sourceproto.SourcePosition();
    pos.setUri(sourcePosition.uri);
    pos.setLine(sourcePosition.line);
    pos.setColumn(sourcePosition.column);
    return pos;
}

/** @internal */
export function marshalStackTrace(stackTrace?: (SourcePosition | undefined)[]) {
    if (stackTrace === undefined) {
        return undefined;
    }
    const trace = new sourceproto.StackTrace();
    trace.setFramesList(
        stackTrace.map((pos) => {
            const frame = new sourceproto.StackFrame();
            const captured = pos as StackFrame | undefined;
            frame.setPc(marshalSourcePosition(pos)!);
            if (captured?.module) {
                frame.setModule(captured.module);
            }
            if (captured?.origin) {
                frame.setOrigin(captured.origin);
            }
            return frame;
        }),
    );
    return trace;
}
