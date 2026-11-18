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

import * as url from "url";
import * as sourceproto from "../proto/source_pb";
import type { SourcePosition } from "./resource";

interface CallSite {
    file?: string;
    line?: number;
    column?: number;
}

const framePositionRegExp = /^\s*at .* \((?<file>.*):(?<line>[0-9]+):(?<column>[0-9]+)\)$/;

/**
 * Returns the list of callsites for the current stack. The top of the stack is at index 0.
 *
 * Some callsites may not contain position information. It is up to the caller to detect this situation.
 */
function callsites(): CallSite[] {
    // NOTE: it would be lovely to implement this using prepareStackTrace and actual V8 callsite information.
    // Unfortunately, the V8 source locations are not mapped from transpiled locations to user locations, and
    // Node 20.x does not expose an easy way to access source map information. Furthermore, ts-node may inject
    // hooks that perform this mapping as part of prepareStackTrace. Instead of using prepareStackTrace, then,
    // we parse its output as returned via `new Error().stack`. The stack trace will look like:
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
        return (
            stackTrace
                ?.split("\n")
                ?.slice(1)
                ?.map((frame) => {
                    const { file, line, column } = framePositionRegExp.exec(frame)?.groups || {};
                    if (!file || !line || !column || file.startsWith("node:")) {
                        return {};
                    }
                    const lineNum = parseInt(line, 10);
                    const colNum = parseInt(column, 10);
                    if (Number.isNaN(lineNum) || Number.isNaN(colNum)) {
                        return {};
                    }
                    return { file, line: lineNum, column: colNum };
                }) || []
        );
    } finally {
        Error.stackTraceLimit = stackTraceLimit;
    }
}

/** @internal */
export function getStackTrace(skip: number): (SourcePosition | undefined)[] {
    let stack = callsites();
    if (stack.length < skip + 1) {
        return [];
    }
    stack = stack.slice(skip + 1);

    return stack.map((frame) => {
        const { file, line, column } = frame;
        if (!file || !line || !column) {
            return undefined;
        }

        return {
            uri: url.pathToFileURL(file).toString(),
            line,
            column,
        };
    });
}

/** @internal */
export function marshalSourcePosition(sourcePosition?: SourcePosition) {
    if (sourcePosition === undefined) {
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
            frame.setPc(marshalSourcePosition(pos)!);
            return frame;
        }),
    );
    return trace;
}
