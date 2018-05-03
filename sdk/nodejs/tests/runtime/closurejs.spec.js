"use strict";
// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
// tslint:disable:max-line-length
const assert = require("assert");
const os_1 = require("os");
const index_1 = require("../../index");
const resource_1 = require("../../resource");
const util_1 = require("../util");
// This group of tests ensure that we serialize closures properly.
describe("closure", () => {
    const cases = [];

    const version = Number(process.version.match(/^v(\d+)\.\d+/)[1]);

    if (version >= 8) {
        console.log("Process: " + process.version + "\n");
        console.log("Version: " + version + "\n");
        cases.push({
            title: "Async anonymous function closure (js)",
            // tslint:disable-next-line
            func: async function (a) { await a; },
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return async function (a) { await a; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });

        cases.push({
            title: "Async anonymous function closure - extra space (js)",
            // tslint:disable-next-line
            func: async  function (a) { await a; },
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return async function (a) { await a; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });

        cases.push({
            title: "Async named function closure (js)",
            // tslint:disable-next-line
            func: async function foo(a) { await a; },
            expectText: `exports.handler = __foo;

function __foo() {
  return (function() {
    with({ foo: __foo }) {

return async function /*foo*/(a) { await a; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });

        cases.push({
            title: "Async arrow function closure (js)",
            // tslint:disable-next-line
            func: async (a) => { await a; },
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return async (a) => { await a; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    // Make a callback to keep running tests.
    let remaining = cases;
    while (true) {
        const test = remaining.shift();
        if (!test) {
            return;
        }
        // if (test.title !== "Invocation of async function that does not capture this.") {
        //     continue;
        // }
        it(test.title, util_1.asyncTest(() => __awaiter(this, void 0, void 0, function* () {
            // Run pre-actions.
            if (test.pre) {
                test.pre();
            }
            // Invoke the test case.
            if (test.expectText) {
                const text = yield index_1.runtime.serializeFunctionAsync(test.func);
                assert.equal(text, test.expectText);
            }
            else {
                const message = yield util_1.assertAsyncThrows(() => __awaiter(this, void 0, void 0, function* () {
                    yield index_1.runtime.serializeFunctionAsync(test.func);
                }));
                // replace real locations with (0,0) so that our test baselines do not need to
                // updated any time this file changes.
                const regex = /\([0-9]+,[0-9]+\)/g;
                const withoutLocations = message.replace(regex, "(0,0)");
                if (test.error) {
                    assert.equal(test.error, withoutLocations);
                }
            }
        })));
        // Schedule any additional tests.
        if (test.afters) {
            remaining = test.afters.concat(remaining);
        }
    }
});