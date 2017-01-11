// Copyright 2017 Marapongo, Inc. All rights reserved.

import * as assert from "assert";
import {fs} from "nodets";
import * as os from "os";
import * as path from "path";
import {compiler, pack} from "../../lib";
import {asyncTest} from "../util";

// This test suite loops through a list of programs and compiles each one.  For each, the expected set of diagnostics
// are compared and, if successful, the lowered MuPack/MuIL AST is compared to the expected final output.

let testCases: string[] = [
    "empty",

    "modules/var_1",
    "modules/var_exp_1",
    "modules/func_1",
    "modules/func_exp_1",
    "modules/func_exp_def_1",
    "modules/class_1",
    "modules/class_exp_1",

    // These are not quite real-world-code, but they are more complex "integration" style tests.
    "scenarios/point",
];

describe("outputs", () => {
    const messageBaselineFile: string = "messages.txt";
    const outputTreeBaselineFile: string = "Mu.out.json";
    for (let testCase of testCases) {
        it(`${testCase} successfully produces the expected results`, asyncTest(async () => {
            let testPath: string = path.join(__dirname, testCase);

            // First, compile the code.
            let output: compiler.Compilation = await compiler.compile(testPath);

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
            assert.strictEqual(output.diagnostics.length, expectedMessages.length, "Expected message count to match");

            // Now format them and ensure the text of the messages are correct.
            for (let i = 0; i < expectedMessages.length; i++) {
                let actual: string = output.formatDiagnostic(i);
                assert.strictEqual(actual, expectedMessages[i], `Expected message #{i}'s text to match`);
            }

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

            if (output.tree) {
                if (expectedOutputTree) {
                    let mupackTree: pack.Package = await compiler.transpile(output);
                    let mupackTreeText: string = JSON.stringify(mupackTree, null, 4) + "\n";

                    // Do a line-by-line comparison to make debugging failures nicer.
                    let actualLines: string[] = mupackTreeText.split("\n");
                    let expectLines: string[] = expectedOutputTree.split("\n");
                    assert.strictEqual(actualLines.length, expectLines.length, "Expected tree line count to match");
                    for (let i = 0; i < actualLines.length && i < expectLines.length; i++) {
                        assert.strictEqual(actualLines[i], expectLines[i], `Expected tree line #${i} to match`);
                    }
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

