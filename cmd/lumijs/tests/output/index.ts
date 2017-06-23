// Copyright 2016-2017, Pulumi Corporation
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

import * as assert from "assert";
import {fs} from "nodejs-ts";
import * as os from "os";
import * as path from "path";
import {compiler, pack} from "../../lib";
import {asyncTest} from "../util";

// This test suite loops through a list of programs and compiles each one.  For each, the expected set of diagnostics
// are compared and, if successful, the lowered LumiPack/LumiIL AST is compared to the expected final output.

let testCases: string[] = [
    // Basic language constructs.
    "basic/empty",
    "basic/empty_yaml",
    "basic/literals",
    "basic/arrays",
    "basic/maps",
    "basic/super",
    "basic/null_undefined",
    "basic/props",

    // Module members and exports.
    "modules/var_1",
    "modules/var_exp_1",
    "modules/func_1",
    "modules/func_exp_1",
    "modules/func_exp_def_1",
    "modules/func_cross_call",
    "modules/class_1",
    "modules/class_exp_1",
    "modules/iface_1",
    "modules/iface_exp_1",
    "modules/reexport",
    "modules/reexport_all",
    "modules/reexport_rename",
    "modules/export",
    "modules/intra",

    // These are not quite real-world-code, but they are more complex "integration" style tests.
    "scenarios/point",
];

describe("outputs", () => {
    const messageBaselineFile: string = "messages.txt";
    const outputTreeBaselineFile: string = pack.lumipackBase+".json";
    for (let testCase of testCases) {
        it(`${testCase} successfully produces the expected results`, asyncTest(async () => {
            let testPath: string = path.join(__dirname, testCase);

            // First, compile the code.
            let output: compiler.CompileResult = await compiler.compile(testPath);

            // Ensure that the expected number of messages got output.
            let expectedMessages: string[];
            try {
                expectedMessages = (await fs.readFile(path.join(testPath, messageBaselineFile))).split("\n");
            }
            catch (err) {
                // Permit missing file errors; we will simply assume that means no messages are expected.
                if (err.code === "ENOENT") {
                    expectedMessages = [];
                }
                else {
                    throw err;
                }
            }

            // Now format them and ensure the text of the messages are correct.
            let actualMessages: string[] = [];
            for (let i = 0; i < output.diagnostics.length; i++) {
                actualMessages.push(output.formatDiagnostic(i));
            }
            compareLines(actualMessages, expectedMessages, "messages");

            // Next, see if there is an expected program tree (possibly none in the case of fatal errors).
            let expectedOutputTree: string | undefined;
            try {
                expectedOutputTree = await fs.readFile(path.join(testPath, outputTreeBaselineFile));
            }
            catch (err) {
                // Permit missing file errors; we will simply assume that means no output is expected.
                if (err.code !== "ENOENT") {
                    throw err;
                }
            }

            if (output.pkg) {
                if (expectedOutputTree) {
                    let mupackTreeText: string = JSON.stringify(output.pkg, null, 4) + os.EOL;

                    // Do a line-by-line comparison to make debugging failures nicer.
                    let actualLines: string[] = mupackTreeText.split(os.EOL);
                    let expectLines: string[] = expectedOutputTree.split(os.EOL);
                    compareLines(actualLines, expectLines, "AST");
                }
                else {
                    assert(false, "Expected an empty program tree, but one was returned");
                }
            }
            else if (expectedOutputTree) {
                assert(false, "Expected a non-empty program tree, but an empty one was returned");
            }
        }));
    }
});

function compareLines(actuals: string[], expects: string[], label: string): void {
    let mismatches: { line: number, actual: string, expect: string }[] = [];
    for (let i = 0; i < actuals.length && i < expects.length; i++) {
        if (actuals[i] !== expects[i]) {
            mismatches.push({
                line:   i+1,
                actual: actuals[i],
                expect: expects[i],
            });
        }
    }
    if (mismatches.length > 0) {
        // We batch up the mismatches so we can report them in one batch, easing debugging.
        let expect: string = "";
        let actual: string = "";
        for (let mismatch of mismatches) {
            actual += `${mismatch.line}: ${mismatch.actual}${os.EOL}`;
            expect += `${mismatch.line}: ${mismatch.expect}${os.EOL}`;
        }
        assert.strictEqual(actual, expect, `Expected ${label} to match; ${mismatches.length} did not`);
    }
    assert.strictEqual(actuals.length, expects.length, `Expected ${label} line count to match`);
}

