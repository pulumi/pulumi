// Copyright 2016-2018, Pulumi Corporation.
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

// tslint:disable

import * as assert from "assert";
import { EOL } from "os";
import { runtime } from "../../index";
import * as pulumi from "../../index";
import { output } from "../../output";
import { assertAsyncThrows, asyncTest } from "../util";
import * as typescript from "typescript";
import * as semver from "semver";

import * as deploymentOnlyModule from "./deploymentOnlyModule";

interface ClosureCase {
    pre?: () => void;               // an optional function to run before this case.
    title: string;                  // a title banner for the test case.
    func?: Function;                // the function whose body and closure to serialize.
    factoryFunc?: Function;         // the function whose body and closure to serialize (as a factory).
    expectText?: string;            // optionally also validate the serialization to JavaScript text.
    error?: string;                 // error message we expect to be thrown if we are unable to serialize closure.
    afters?: ClosureCase[];         // an optional list of test cases to run afterwards.
    allowSecrets?: boolean;         // optionally allow secrets to be captured.
}

/** @internal */
export const exportedValue = 42;

// This group of tests ensure that we serialize closures properly.
describe("closure", () => {
    const cases: ClosureCase[] = [];

    cases.push({
        title: "Empty function closure",
        func: function () { },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return function () { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Empty named function",
        func: function f() { },
        expectText: `exports.handler = __f;

function __f() {
  return (function() {
    with({ f: __f }) {

return function /*f*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Named function with self-reference",
        func: function f() { f(); },
        expectText: `exports.handler = __f;

function __f() {
  return (function() {
    with({ f: __f }) {

return function /*f*/() { f(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Function closure with this capture",
        func: function () { console.log(this); },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return function () { console.log(this); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Function closure with this and arguments capture",
        // @ts-ignore: this is just test code.
        func: function () { console.log(this + arguments); },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return function () { console.log(this + arguments); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Empty arrow closure",
        func: () => { },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return () => { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Arrow closure with this capture",
        // @ts-ignore: this is just test code.
        func: () => { console.log(this); },
        expectText: undefined,
        error:
`Error serializing function 'func': tsClosureCases.js(0,0)

function 'func': tsClosureCases.js(0,0): which could not be serialized because
  arrow function captured 'this'. Assign 'this' to another name outside function and capture that.

Function code:
  () => { console.log(this); }
`,
    });

    const awaiterCode =
`
function __f1(__0, __1, __2, __3) {
  return (function() {
    with({  }) {

return function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`;

    cases.push({
        title: "Async lambda that does not capture this",
        func: async () => { },
        expectText: `exports.handler = __f0;
${awaiterCode}
function __f0() {
  return (function() {
    with({ __awaiter: __f1 }) {

return () => __awaiter(void 0, void 0, void 0, function* () { });

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Async lambda that does capture this",
        // @ts-ignore: this is just test code.
        func: async () => { console.log(this); },
        expectText: `exports.handler = __f0;
${awaiterCode}
function __f0() {
  return (function() {
    with({ __awaiter: __f1 }) {

return () => __awaiter(void 0, void 0, void 0, function* () { console.log(this); });

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Async function that does not capture this",
        func: async function() { },
        expectText: `exports.handler = __f0;
${awaiterCode}
function __f0() {
  return (function() {
    with({ __awaiter: __f1 }) {

return function () {
            return __awaiter(this, void 0, void 0, function* () { });
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Async function that does capture this",
        func: async function () { console.log(this); },
        expectText: `exports.handler = __f0;
${awaiterCode}
function __f0() {
  return (function() {
    with({ __awaiter: __f1 }) {

return function () {
            return __awaiter(this, void 0, void 0, function* () { console.log(this); });
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Arrow closure with this and arguments capture",
        // @ts-ignore: this is just test code.
        func: (function() { return () => { console.log(this + arguments); } }).apply(this, [0, 1]),
        expectText: undefined,
        error: `Error serializing function '<anonymous>': tsClosureCases.js(0,0)

function '<anonymous>': tsClosureCases.js(0,0): which could not be serialized because
  arrow function captured 'this'. Assign 'this' to another name outside function and capture that.

Function code:
  () => { console.log(this + arguments); }
`,
    });

    cases.push({
        title: "Arrow closure with this capture inside function closure",
        func: function () { () => { console.log(this); } },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return function () { () => { console.log(this); }; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Arrow closure with this and arguments capture inside function closure",
        // @ts-ignore: this is just test code.
        func: function () { () => { console.log(this + arguments); } },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return function () { () => { console.log(this + arguments); }; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    {
        class Task {
            run: any;
            constructor() {
                this.run = async function() { };
            }
        }

        const task = new Task();

        cases.push({
            title: "Invocation of async function that does not capture this #1",
            func: async function() { await task.run(); },
            expectText: `exports.handler = __f0;

var __task = {run: __f2};

function __f1(__0, __1, __2, __3) {
  return (function() {
    with({  }) {

return function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ __awaiter: __f1 }) {

return function () {
                    return __awaiter(this, void 0, void 0, function* () { });
                };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ __awaiter: __f1, task: __task }) {

return function () {
                return __awaiter(this, void 0, void 0, function* () { yield task.run(); });
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class Task {
            run: any;
            constructor() {
                this.run = async function() { console.log(this); };
            }
        }

        const task = new Task();

        cases.push({
            title: "Invocation of async function that does capture this #1",
            func: async function() { await task.run(); },
            expectText: `exports.handler = __f0;

var __task_proto = {};
Object.defineProperty(__task_proto, "constructor", { configurable: true, writable: true, value: __f2 });
var __task = Object.create(__task_proto);
__task.run = __f3;

function __f1(__0, __1, __2, __3) {
  return (function() {
    with({  }) {

return function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ __awaiter: __f1 }) {

return function /*constructor*/() {
                this.run = function () {
                    return __awaiter(this, void 0, void 0, function* () { console.log(this); });
                };
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({ __awaiter: __f1 }) {

return function () {
                    return __awaiter(this, void 0, void 0, function* () { console.log(this); });
                };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ __awaiter: __f1, task: __task }) {

return function () {
                return __awaiter(this, void 0, void 0, function* () { yield task.run(); });
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class Task {
            run: any;
            constructor() {
                this.run = async () => { };
            }
        }

        const task = new Task();

        cases.push({
            title: "Invocation of async lambda that does not capture this #1",
            func: async function() { await task.run(); },
            expectText: `exports.handler = __f0;

var __task = {run: __f2};

function __f1(__0, __1, __2, __3) {
  return (function() {
    with({  }) {

return function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ __awaiter: __f1 }) {

return () => __awaiter(this, void 0, void 0, function* () { });

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ __awaiter: __f1, task: __task }) {

return function () {
                return __awaiter(this, void 0, void 0, function* () { yield task.run(); });
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class Task {
            run: any;
            constructor() {
                this.run = async () => { console.log(this); };
            }
        }

        const task = new Task();

        cases.push({
            title: "Invocation of async lambda that capture this #1",
            func: async function() { await task.run(); },
            expectText: undefined,
            error: `Error serializing function 'func': tsClosureCases.js(0,0)

function 'func': tsClosureCases.js(0,0): captured
  variable 'task' which indirectly referenced
    function '<anonymous>': tsClosureCases.js(0,0): which could not be serialized because
      arrow function captured 'this'. Assign 'this' to another name outside function and capture that.

Function code:
  () => __awaiter(this, void 0, void 0, function* () { console.log(this); })
`,
        });
    }

    cases.push({
        title: "Empty function closure w/ args",
        func: function (x: any, y: any, z: any) { },
        expectText: `exports.handler = __f0;

function __f0(__0, __1, __2) {
  return (function() {
    with({  }) {

return function (x, y, z) { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Empty arrow closure w/ args",
        func: (x: any, y: any, z: any) => { },
        expectText: `exports.handler = __f0;

function __f0(__0, __1, __2) {
  return (function() {
    with({  }) {

return (x, y, z) => { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    // Serialize captures.
    cases.push({
        title: "Doesn't serialize global captures",
        func: () => { console.log("Just a global object reference"); },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return () => { console.log("Just a global object reference"); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });
    {
        const a = -0;
        const b = -0.0;
        const c = Infinity;
        const d = -Infinity;
        const e = NaN;
        const f = Number.MAX_SAFE_INTEGER;
        const g = Number.MAX_VALUE;
        const h = Number.MIN_SAFE_INTEGER;
        const i = Number.MIN_VALUE;

        cases.push({
            title: "Handle edge-case literals",
            func: () => { const x = [a, b, c, d, e, f, g, h, i]; },
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ a: -0, b: -0, c: Infinity, d: -Infinity, e: NaN, f: 9007199254740991, g: 1.7976931348623157e+308, h: -9007199254740991, i: 5e-324 }) {

return () => { const x = [a, b, c, d, e, f, g, h, i]; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }
    {
        const wcap = "foo";
        const xcap = 97;
        const ycap = [ true, -1, "yup" ];
        const zcap = {
            a: "a",
            b: false,
            c: [ 0 ],
        };
        cases.push({
            title: "Serializes basic captures",
            func: () => { console.log(wcap + `${xcap}` + ycap.length + eval(zcap.a + zcap.b + zcap.c)); },
            expectText: `exports.handler = __f0;

var __ycap = [true, -1, "yup"];
var __zcap = {};
__zcap.a = "a";
__zcap.b = false;
var __zcap_c = [0];
__zcap.c = __zcap_c;

function __f0() {
  return (function() {
    with({ wcap: "foo", xcap: 97, ycap: __ycap, zcap: __zcap }) {

return () => { console.log(wcap + \`\${xcap}\` + ycap.length + eval(zcap.a + zcap.b + zcap.c)); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }
    {
        let nocap1 = 1, nocap2 = 2, nocap3 = 3, nocap4 = 4, nocap5 = 5, nocap6 = 6, nocap7 = 7;
        let nocap8 = 8, nocap9 = 9, nocap10 = 10;
        let cap1 = 100, cap2 = 200, cap3 = 300, cap4 = 400, cap5 = 500, cap6 = 600, cap7 = 700;
        let cap8 = 800;

        const functext = `(nocap1, nocap2) => {
    let zz = nocap1 + nocap2; // not a capture: args
    let yy = nocap3; // not a capture: var later on
    if (zz) {
        zz += cap1; // true capture
        let cap1 = 9; // because let is properly scoped
        zz += nocap4; // not a capture
        var nocap4 = 7; // because var is function scoped
        zz += cap2; // true capture
        const cap2 = 33;
        var nocap3 = 8; // block the above capture
    }
    let f1 = (nocap5) => {
        yy += nocap5; // not a capture: args
        cap3++; // capture
    };
    let f2 = (function (nocap6) {
        zz += nocap6; // not a capture: args
        if (cap4) { // capture
            yy = 0;
        }
    });
    let www = nocap7(); // not a capture; it is defined below
    if (true) {
        function nocap7() {
        }
    }
    let [{t: [nocap8]},,nocap9 = "hello",...nocap10] = [{t: [true]},null,undefined,1,2];
    let vvv = [nocap8, nocap9, nocap10]; // not a capture; declarations from destructuring
    let aaa = { // captures in property and method declarations
        [cap5]: cap6,
        [cap7]() {
            cap8
        }
    }
}`;
        cases.push({
            title: "Doesn't serialize non-free variables (but retains frees)",
            func: eval(functext),
            expectText: `exports.handler = __f0;

function __f0(__0, __1) {
  return (function() {
    with({ cap1: 100, cap2: 200, cap3: 300, cap4: 400, cap5: 500, cap6: 600, cap7: 700, cap8: 800 }) {

return (nocap1, nocap2) => {
    let zz = nocap1 + nocap2; // not a capture: args
    let yy = nocap3; // not a capture: var later on
    if (zz) {
        zz += cap1; // true capture
        let cap1 = 9; // because let is properly scoped
        zz += nocap4; // not a capture
        var nocap4 = 7; // because var is function scoped
        zz += cap2; // true capture
        const cap2 = 33;
        var nocap3 = 8; // block the above capture
    }
    let f1 = (nocap5) => {
        yy += nocap5; // not a capture: args
        cap3++; // capture
    };
    let f2 = (function (nocap6) {
        zz += nocap6; // not a capture: args
        if (cap4) { // capture
            yy = 0;
        }
    });
    let www = nocap7(); // not a capture; it is defined below
    if (true) {
        function nocap7() {
        }
    }
    let [{t: [nocap8]},,nocap9 = "hello",...nocap10] = [{t: [true]},null,undefined,1,2];
    let vvv = [nocap8, nocap9, nocap10]; // not a capture; declarations from destructuring
    let aaa = { // captures in property and method declarations
        [cap5]: cap6,
        [cap7]() {
            cap8
        }
    }
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }
    {
        let nocap1 = 1;
        let cap1 = 100;

        cases.push({
            title: "Complex capturing cases #1",
            func: () => {
                // cap1 is captured here.
                // nocap1 introduces a new variable that shadows the outer one.
                let [nocap1 = cap1] = [];
                console.log(nocap1);
            },
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ cap1: 100 }) {

return () => {
                // cap1 is captured here.
                // nocap1 introduces a new variable that shadows the outer one.
                let [nocap1 = cap1] = [];
                console.log(nocap1);
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }
    {
        let nocap1 = 1;
        let cap1 = 100;

        cases.push({
            title: "Complex capturing cases #2",
            func: () => {
    // cap1 is captured here.
    // nocap1 introduces a new variable that shadows the outer one.
    let {nocap1 = cap1} = {};
    console.log(nocap1);
},
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ cap1: 100 }) {

return () => {
                // cap1 is captured here.
                // nocap1 introduces a new variable that shadows the outer one.
                let { nocap1 = cap1 } = {};
                console.log(nocap1);
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }
    {
        let nocap1 = 1;
        let cap1 = 100;

        cases.push({
            title: "Complex capturing cases #3",
            func: () => {
    // cap1 is captured here.
    // nocap1 introduces a new variable that shadows the outer one.
    let {x: nocap1 = cap1} = {};
    console.log(nocap1);
},
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ cap1: 100 }) {

return () => {
                // cap1 is captured here.
                // nocap1 introduces a new variable that shadows the outer one.
                let { x: nocap1 = cap1 } = {};
                console.log(nocap1);
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    cases.push({
        title: "Don't capture built-ins",
        func: () => { let x: any = eval("undefined + null + NaN + Infinity + __filename"); require("os"); },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return () => { let x = eval("undefined + null + NaN + Infinity + __filename"); require("os"); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    {
        const os = require("os");

        cases.push({
            title: "Capture built in module by ref",
            func: () => os,
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ os: require("os") }) {

return () => os;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const os = require("os");

        cases.push({
            title: "Wrapped lambda function",
            func: (a: any,
                   b: any,
                   c: any) => {
                       const v =
                           os;
                       return { v };
                   },
            expectText: `exports.handler = __f0;

function __f0(__0, __1, __2) {
  return (function() {
    with({ os: require("os") }) {

return (a, b, c) => {
                const v = os;
                return { v };
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const os = require("os");
        function wrap(handler: Function) {
            return () => handler;
        }

        const func = wrap(() => os);

        cases.push({
            title: "Capture module through indirect function references",
            func: func,
            expectText: `exports.handler = __f0;

function __f1() {
  return (function() {
    with({ os: require("os") }) {

return () => os;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ handler: __f1 }) {

return () => handler;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const util = require("../util");
        cases.push({
            title: "Capture user-defined module by value",
            func: () => util,
            expectText: `exports.handler = __f0;

var __util = {};
Object.defineProperty(__util, "__esModule", { value: true });
__util.asyncTest = __asyncTest;
__util.assertAsyncThrows = __assertAsyncThrows;

function __f1(__0, __1, __2, __3) {
  return (function() {
    with({  }) {

return function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __asyncTest(__0) {
  return (function() {
    with({ __awaiter: __f1, asyncTest: __asyncTest }) {

return function /*asyncTest*/(test) {
    return (done) => {
        const go = () => __awaiter(this, void 0, void 0, function* () {
            let caught;
            try {
                yield test();
            }
            catch (err) {
                caught = err;
            }
            finally {
                done(caught);
            }
        });
        go();
    };
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __assertAsyncThrows(__0) {
  return (function() {
    with({ __awaiter: __f1, assert: require("assert"), assertAsyncThrows: __assertAsyncThrows }) {

return function /*assertAsyncThrows*/(test) {
    return __awaiter(this, void 0, void 0, function* () {
        try {
            yield test();
        }
        catch (err) {
            return err.message;
        }
        assert(false, "Function was expected to throw, but didn't");
        return "";
    });
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ util: __util }) {

return () => util;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    cases.push({
        title: "Don't capture catch variables",
        func: () => { try { } catch (err) { console.log(err); } },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return () => { try { }
        catch (err) {
            console.log(err);
        } };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    {
        const defaultValue = 1;

        cases.push({
            title: "Capture default parameters",
            func: (arg: any = defaultValue) => {},
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ defaultValue: 1 }) {

return (arg = defaultValue) => { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    // Recursive function serialization.
    {
        const fff = "fff!";
        const ggg = "ggg!";
        const xcap = {
            fff: function () { console.log(fff); },
            ggg: () => { console.log(ggg); },
            zzz: {
                a: [ (a1: any, a2: any) => { console.log(a1 + a2); } ],
            },
        };
        const func = () => {
    xcap.fff();
    xcap.ggg();
    xcap.zzz.a[0]("x", "y");
};

        cases.push({
            title: "Serializes recursive function captures",
            func: func,
            expectText: `exports.handler = __f0;

var __xcap = {};
__xcap.fff = __f1;
__xcap.ggg = __f2;
var __xcap_zzz = {};
var __xcap_zzz_a = [__f3];
__xcap_zzz.a = __xcap_zzz_a;
__xcap.zzz = __xcap_zzz;

function __f1() {
  return (function() {
    with({ fff: "fff!" }) {

return function () { console.log(fff); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ ggg: "ggg!" }) {

return () => { console.log(ggg); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3(__0, __1) {
  return (function() {
    with({  }) {

return (a1, a2) => { console.log(a1 + a2); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ xcap: __xcap }) {

return () => {
            xcap.fff();
            xcap.ggg();
            xcap.zzz.a[0]("x", "y");
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class CapCap {
            constructor() {
                (<any>this).x = 42;
                (<any>this).f = () => { console.log((<any>this).x); };
            }
        }

        const cap: any = new CapCap();

        cases.push({
            title: "Serializes `this` capturing arrow functions",
            func: cap.f,
            expectText: undefined,
            error: `Error serializing function '<anonymous>': tsClosureCases.js(0,0)

function '<anonymous>': tsClosureCases.js(0,0): which could not be serialized because
  arrow function captured 'this'. Assign 'this' to another name outside function and capture that.

Function code:
  () => { console.log(this.x); }
`,
        });
    }

    cases.push({
        title: "Don't serialize `this` in function expressions",
        func: function() { return this; },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return function () { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    {
        const mutable: any = {};
        cases.push({
            title: "Serialize mutable objects by value at the time of capture (pre-mutation)",
            func: function() { return mutable; },
            expectText: `exports.handler = __f0;

var __mutable = {};

function __f0() {
  return (function() {
    with({ mutable: __mutable }) {

return function () { return mutable; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
            afters: [{
                pre: () => { mutable.timesTheyAreAChangin = true; },
                title: "Serialize mutable objects by value at the time of capture (post-mutation)",
                func: function() { return mutable; },
                expectText: `exports.handler = __f0;

var __mutable = {timesTheyAreAChangin: true};

function __f0() {
  return (function() {
    with({ mutable: __mutable }) {

return function () { return mutable; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
            }],
        });
    }

    {
        const v = { d: output(4) };
        cases.push({
            title: "Output capture",
            func: function () { console.log(v); },
            expectText: `exports.handler = __f0;

var __v = {};
var __v_d_proto = {};
Object.defineProperty(__f1, "prototype", { value: __v_d_proto });
Object.defineProperty(__v_d_proto, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__v_d_proto, "apply", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__v_d_proto, "get", { configurable: true, writable: true, value: __f3 });
var __v_d = Object.create(__v_d_proto);
__v_d.value = 4;
__v.d = __v_d;

function __f1(__0) {
  return (function() {
    with({  }) {

return function /*constructor*/(value) {
        this.value = value;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2(__0) {
  return (function() {
    with({  }) {

return function /*apply*/(func) {
        throw new Error("'apply' is not allowed from inside a cloud-callback. Use 'get' to retrieve the value of this Output directly.");
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({  }) {

return function /*get*/() {
        return this.value;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ v: __v }) {

return function () { console.log(v); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const v = {
            d1: output(4),
            d2: output("str"),
            d3: output(undefined),
            d4: output({ a: 1, b: true }),
        };
        cases.push({
            title: "Multiple output capture",
            func: function () { console.log(v); },
            expectText: `exports.handler = __f0;

var __v = {};
var __v_d1_proto = {};
Object.defineProperty(__f1, "prototype", { value: __v_d1_proto });
Object.defineProperty(__v_d1_proto, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__v_d1_proto, "apply", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__v_d1_proto, "get", { configurable: true, writable: true, value: __f3 });
var __v_d1 = Object.create(__v_d1_proto);
__v_d1.value = 4;
__v.d1 = __v_d1;
var __v_d2 = Object.create(__v_d1_proto);
__v_d2.value = "str";
__v.d2 = __v_d2;
var __v_d3 = Object.create(__v_d1_proto);
__v_d3.value = undefined;
__v.d3 = __v_d3;
var __v_d4 = Object.create(__v_d1_proto);
var __v_d4_value = {a: 1, b: true};
__v_d4.value = __v_d4_value;
__v.d4 = __v_d4;

function __f1(__0) {
  return (function() {
    with({  }) {

return function /*constructor*/(value) {
        this.value = value;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2(__0) {
  return (function() {
    with({  }) {

return function /*apply*/(func) {
        throw new Error("'apply' is not allowed from inside a cloud-callback. Use 'get' to retrieve the value of this Output directly.");
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({  }) {

return function /*get*/() {
        return this.value;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ v: __v }) {

return function () { console.log(v); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const v = {
            d1: output(4),
            d2: <any>undefined,
        };

        v.d2 = output({ a: 1, b: v });

        cases.push({
            title: "Recursive output capture",
            func: function () { console.log(v); },
            expectText: `exports.handler = __f0;

var __v = {};
var __v_d1_proto = {};
Object.defineProperty(__f1, "prototype", { value: __v_d1_proto });
Object.defineProperty(__v_d1_proto, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__v_d1_proto, "apply", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__v_d1_proto, "get", { configurable: true, writable: true, value: __f3 });
var __v_d1 = Object.create(__v_d1_proto);
__v_d1.value = 4;
__v.d1 = __v_d1;
var __v_d2 = Object.create(__v_d1_proto);
var __v_d2_value = {};
__v_d2_value.a = 1;
var __v_d2_value_b = {d1: 4, d2: undefined};
__v_d2_value.b = __v_d2_value_b;
__v_d2.value = __v_d2_value;
__v.d2 = __v_d2;

function __f1(__0) {
  return (function() {
    with({  }) {

return function /*constructor*/(value) {
        this.value = value;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2(__0) {
  return (function() {
    with({  }) {

return function /*apply*/(func) {
        throw new Error("'apply' is not allowed from inside a cloud-callback. Use 'get' to retrieve the value of this Output directly.");
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({  }) {

return function /*get*/() {
        return this.value;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ v: __v }) {

return function () { console.log(v); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const x = { a: 1, b: true };

        const o1 = output(x);
        const o2 = output(x);

        const y = { o1, o2 };
        const o3 = output(y);
        const o4 = output(y);

        const o5: any = { o3, o4 };

        o5.a = output(y);
        o5.b = y;
        o5.c = [output(y)];
        o5.d = [y];

        o5.a_1 = o5.a;
        o5.b_1 = o5.b;
        o5.c_1 = o5.c;
        o5.d_1 = o5.d;

        const o6 = output(o5);

        const v = { x, o1, o2, y, o3, o4, o5, o6 };
        cases.push({
            title: "Capturing same value through outputs multiple times",
            func: function () { console.log(v); },
            expectText: `exports.handler = __f0;

var __v = {};
var __v_x = {a: 1, b: true};
__v.x = __v_x;
var __v_o1_proto = {};
Object.defineProperty(__f1, "prototype", { value: __v_o1_proto });
Object.defineProperty(__v_o1_proto, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__v_o1_proto, "apply", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__v_o1_proto, "get", { configurable: true, writable: true, value: __f3 });
var __v_o1 = Object.create(__v_o1_proto);
var __v_o1_value = {a: 1, b: true};
__v_o1.value = __v_o1_value;
__v.o1 = __v_o1;
var __v_o2 = Object.create(__v_o1_proto);
var __v_o2_value = {a: 1, b: true};
__v_o2.value = __v_o2_value;
__v.o2 = __v_o2;
var __v_y = {};
__v_y.o1 = __v_o1;
__v_y.o2 = __v_o2;
__v.y = __v_y;
var __v_o3 = Object.create(__v_o1_proto);
var __v_o3_value = {};
var __v_o3_value_o1 = {a: 1, b: true};
__v_o3_value.o1 = __v_o3_value_o1;
var __v_o3_value_o2 = {a: 1, b: true};
__v_o3_value.o2 = __v_o3_value_o2;
__v_o3.value = __v_o3_value;
__v.o3 = __v_o3;
var __v_o4 = Object.create(__v_o1_proto);
var __v_o4_value = {};
var __v_o4_value_o1 = {a: 1, b: true};
__v_o4_value.o1 = __v_o4_value_o1;
var __v_o4_value_o2 = {a: 1, b: true};
__v_o4_value.o2 = __v_o4_value_o2;
__v_o4.value = __v_o4_value;
__v.o4 = __v_o4;
var __v_o5 = {};
__v_o5.o3 = __v_o3;
__v_o5.o4 = __v_o4;
var __v_o5_a = Object.create(__v_o1_proto);
var __v_o5_a_value = {};
var __v_o5_a_value_o1 = {a: 1, b: true};
__v_o5_a_value.o1 = __v_o5_a_value_o1;
var __v_o5_a_value_o2 = {a: 1, b: true};
__v_o5_a_value.o2 = __v_o5_a_value_o2;
__v_o5_a.value = __v_o5_a_value;
__v_o5.a = __v_o5_a;
__v_o5.b = __v_y;
var __v_o5_c = [];
var __v_o5_c_0 = Object.create(__v_o1_proto);
var __v_o5_c_0_value = {};
var __v_o5_c_0_value_o1 = {a: 1, b: true};
__v_o5_c_0_value.o1 = __v_o5_c_0_value_o1;
var __v_o5_c_0_value_o2 = {a: 1, b: true};
__v_o5_c_0_value.o2 = __v_o5_c_0_value_o2;
__v_o5_c_0.value = __v_o5_c_0_value;
__v_o5_c[0] = __v_o5_c_0;
__v_o5.c = __v_o5_c;
var __v_o5_d = [];
__v_o5_d[0] = __v_y;
__v_o5.d = __v_o5_d;
__v_o5.a_1 = __v_o5_a;
__v_o5.b_1 = __v_y;
__v_o5.c_1 = __v_o5_c;
__v_o5.d_1 = __v_o5_d;
__v.o5 = __v_o5;
var __v_o6 = Object.create(__v_o1_proto);
var __v_o6_value = {};
var __v_o6_value_o3 = {};
var __v_o6_value_o3_o1 = {a: 1, b: true};
__v_o6_value_o3.o1 = __v_o6_value_o3_o1;
var __v_o6_value_o3_o2 = {a: 1, b: true};
__v_o6_value_o3.o2 = __v_o6_value_o3_o2;
__v_o6_value.o3 = __v_o6_value_o3;
var __v_o6_value_o4 = {};
var __v_o6_value_o4_o1 = {a: 1, b: true};
__v_o6_value_o4.o1 = __v_o6_value_o4_o1;
var __v_o6_value_o4_o2 = {a: 1, b: true};
__v_o6_value_o4.o2 = __v_o6_value_o4_o2;
__v_o6_value.o4 = __v_o6_value_o4;
var __v_o6_value_a = {};
var __v_o6_value_a_o1 = {a: 1, b: true};
__v_o6_value_a.o1 = __v_o6_value_a_o1;
var __v_o6_value_a_o2 = {a: 1, b: true};
__v_o6_value_a.o2 = __v_o6_value_a_o2;
__v_o6_value.a = __v_o6_value_a;
var __v_o6_value_b = {};
var __v_o6_value_b_o1 = {a: 1, b: true};
__v_o6_value_b.o1 = __v_o6_value_b_o1;
var __v_o6_value_b_o2 = {a: 1, b: true};
__v_o6_value_b.o2 = __v_o6_value_b_o2;
__v_o6_value.b = __v_o6_value_b;
var __v_o6_value_c = [];
var __v_o6_value_c_0 = {};
var __v_o6_value_c_0_o1 = {a: 1, b: true};
__v_o6_value_c_0.o1 = __v_o6_value_c_0_o1;
var __v_o6_value_c_0_o2 = {a: 1, b: true};
__v_o6_value_c_0.o2 = __v_o6_value_c_0_o2;
__v_o6_value_c[0] = __v_o6_value_c_0;
__v_o6_value.c = __v_o6_value_c;
var __v_o6_value_d = [];
var __v_o6_value_d_0 = {};
var __v_o6_value_d_0_o1 = {a: 1, b: true};
__v_o6_value_d_0.o1 = __v_o6_value_d_0_o1;
var __v_o6_value_d_0_o2 = {a: 1, b: true};
__v_o6_value_d_0.o2 = __v_o6_value_d_0_o2;
__v_o6_value_d[0] = __v_o6_value_d_0;
__v_o6_value.d = __v_o6_value_d;
var __v_o6_value_a_1 = {};
var __v_o6_value_a_1_o1 = {a: 1, b: true};
__v_o6_value_a_1.o1 = __v_o6_value_a_1_o1;
var __v_o6_value_a_1_o2 = {a: 1, b: true};
__v_o6_value_a_1.o2 = __v_o6_value_a_1_o2;
__v_o6_value.a_1 = __v_o6_value_a_1;
var __v_o6_value_b_1 = {};
var __v_o6_value_b_1_o1 = {a: 1, b: true};
__v_o6_value_b_1.o1 = __v_o6_value_b_1_o1;
var __v_o6_value_b_1_o2 = {a: 1, b: true};
__v_o6_value_b_1.o2 = __v_o6_value_b_1_o2;
__v_o6_value.b_1 = __v_o6_value_b_1;
var __v_o6_value_c_1 = [];
var __v_o6_value_c_1_0 = {};
var __v_o6_value_c_1_0_o1 = {a: 1, b: true};
__v_o6_value_c_1_0.o1 = __v_o6_value_c_1_0_o1;
var __v_o6_value_c_1_0_o2 = {a: 1, b: true};
__v_o6_value_c_1_0.o2 = __v_o6_value_c_1_0_o2;
__v_o6_value_c_1[0] = __v_o6_value_c_1_0;
__v_o6_value.c_1 = __v_o6_value_c_1;
var __v_o6_value_d_1 = [];
var __v_o6_value_d_1_0 = {};
var __v_o6_value_d_1_0_o1 = {a: 1, b: true};
__v_o6_value_d_1_0.o1 = __v_o6_value_d_1_0_o1;
var __v_o6_value_d_1_0_o2 = {a: 1, b: true};
__v_o6_value_d_1_0.o2 = __v_o6_value_d_1_0_o2;
__v_o6_value_d_1[0] = __v_o6_value_d_1_0;
__v_o6_value.d_1 = __v_o6_value_d_1;
__v_o6.value = __v_o6_value;
__v.o6 = __v_o6;

function __f1(__0) {
  return (function() {
    with({  }) {

return function /*constructor*/(value) {
        this.value = value;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2(__0) {
  return (function() {
    with({  }) {

return function /*apply*/(func) {
        throw new Error("'apply' is not allowed from inside a cloud-callback. Use 'get' to retrieve the value of this Output directly.");
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({  }) {

return function /*get*/() {
        return this.value;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ v: __v }) {

return function () { console.log(v); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const obj = { method1() { return this.method2(); }, method2: () => { return; } };

        cases.push({
            title: "Capture object with methods",
            func: function () { console.log(obj); },
            expectText: `exports.handler = __f0;

var __obj = {method1: __f1, method2: __f2};

function __f1() {
  return (function() {
    with({  }) {

return function /*method1*/() { return this.method2(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({  }) {

return () => { return; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ obj: __obj }) {

return function () { console.log(obj); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        cases.push({
            title: "Undeclared variable in typeof",
            // @ts-ignore
            func: function () { const x = typeof a; },
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ a: undefined }) {

return function () { const x = typeof a; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const a = 0;
        cases.push({
            title: "Declared variable in typeof",
            // @ts-ignore
            func: function () { const x = typeof a; },
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ a: 0 }) {

return function () { const x = typeof a; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const array: any[] = [];
        const obj = { 80: "foo", arr: array };
        array.push(obj);

        cases.push({
            title: "Capture numeric property",
            func: function () { return array; },
            expectText: `exports.handler = __f0;

var __array = [];
var __array_0 = {};
__array_0["80"] = "foo";
__array_0.arr = __array;
__array[0] = __array_0;

function __f0() {
  return (function() {
    with({ array: __array }) {

return function () { return array; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const outer: any = { o: 1 };
        const array = [outer];
        outer.b = array;
        const C = (function () {
            function C() {}
            C.prototype.m = function () { return this.n() + outer; };
            C.prototype.n = function () { return array; };
            (<any>C).m = function () { return this.n(); };
            return C;
        }());

        cases.push({
            title: "Serialize es5-style class",
            func: () => C,
            expectText: `exports.handler = __f0;

var __C_prototype = {};
Object.defineProperty(__C_prototype, "constructor", { configurable: true, writable: true, value: __C });
var __outer = {};
__outer.o = 1;
var __outer_b = [];
__outer_b[0] = __outer;
__outer.b = __outer_b;
__C_prototype.m = __f1;
__C_prototype.n = __f2;
Object.defineProperty(__C, "prototype", { writable: true, value: __C_prototype });
__C.m = __f3;

function __C() {
  return (function() {
    with({ C: __C }) {

return function /*C*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({ outer: __outer }) {

return function () { return this.n() + outer; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ array: __outer_b }) {

return function () { return array; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({  }) {

return function () { return this.n(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __C }) {

return () => C;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const outer: any = { o: 1 };
        const array = [outer];
        outer.b = array;
        class C {
            public static s() { return array; }
            public m() { return this.n(); }
            public n() { return outer; }
        }
        cases.push({
            title: "Serialize class",
            func: () => C,
            expectText: `exports.handler = __f0;

var __f1_prototype = {};
Object.defineProperty(__f1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__f1_prototype, "m", { configurable: true, writable: true, value: __f2 });
var __outer = {};
__outer.o = 1;
var __outer_b = [];
__outer_b[0] = __outer;
__outer.b = __outer_b;
Object.defineProperty(__f1_prototype, "n", { configurable: true, writable: true, value: __f3 });
Object.defineProperty(__f1, "prototype", { value: __f1_prototype });
Object.defineProperty(__f1, "s", { configurable: true, writable: true, value: __f4 });

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({  }) {

return function /*m*/() { return this.n(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({ outer: __outer }) {

return function /*n*/() { return outer; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4() {
  return (function() {
    with({ array: __outer_b }) {

return function /*s*/() { return array; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __f1 }) {

return () => C;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            private x: number;
            public static s() { return 0; }
            constructor() {
                this.x = 1;
            }
            public m() { return this.n(); }
            public n() { return 1; }
        }
        cases.push({
            title: "Serialize class with constructor and field",
            func: () => C,
            expectText: `exports.handler = __f0;

var __f1_prototype = {};
Object.defineProperty(__f1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__f1_prototype, "m", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__f1_prototype, "n", { configurable: true, writable: true, value: __f3 });
Object.defineProperty(__f1, "prototype", { value: __f1_prototype });
Object.defineProperty(__f1, "s", { configurable: true, writable: true, value: __f4 });

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() {
                this.x = 1;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({  }) {

return function /*m*/() { return this.n(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({  }) {

return function /*n*/() { return 1; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4() {
  return (function() {
    with({  }) {

return function /*s*/() { return 0; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __f1 }) {

return () => C;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            private x: number;
            public static s() { return 0; }
            constructor() {
                this.x = 1;
            }
            public m() { return this.n(); }
            public n() { return 1; }
        }
        cases.push({
            title: "Serialize constructed class",
            func: () => new C(),
            expectText: `exports.handler = __f0;

var __f1_prototype = {};
Object.defineProperty(__f1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__f1_prototype, "m", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__f1_prototype, "n", { configurable: true, writable: true, value: __f3 });
Object.defineProperty(__f1, "prototype", { value: __f1_prototype });
Object.defineProperty(__f1, "s", { configurable: true, writable: true, value: __f4 });

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() {
                this.x = 1;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({  }) {

return function /*m*/() { return this.n(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({  }) {

return function /*n*/() { return 1; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4() {
  return (function() {
    with({  }) {

return function /*s*/() { return 0; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __f1 }) {

return () => new C();

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            public m() { return this.n(); }
            public n() { return 0; }
        }
        cases.push({
            title: "Serialize instance class methods",
            func: new C().m,
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return function /*m*/() { return this.n(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            public delete() { return 0; }
        }
        cases.push({
            title: "Serialize method with reserved name",
            func: new C().delete,
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return function /*delete*/() { return 0; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }


    {
        class C {
            public static m() { return this.n(); }
            public static n() { return 0; }
        }
        cases.push({
            title: "Serialize static class methods",
            func: C.m,
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return function /*m*/() { return this.n(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const D = (function () {
            function D() {
                ;
            }
            (<any>D).m = function () { return this.n(); };
            (<any>D).n = function () { return 0; };
            return D;
        }());
        cases.push({
            title: "Serialize static class methods (es5 class style)",
            func: (<any>D).m,
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return function () { return this.n(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const array: any[] = [1];
        array.push(array);

        cases.push({
            title: "Cyclic object #1",
            func: () => array,
            expectText: `exports.handler = __f0;

var __array = [];
__array[0] = 1;
__array[1] = __array;

function __f0() {
  return (function() {
    with({ array: __array }) {

return () => array;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const obj: any = {a: 1};
        obj.b = obj;

        cases.push({
            title: "Cyclic object #2",
            func: () => obj,
            expectText: `exports.handler = __f0;

var __obj = {};
__obj.a = 1;
__obj.b = __obj;

function __f0() {
  return (function() {
    with({ obj: __obj }) {

return () => obj;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const obj: any = {a: []};
        obj.a.push(obj);
        obj.b = obj.a;

        cases.push({
            title: "Cyclic object #3",
            func: () => obj,
            expectText: `exports.handler = __f0;

var __obj = {};
var __obj_a = [];
__obj_a[0] = __obj;
__obj.a = __obj_a;
__obj.b = __obj_a;

function __f0() {
  return (function() {
    with({ obj: __obj }) {

return () => obj;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const obj: any = {a: []};
        obj.a.push(obj);
        obj.b = obj.a;
        const obj2 = [obj, obj];

        cases.push({
            title: "Cyclic object #4",
            func: () => obj2,
            expectText: `exports.handler = __f0;

var __obj2 = [];
var __obj2_0 = {};
var __obj2_0_a = [];
__obj2_0_a[0] = __obj2_0;
__obj2_0.a = __obj2_0_a;
__obj2_0.b = __obj2_0_a;
__obj2[0] = __obj2_0;
__obj2[1] = __obj2_0;

function __f0() {
  return (function() {
    with({ obj2: __obj2 }) {

return () => obj2;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const obj: any = {a: 1};

        function f1() { return obj; }
        function f2() { console.log(obj); }

        cases.push({
            title: "Object captured across multiple functions",
            func: () => { f1(); obj.a = 2; f2(); },
            expectText: `exports.handler = __f0;

var __obj = {a: 1};

function __f1() {
  return (function() {
    with({ obj: __obj, f1: __f1 }) {

return function /*f1*/() { return obj; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ obj: __obj, f2: __f2 }) {

return function /*f2*/() { console.log(obj); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ f1: __f1, obj: __obj, f2: __f2 }) {

return () => { f1(); obj.a = 2; f2(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const v = {};
        Object.defineProperty(v, "key", {
            configurable: true,
            value: 1,
        });
        cases.push({
            title: "Complex property descriptor #1",
            func: () => v,
            expectText: `exports.handler = __f0;

var __v = {};
Object.defineProperty(__v, "key", { configurable: true, value: 1 });

function __f0() {
  return (function() {
    with({ v: __v }) {

return () => v;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const v = {};
        Object.defineProperty(v, "key", {
            writable: true,
            enumerable: true,
            value: 1,
        });
        cases.push({
            title: "Complex property descriptor #2",
            func: () => v,
            expectText: `exports.handler = __f0;

var __v = {};
Object.defineProperty(__v, "key", { enumerable: true, writable: true, value: 1 });

function __f0() {
  return (function() {
    with({ v: __v }) {

return () => v;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const v = [1, 2, 3];
        delete v[1];

        cases.push({
            title: "Test array #1",
            func: () => v,
            expectText: `exports.handler = __f0;

var __v = [];
__v[0] = 1;
__v[2] = 3;

function __f0() {
  return (function() {
    with({ v: __v }) {

return () => v;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const v = [1, 2, 3];
        delete v[1];
        (<any>v).foo = "";

        cases.push({
            title: "Test array #2",
            func: () => v,
            expectText: `exports.handler = __f0;

var __v = [];
__v[0] = 1;
__v[2] = 3;
__v.foo = "";

function __f0() {
  return (function() {
    with({ v: __v }) {

return () => v;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const v = () => { return 1; };
        (<any>v).foo = "bar";

        cases.push({
            title: "Test function with property",
            func: v,
            expectText: `exports.handler = __f0;

__f0.foo = "bar";

function __f0() {
  return (function() {
    with({  }) {

return () => { return 1; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const x = Object.create(null);
        const v = () => { return x; };

        cases.push({
            title: "Test null prototype",
            func: v,
            expectText: `exports.handler = __f0;

var __x = Object.create(null);

function __f0() {
  return (function() {
    with({ x: __x }) {

return () => { return x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const x = Object.create(Number.prototype);
        const v = () => { return x; };

        cases.push({
            title: "Test non-default object prototype",
            func: v,
            expectText: `exports.handler = __f0;

var __x = Object.create(global.Number.prototype);

function __f0() {
  return (function() {
    with({ x: __x }) {

return () => { return x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const x = Object.create({ x() { return v; } });
        const v = () => { return x; };

        cases.push({
            title: "Test recursive prototype object prototype",
            func: v,
            expectText: `exports.handler = __f0;

var __x_proto = {x: __f1};
var __x = Object.create(__x_proto);

function __f1() {
  return (function() {
    with({ v: __f0 }) {

return function /*x*/() { return v; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ x: __x }) {

return () => { return x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const v = () => { return 0; };
        Object.setPrototypeOf(v, () => { return 1; });

        cases.push({
            title: "Test non-default function prototype",
            func: v,
            expectText: `exports.handler = __f0;

Object.setPrototypeOf(__f0, __f1);

function __f0() {
  return (function() {
    with({  }) {

return () => { return 0; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({  }) {

return () => { return 1; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        function *f() { yield 1; }

        cases.push({
            title: "Test generator func",
            func: f,
            expectText: `exports.handler = __f;

var __f_prototype = Object.create(Object.getPrototypeOf((function*(){}).prototype));
Object.defineProperty(__f, "prototype", { writable: true, value: __f_prototype });
Object.setPrototypeOf(__f, Object.getPrototypeOf(function*(){}));

function __f() {
  return (function() {
    with({ f: __f }) {

return function* /*f*/() { yield 1; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const gf = (function *() { yield 1; });

        cases.push({
            title: "Test anonymous generator func",
            func: gf,
            expectText: `exports.handler = __f0;

var __f0_prototype = Object.create(Object.getPrototypeOf((function*(){}).prototype));
Object.defineProperty(__f0, "prototype", { writable: true, value: __f0_prototype });
Object.setPrototypeOf(__f0, Object.getPrototypeOf(function*(){}));

function __f0() {
  return (function() {
    with({  }) {

return function* () { yield 1; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            private _x: number;

            constructor() {
                this._x = 0;
            }

            get foo() {
                return this._x;
            }

            set foo(v: number) {
                this._x = v;
            }
        }

        cases.push({
            title: "Test getter/setter #1",
            func: () => C,
            expectText: `exports.handler = __f0;

var __f1_prototype = {};
Object.defineProperty(__f1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__f1_prototype, "foo", { configurable: true, get: __f2, set: __f3 });
Object.defineProperty(__f1, "prototype", { value: __f1_prototype });

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() {
                this._x = 0;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({  }) {

return function /*foo*/() {
                return this._x;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3(__0) {
  return (function() {
    with({  }) {

return function /*foo*/(v) {
                this._x = v;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __f1 }) {

return () => C;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            static get foo() {
                throw new Error("This getter function should not be evaluated while closure serialization.")
            }

            static set foo(v: number) {
                throw new Error("This setter function should not be evaluated while closure serialization.")
            }
        }

        cases.push({
            title: "Test getter/setter #2",
            func: () => C,
            expectText: `exports.handler = __f0;

Object.defineProperty(__f1, "foo", { configurable: true, get: __f2, set: __f3 });

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({  }) {

return function /*foo*/() {
                throw new Error("This getter function should not be evaluated while closure serialization.");
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3(__0) {
  return (function() {
    with({  }) {

return function /*foo*/(v) {
                throw new Error("This setter function should not be evaluated while closure serialization.");
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __f1 }) {

return () => C;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const methodName = "method name";
        class C {
            [methodName](a: number) {
                return a;
            }
        }

        cases.push({
            title: "Test computed method name.",
            func: () => C,
            expectText: `exports.handler = __f0;

var __f1_prototype = {};
Object.defineProperty(__f1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__f1_prototype, "method name", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__f1, "prototype", { value: __f1_prototype });

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2(__0) {
  return (function() {
    with({  }) {

return function (a) {
                return a;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __f1 }) {

return () => C;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const sym = Symbol("test_symbol");
        class C {
            [sym](a: number) {
                return a;
            }

            getSym() { return sym; }
        }

        cases.push({
            title: "Test symbols #1",
            func: () => C,
            expectText: `exports.handler = __f0;

var __f1_prototype = {};
Object.defineProperty(__f1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
var __sym = Object.create(global.Symbol.prototype);
Object.defineProperty(__f1_prototype, "getSym", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__f1_prototype, __sym, { configurable: true, writable: true, value: __f3 });
Object.defineProperty(__f1, "prototype", { value: __f1_prototype });

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ sym: __sym }) {

return function /*getSym*/() { return sym; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3(__0) {
  return (function() {
    with({  }) {

return function (a) {
                return a;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __f1 }) {

return () => C;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            [Symbol.iterator](a: number) {
                return a;
            }
        }

        cases.push({
            title: "Test Symbol.iterator",
            func: () => C,
            expectText: `exports.handler = __f0;

var __f1_prototype = {};
Object.defineProperty(__f1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__f1_prototype, Symbol.iterator, { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__f1, "prototype", { value: __f1_prototype });

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2(__0) {
  return (function() {
    with({  }) {

return function (a) {
                return a;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __f1 }) {

return () => C;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class D {
            public n: number;
            constructor(n: number) {
                this.n = n;
                console.log("DConstructor");
            }
            dMethod(x: number) { return x; }
            dVirtual() { return 1; }
        }
        class C extends D {
            constructor(n: number) {
                super(n + 1);
                console.log("CConstructor");
            }
            cMethod() {
                return "" +
                    super.dMethod + super["dMethod"] +
                    super.dMethod(1) + super["dMethod"](2) +
                    super.dMethod(super.dMethod(3));
                }
            dVirtual() { return 3; }
        }

        cases.push({
            title: "Test class extension",
            func: () => C,
            expectText: `exports.handler = __f0;

var __f2_prototype = {};
Object.defineProperty(__f2_prototype, "constructor", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__f2_prototype, "dMethod", { configurable: true, writable: true, value: __f3 });
Object.defineProperty(__f2_prototype, "dVirtual", { configurable: true, writable: true, value: __f4 });
Object.defineProperty(__f2, "prototype", { value: __f2_prototype });
var __f1_prototype = Object.create(__f2_prototype);
Object.defineProperty(__f1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__f1_prototype, "cMethod", { configurable: true, writable: true, value: __f5 });
Object.defineProperty(__f1_prototype, "dVirtual", { configurable: true, writable: true, value: __f6 });
Object.defineProperty(__f1, "prototype", { value: __f1_prototype });
Object.setPrototypeOf(__f1, __f2);

function __f2(__0) {
  return (function() {
    with({  }) {

return function /*constructor*/(n) {
                this.n = n;
                console.log("DConstructor");
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3(__0) {
  return (function() {
    with({  }) {

return function /*dMethod*/(x) { return x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4() {
  return (function() {
    with({  }) {

return function /*dVirtual*/() { return 1; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1(__0) {
  return (function() {
    with({ __super: __f2 }) {

return function /*constructor*/(n) {
    __super.call(this, n + 1);
    console.log("CConstructor");
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f5() {
  return (function() {
    with({ __super: __f2 }) {

return function /*cMethod*/() {
    return "" +
        __super.prototype.dMethod + __super.prototype["dMethod"] +
        __super.prototype.dMethod.call(this, 1) + __super.prototype["dMethod"].call(this, 2) +
        __super.prototype.dMethod.call(this, __super.prototype.dMethod.call(this, 3));
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f6() {
  return (function() {
    with({ __super: __f2 }) {

return function /*dVirtual*/() { return 3; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __f1 }) {

return () => C;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class A {
            public n: number;
            constructor(n: number) {
                this.n = n;
                console.log("AConstruction");
            }
            method(x: number) { return x; }
        }
        class B extends A {
            constructor(n: number) {
                super(n + 1);
                console.log("BConstructor");
            }
            method(n: number) { return 1 + super.method(n + 1); }
        }
        class C extends B {
            constructor(n: number) {
                super(n * 2);
                console.log("CConstructor");
            }
            method(n: number) { return 2 * super.method(n * 2); }
        }

        cases.push({
            title: "Three level inheritance",
            func: () => C,
            expectText: `exports.handler = __f0;

var __f3_prototype = {};
Object.defineProperty(__f3_prototype, "constructor", { configurable: true, writable: true, value: __f3 });
Object.defineProperty(__f3_prototype, "method", { configurable: true, writable: true, value: __f4 });
Object.defineProperty(__f3, "prototype", { value: __f3_prototype });
var __f2_prototype = Object.create(__f3_prototype);
Object.defineProperty(__f2_prototype, "constructor", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__f2_prototype, "method", { configurable: true, writable: true, value: __f5 });
Object.defineProperty(__f2, "prototype", { value: __f2_prototype });
Object.setPrototypeOf(__f2, __f3);
var __f1_prototype = Object.create(__f2_prototype);
Object.defineProperty(__f1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__f1_prototype, "method", { configurable: true, writable: true, value: __f6 });
Object.defineProperty(__f1, "prototype", { value: __f1_prototype });
Object.setPrototypeOf(__f1, __f2);

function __f3(__0) {
  return (function() {
    with({  }) {

return function /*constructor*/(n) {
                this.n = n;
                console.log("AConstruction");
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4(__0) {
  return (function() {
    with({  }) {

return function /*method*/(x) { return x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2(__0) {
  return (function() {
    with({ __super: __f3 }) {

return function /*constructor*/(n) {
    __super.call(this, n + 1);
    console.log("BConstructor");
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f5(__0) {
  return (function() {
    with({ __super: __f3 }) {

return function /*method*/(n) { return 1 + __super.prototype.method.call(this, n + 1); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1(__0) {
  return (function() {
    with({ __super: __f2 }) {

return function /*constructor*/(n) {
    __super.call(this, n * 2);
    console.log("CConstructor");
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f6(__0) {
  return (function() {
    with({ __super: __f2 }) {

return function /*method*/(n) { return 2 * __super.prototype.method.call(this, n * 2); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __f1 }) {

return () => C;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`});
    }

    {
        const sym = Symbol();

        class A {
            public n: number;
            constructor(n: number) {
                this.n = n;
                console.log("AConstruction");
            }
            public [sym](x: number) { return x; }
        }
        class B extends A {
            constructor(n: number) {
                super(n + 1);
                console.log("BConstructor");
            }
            // @ts-ignore
            public [sym](n: number) { return 1 + super[sym](n + 1); }
        }
        class C extends B {
            constructor(n: number) {
                super(n * 2);
                console.log("CConstructor");
            }
            // @ts-ignore
            public [sym](n: number) { return 2 * super[sym](n * 2); }
        }

        cases.push({
            title: "Three level inheritance with symbols",
            func: () => C,
            expectText: `exports.handler = __f0;

var __f3_prototype = {};
Object.defineProperty(__f3_prototype, "constructor", { configurable: true, writable: true, value: __f3 });
var __f3_prototype_sym = Object.create(global.Symbol.prototype);
Object.defineProperty(__f3_prototype, __f3_prototype_sym, { configurable: true, writable: true, value: __f4 });
Object.defineProperty(__f3, "prototype", { value: __f3_prototype });
var __f2_prototype = Object.create(__f3_prototype);
Object.defineProperty(__f2_prototype, "constructor", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__f2_prototype, __f3_prototype_sym, { configurable: true, writable: true, value: __f5 });
Object.defineProperty(__f2, "prototype", { value: __f2_prototype });
Object.setPrototypeOf(__f2, __f3);
var __f1_prototype = Object.create(__f2_prototype);
Object.defineProperty(__f1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__f1_prototype, __f3_prototype_sym, { configurable: true, writable: true, value: __f6 });
Object.defineProperty(__f1, "prototype", { value: __f1_prototype });
Object.setPrototypeOf(__f1, __f2);

function __f3(__0) {
  return (function() {
    with({  }) {

return function /*constructor*/(n) {
                this.n = n;
                console.log("AConstruction");
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4(__0) {
  return (function() {
    with({  }) {

return function (x) { return x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2(__0) {
  return (function() {
    with({ __super: __f3 }) {

return function /*constructor*/(n) {
    __super.call(this, n + 1);
    console.log("BConstructor");
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f5(__0) {
  return (function() {
    with({ sym: __f3_prototype_sym, __super: __f3 }) {

return function /*__computed*/(n) { return 1 + __super.prototype[sym].call(this, n + 1); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1(__0) {
  return (function() {
    with({ __super: __f2 }) {

return function /*constructor*/(n) {
    __super.call(this, n * 2);
    console.log("CConstructor");
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f6(__0) {
  return (function() {
    with({ sym: __f3_prototype_sym, __super: __f2 }) {

return function /*__computed*/(n) { return 2 * __super.prototype[sym].call(this, n * 2); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ C: __f1 }) {

return () => C;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`});
    }

    {
        const sym = Symbol();

        class A {
            public n: number;
            static method(x: number) { return x; }
            static [sym](x: number) { return x * x; }
            constructor(n: number) {
                this.n = n;
                console.log("AConstruction");
            }
        }
        class B extends A {
            static method(n: number) { return 1 + super.method(n + 1); }
            // @ts-ignore
            static [sym](x: number) { return x * super[sym](x + 1); }
            constructor(n: number) {
                super(n + 1);
                console.log("BConstructor");
            }
        }

        cases.push({
            title: "Two level static inheritance",
            func: () => B,
            expectText: `exports.handler = __f0;

Object.defineProperty(__f2, "method", { configurable: true, writable: true, value: __f3 });
var __f2_sym = Object.create(global.Symbol.prototype);
Object.defineProperty(__f2, __f2_sym, { configurable: true, writable: true, value: __f4 });
Object.defineProperty(__f1, "method", { configurable: true, writable: true, value: __f5 });
Object.defineProperty(__f1, __f2_sym, { configurable: true, writable: true, value: __f6 });
Object.setPrototypeOf(__f1, __f2);

function __f2(__0) {
  return (function() {
    with({  }) {

return function /*constructor*/(n) {
                this.n = n;
                console.log("AConstruction");
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3(__0) {
  return (function() {
    with({  }) {

return function /*method*/(x) { return x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4(__0) {
  return (function() {
    with({  }) {

return function (x) { return x * x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1(__0) {
  return (function() {
    with({ __super: __f2 }) {

return function /*constructor*/(n) {
    __super.call(this, n + 1);
    console.log("BConstructor");
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f5(__0) {
  return (function() {
    with({ __super: __f2 }) {

return function /*method*/(n) { return 1 + __super.method.call(this, n + 1); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f6(__0) {
  return (function() {
    with({ sym: __f2_sym, __super: __f2 }) {

return function /*__computed*/(x) { return x * __super[sym].call(this, x + 1); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ B: __f1 }) {

return () => B;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`});
    }

    {
        const o = { a: 1, b: 2 };

        cases.push({
            title: "Capture subset of properties #1",
            func: function () { console.log(o.a); },
            expectText: `exports.handler = __f0;

var __o = {a: 1};

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.a); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: 2 };

        cases.push({
            title: "Capture subset of properties #1.1",
            func: function () { console.log(o["a"]); },
            expectText: `exports.handler = __f0;

var __o = {a: 1};

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o["a"]); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: 2, c: 3 };

        cases.push({
            title: "Capture subset of properties #2",
            func: function () { console.log(o.b + o.c); },
            expectText: `exports.handler = __f0;

var __o = {b: 2, c: 3};

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b + o.c); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: 2, c: 3 };

        cases.push({
            title: "Capture subset of properties #2.1",
            func: function () { console.log(o["b"] + o["c"]); },
            expectText: `exports.handler = __f0;

var __o = {b: 2, c: 3};

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o["b"] + o["c"]); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: 2, c: 3 };

        cases.push({
            title: "Capture all if object is used as is.",
            func: function () { console.log(o); },
            expectText: `exports.handler = __f0;

var __o = {a: 1, b: 2, c: 3};

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: 2, c() { return this; } };

        cases.push({
            title: "Capture all if object property is invoked, and it uses this. #1",
            func: function () { console.log(o.c()); },
            expectText: `exports.handler = __f0;

var __o = {a: 1, b: 2, c: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.c()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: 2, c() { return this; } };

        cases.push({
            title: "Capture all if object property is invoked, and it uses this. #1.1",
            func: function () { console.log(o["c"]()); },
            expectText: `exports.handler = __f0;

var __o = {a: 1, b: 2, c: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o["c"]()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: 2, c() { const v = () => this; } };

        cases.push({
            title: "Capture all if object property is invoked, and it uses this in nested arrow function.",
            func: function () { console.log(o.c()); },
            expectText: `exports.handler = __f0;

var __o = {a: 1, b: 2, c: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { const v = () => this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.c()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
      // @ts-ignore: this is just test code.
        const o = { a: 1, b: 2, c() { const v = function () { return this; }; } };

        cases.push({
            title: "Capture one if object property is invoked, but it uses this in nested function.",
            func: function () { console.log(o.c()); },
            expectText: `exports.handler = __f0;

var __o = {c: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { const v = function () { return this; }; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.c()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: 2, c() { return this; } };

        cases.push({
            title: "Capture one if object property is captured, uses this, but is not invoked. #1",
            func: function () { console.log(o.c); },
            expectText: `exports.handler = __f0;

var __o = {c: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.c); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: 2, c() { return this; } };

        cases.push({
            title: "Capture one if object property is captured, uses this, but is not invoked. #1.1",
            func: function () { console.log(o["c"]); },
            expectText: `exports.handler = __f0;

var __o = {c: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o["c"]); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: 2, c() { return 0; } };

        cases.push({
            title: "Capture one if object property is invoked, and it does not use this. #1",
            func: function () { console.log(o.c()); },
            expectText: `exports.handler = __f0;

var __o = {c: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { return 0; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.c()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: 2, c() { return 0; } };

        cases.push({
            title: "Capture one if object property is invoked, and it does not use this. #1.1",
            func: function () { console.log(o["c"]()); },
            expectText: `exports.handler = __f0;

var __o = {c: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { return 0; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o["c"]()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: { c() { return this; } } };

        cases.push({
            title: "Capture subset if sub object property is invoked. #1",
            func: function () { console.log(o.b.c()); },
            expectText: `exports.handler = __f0;

var __o = {};
var __o_b = {c: __f1};
__o.b = __o_b;

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b.c()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, b: { c() { return this; } } };

        cases.push({
            title: "Capture subset if sub object property is invoked. #1.1",
            func: function () { console.log(o["b"]["c"]()); },
            expectText: `exports.handler = __f0;

var __o = {};
var __o_b = {c: __f1};
__o.b = __o_b;

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o["b"]["c"]()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, get b() { return this; } };

        cases.push({
            title: "Capture all if getter and getter uses this. #1",
            func: function () { console.log(o.b); },
            expectText: `exports.handler = __f0;

var __o = {};
__o.a = 1;
Object.defineProperty(__o, "b", { configurable: true, enumerable: true, get: __f1 });

function __f1() {
  return (function() {
    with({  }) {

return function /*b*/() { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, get b() { return this; } };

        cases.push({
            title: "Capture all if getter and getter uses this. #1.1",
            func: function () { console.log(o["b"]); },
            expectText: `exports.handler = __f0;

var __o = {};
__o.a = 1;
Object.defineProperty(__o, "b", { configurable: true, enumerable: true, get: __f1 });

function __f1() {
  return (function() {
    with({  }) {

return function /*b*/() { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o["b"]); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, get b() { return 0; } };

        cases.push({
            title: "Capture one if getter and getter does not use this. #1",
            func: function () { console.log(o.b); },
            expectText: `exports.handler = __f0;

var __o = {};
Object.defineProperty(__o, "b", { configurable: true, enumerable: true, get: __f1 });

function __f1() {
  return (function() {
    with({  }) {

return function /*b*/() { return 0; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, get b() { return 0; } };

        cases.push({
            title: "Capture one if getter and getter does not use this. #1.1",
            func: function () { console.log(o["b"]); },
            expectText: `exports.handler = __f0;

var __o = {};
Object.defineProperty(__o, "b", { configurable: true, enumerable: true, get: __f1 });

function __f1() {
  return (function() {
    with({  }) {

return function /*b*/() { return 0; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o["b"]); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, b: 1, c: 2 };
        function f1() {
            console.log(o.a);
            f2();
        }

        function f2() {
            console.log(o.c);
        }

        cases.push({
            title: "Capture multi props from different contexts #1",
            func: f1,
            expectText: `exports.handler = __f1;

var __o = {a: 1, c: 2};

function __f2() {
  return (function() {
    with({ o: __o, f2: __f2 }) {

return function /*f2*/() {
            console.log(o.c);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({ o: __o, f2: __f2, f1: __f1 }) {

return function /*f1*/() {
            console.log(o.a);
            f2();
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, b: 1, c: 2 };
        function f1() {
            console.log(o["a"]);
            f2();
        }

        function f2() {
            console.log(o["c"]);
        }

        cases.push({
            title: "Capture multi props from different contexts #1.1",
            func: f1,
            expectText: `exports.handler = __f1;

var __o = {a: 1, c: 2};

function __f2() {
  return (function() {
    with({ o: __o, f2: __f2 }) {

return function /*f2*/() {
            console.log(o["c"]);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({ o: __o, f2: __f2, f1: __f1 }) {

return function /*f1*/() {
            console.log(o["a"]);
            f2();
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1 };
        function f1() {
            // @ts-ignore
            console.log(o.c);
        }

        cases.push({
            title: "Do not capture non-existent prop #1",
            func: f1,
            expectText: `exports.handler = __f1;

var __o = {};

function __f1() {
  return (function() {
    with({ o: __o, f1: __f1 }) {

return function /*f1*/() {
            // @ts-ignore
            console.log(o.c);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1 };
        function f1() {
            // @ts-ignore
            console.log(o["c"]);
        }

        cases.push({
            title: "Do not capture non-existent prop #1.1",
            func: f1,
            expectText: `exports.handler = __f1;

var __o = {};

function __f1() {
  return (function() {
    with({ o: __o, f1: __f1 }) {

return function /*f1*/() {
            // @ts-ignore
            console.log(o["c"]);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, b: 1, c: 2 };
        function f1() {
            console.log(o.a);
            f2();
        }

        function f2() {
            console.log(o);
        }

        cases.push({
            title: "Capture all props from different contexts #1",
            func: f1,
            expectText: `exports.handler = __f1;

var __o = {a: 1, b: 1, c: 2};

function __f2() {
  return (function() {
    with({ o: __o, f2: __f2 }) {

return function /*f2*/() {
            console.log(o);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({ o: __o, f2: __f2, f1: __f1 }) {

return function /*f1*/() {
            console.log(o.a);
            f2();
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, b: 1, c: 2 };
        function f1() {
            console.log(o["a"]);
            f2();
        }

        function f2() {
            console.log(o);
        }

        cases.push({
            title: "Capture all props from different contexts #1.1",
            func: f1,
            expectText: `exports.handler = __f1;

var __o = {a: 1, b: 1, c: 2};

function __f2() {
  return (function() {
    with({ o: __o, f2: __f2 }) {

return function /*f2*/() {
            console.log(o);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({ o: __o, f2: __f2, f1: __f1 }) {

return function /*f1*/() {
            console.log(o["a"]);
            f2();
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, b: 1, c: 2 };
        function f1() {
            console.log(o);
            f2();
        }

        function f2() {
            console.log(o.a);
        }

        cases.push({
            title: "Capture all props from different contexts #2",
            func: f1,
            expectText: `exports.handler = __f1;

var __o = {a: 1, b: 1, c: 2};

function __f2() {
  return (function() {
    with({ o: __o, f2: __f2 }) {

return function /*f2*/() {
            console.log(o.a);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({ o: __o, f2: __f2, f1: __f1 }) {

return function /*f1*/() {
            console.log(o);
            f2();
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, b: 1, c: 2 };
        function f1() {
            console.log(o);
            f2();
        }

        function f2() {
            console.log(o["a"]);
        }

        cases.push({
            title: "Capture all props from different contexts #2.1",
            func: f1,
            expectText: `exports.handler = __f1;

var __o = {a: 1, b: 1, c: 2};

function __f2() {
  return (function() {
    with({ o: __o, f2: __f2 }) {

return function /*f2*/() {
            console.log(o["a"]);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({ o: __o, f2: __f2, f1: __f1 }) {

return function /*f1*/() {
            console.log(o);
            f2();
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        class C {
            a: number;
            b: number;

            constructor() {
                this.a = 1;
                this.b = 2;
            }

            m() { console.log(this); }
        }
        const o = new C();

        cases.push({
            title: "Capture all props if prototype is and uses this #1",
            func: function () { o.m(); },
            expectText: `exports.handler = __f0;

var __o_proto = {};
Object.defineProperty(__f1, "prototype", { value: __o_proto });
Object.defineProperty(__o_proto, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__o_proto, "m", { configurable: true, writable: true, value: __f2 });
var __o = Object.create(__o_proto);
__o.a = 1;
__o.b = 2;

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() {
                this.a = 1;
                this.b = 2;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({  }) {

return function /*m*/() { console.log(this); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { o.m(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            a: number;
            b: number;

            constructor() {
                this.a = 1;
                this.b = 2;
            }

            m() { console.log(this); }
        }
        const o = new C();

        cases.push({
            title: "Capture all props if prototype is and uses this #1.1",
            func: function () { o["m"](); },
            expectText: `exports.handler = __f0;

var __o_proto = {};
Object.defineProperty(__f1, "prototype", { value: __o_proto });
Object.defineProperty(__o_proto, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__o_proto, "m", { configurable: true, writable: true, value: __f2 });
var __o = Object.create(__o_proto);
__o.a = 1;
__o.b = 2;

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() {
                this.a = 1;
                this.b = 2;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({  }) {

return function /*m*/() { console.log(this); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { o["m"](); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            a: number;
            b: number;

            constructor() {
                this.a = 1;
                this.b = 2;
            }

            m() { }
        }
        const o = new C();

        cases.push({
            title: "Capture no props if prototype is used but does not use this #1",
            func: function () { o.m(); },
            expectText: `exports.handler = __f0;

var __o = {};
Object.defineProperty(__o, "m", { configurable: true, writable: true, value: __f1 });

function __f1() {
  return (function() {
    with({  }) {

return function /*m*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { o.m(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            a: number;
            b: number;

            constructor() {
                this.a = 1;
                this.b = 2;
            }

            m() { }
        }
        const o = new C();

        cases.push({
            title: "Capture no props if prototype is used but does not use this #1.1",
            func: function () { o["m"](); },
            expectText: `exports.handler = __f0;

var __o = {};
Object.defineProperty(__o, "m", { configurable: true, writable: true, value: __f1 });

function __f1() {
  return (function() {
    with({  }) {

return function /*m*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { o["m"](); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            a: number;

            constructor() {
                this.a = 1;
            }

            m() { (<any>this).n(); }
        }

        class D extends C {
            b: number;
            constructor() {
                super();
                this.b = 2;
            }
            n() {}
        }
        const o = new D();

        cases.push({
            title: "Capture all props if prototype is accessed #2",
            func: function () { o.m(); },
            expectText: `exports.handler = __f0;

var __o_proto_proto = {};
Object.defineProperty(__f1, "prototype", { value: __o_proto_proto });
Object.defineProperty(__o_proto_proto, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__o_proto_proto, "m", { configurable: true, writable: true, value: __f2 });
var __o_proto = Object.create(__o_proto_proto);
Object.defineProperty(__f3, "prototype", { value: __o_proto });
Object.setPrototypeOf(__f3, __f1);
Object.defineProperty(__o_proto, "constructor", { configurable: true, writable: true, value: __f3 });
Object.defineProperty(__o_proto, "n", { configurable: true, writable: true, value: __f4 });
var __o = Object.create(__o_proto);
__o.a = 1;
__o.b = 2;

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() {
                this.a = 1;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({  }) {

return function /*m*/() { this.n(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({ __super: __f1 }) {

return function /*constructor*/() {
    __super.call(this);
    this.b = 2;
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4() {
  return (function() {
    with({ __super: __f1 }) {

return function /*n*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { o.m(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        class C {
            a: number;

            constructor() {
                this.a = 1;
            }

            m() { (<any>this).n(); }
        }

        class D extends C {
            b: number;
            constructor() {
                super();
                this.b = 2;
            }
            n() {}
        }
        const o = new D();

        cases.push({
            title: "Capture all props if prototype is accessed #2.1",
            func: function () { o["m"](); },
            expectText: `exports.handler = __f0;

var __o_proto_proto = {};
Object.defineProperty(__f1, "prototype", { value: __o_proto_proto });
Object.defineProperty(__o_proto_proto, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__o_proto_proto, "m", { configurable: true, writable: true, value: __f2 });
var __o_proto = Object.create(__o_proto_proto);
Object.defineProperty(__f3, "prototype", { value: __o_proto });
Object.setPrototypeOf(__f3, __f1);
Object.defineProperty(__o_proto, "constructor", { configurable: true, writable: true, value: __f3 });
Object.defineProperty(__o_proto, "n", { configurable: true, writable: true, value: __f4 });
var __o = Object.create(__o_proto);
__o.a = 1;
__o.b = 2;

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() {
                this.a = 1;
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({  }) {

return function /*m*/() { this.n(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({ __super: __f1 }) {

return function /*constructor*/() {
    __super.call(this);
    this.b = 2;
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4() {
  return (function() {
    with({ __super: __f1 }) {

return function /*n*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { o["m"](); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const table1: any = { primaryKey: 1, insert: () => { }, scan: () => { } };

        async function testScanReturnsAllValues() {
            await table1.insert({[table1.primaryKey.get()]: "val1", value1: 1, value2: "1"});
            await table1.insert({[table1.primaryKey.get()]: "val2", value1: 2, value2: "2"});

            const values = null;
            // @ts-ignore
            const value1 = values.find(v => v[table1.primaryKey.get()] === "val1");
            // @ts-ignore
            const value2 = values.find(v => v[table1.primaryKey.get()] === "val2");
        }

        cases.push({
            title: "Cloud table function",
            func: testScanReturnsAllValues,
            expectText: `exports.handler = __testScanReturnsAllValues;

var __table1 = {insert: __f1, primaryKey: 1};

function __f0(__0, __1, __2, __3) {
  return (function() {
    with({  }) {

return function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({  }) {

return () => { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __testScanReturnsAllValues() {
  return (function() {
    with({ __awaiter: __f0, table1: __table1, testScanReturnsAllValues: __testScanReturnsAllValues }) {

return function /*testScanReturnsAllValues*/() {
            return __awaiter(this, void 0, void 0, function* () {
                yield table1.insert({ [table1.primaryKey.get()]: "val1", value1: 1, value2: "1" });
                yield table1.insert({ [table1.primaryKey.get()]: "val2", value1: 2, value2: "2" });
                const values = null;
                // @ts-ignore
                const value1 = values.find(v => v[table1.primaryKey.get()] === "val1");
                // @ts-ignore
                const value2 = values.find(v => v[table1.primaryKey.get()] === "val2");
            });
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, b: { x: 1, doNotCapture: true }, c: 2 };
        function f1() {
            console.log(o);
        }

        cases.push({
            title: "Do not capture #1",
            func: f1,
            expectText: `exports.handler = __f1;

var __o = {a: 1, b: undefined, c: 2};

function __f1() {
  return (function() {
    with({ o: __o, f1: __f1 }) {

return function /*f1*/() {
            console.log(o);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, b: () => console.log("the actual function") };
        (<any>o.b).doNotCapture = true;

        function f1() {
            console.log(o);
        }

        cases.push({
            title: "Do not capture #2",
            func: f1,
            expectText: `exports.handler = __f1;

var __o = {a: 1, b: __f0};

function __f0() {
  return (function() {
    with({ message: "Function 'b' cannot be called at runtime. It can only be used at deployment time.\\n\\nFunction code:\\n  () => console.log(\\"the actual function\\")\\n" }) {

return () => { throw new Error(message); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({ o: __o, f1: __f1 }) {

return function /*f1*/() {
            console.log(o);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const lambda1 = () => console.log(1);
        const lambda2 = () => console.log(1);

        function f3() {
            return (lambda1(), lambda2());
        }

        cases.push({
            title: "Merge simple functions",
            func: f3,
            expectText: `exports.handler = __f3;

function __f0() {
  return (function() {
    with({  }) {

return () => console.log(1);

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({ lambda1: __f0, lambda2: __f0, f3: __f3 }) {

return function /*f3*/() {
            return (lambda1(), lambda2());
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const awaiter1 = function (thisArg: any, _arguments: any, P: any, generator: any) {
            return new (P || (P = Promise))(function (resolve: any, reject: any) {
                function fulfilled(value: any) { try { step(generator.next(value)); } catch (e) { reject(e); } }
                function rejected(value: any) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
                function step(result: any) { result.done ? resolve(result.value) : new P(function (resolve1: any) { resolve1(result.value); }).then(fulfilled, rejected); }
                step((generator = generator.apply(thisArg, _arguments || [])).next());
            });
        };
        const awaiter2 = function (thisArg: any, _arguments: any, P: any, generator: any) {
            return new (P || (P = Promise))(function (resolve: any, reject: any) {
                function fulfilled(value: any) { try { step(generator.next(value)); } catch (e) { reject(e); } }
                function rejected(value: any) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
                function step(result: any) { result.done ? resolve(result.value) : new P(function (resolve1: any) { resolve1(result.value); }).then(fulfilled, rejected); }
                step((generator = generator.apply(thisArg, _arguments || [])).next());
            });
        };

        function f3() {
            const v1 = awaiter1, v2 = awaiter2;
        }

        cases.push({
            title: "Share __awaiter functions",
            func: f3,
            expectText: `exports.handler = __f3;

function __f0(__0, __1, __2, __3) {
  return (function() {
    with({  }) {

return function (thisArg, _arguments, P, generator) {
            return new (P || (P = Promise))(function (resolve, reject) {
                function fulfilled(value) { try {
                    step(generator.next(value));
                }
                catch (e) {
                    reject(e);
                } }
                function rejected(value) { try {
                    step(generator["throw"](value));
                }
                catch (e) {
                    reject(e);
                } }
                function step(result) { result.done ? resolve(result.value) : new P(function (resolve1) { resolve1(result.value); }).then(fulfilled, rejected); }
                step((generator = generator.apply(thisArg, _arguments || [])).next());
            });
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({ awaiter1: __f0, awaiter2: __f0, f3: __f3 }) {

return function /*f3*/() {
            const v1 = awaiter1, v2 = awaiter2;
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        cases.push({
            title: "Capture of exported variable #1",
            func: function () { console.log(exportedValue); },
            expectText: `exports.handler = __f0;

var __exports = {exportedValue: 42};

function __f0() {
  return (function() {
    with({ exports: __exports }) {

return function () { console.log(exports.exportedValue); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        cases.push({
            title: "Capture of exported variable #2",
            func: function () { console.log(exports.exportedValue); },
            expectText: `exports.handler = __f0;

var __exports = {exportedValue: 42};

function __f0() {
  return (function() {
    with({ exports: __exports }) {

return function () { console.log(exports.exportedValue); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        cases.push({
            title: "Capture of exported variable #3",
            func: function () { console.log(module.exports.exportedValue); },
            expectText: `exports.handler = __f0;

var __module = {};
var __module_exports = {exportedValue: 42};
__module.exports = __module_exports;

function __f0() {
  return (function() {
    with({ module: __module }) {

return function () { console.log(module.exports.exportedValue); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {

        function foo() {
            require("./util");
        }

        cases.push({
            title: "Required packages #1",
            func: function () { require("typescript"); foo(); if (true) { require("os") } },
            expectText: `exports.handler = __f0;

function __foo() {
  return (function() {
    with({ foo: __foo }) {

return function /*foo*/() {
            require("./util");
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ foo: __foo }) {

return function () { require("typescript"); foo(); if (true) {
                require("os");
            } };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: { c: 2, d: 3 } };

        cases.push({
            title: "Analyze property chain #1",
            func: function () { console.log(o.b.c); },
            expectText: `exports.handler = __f0;

var __o = {};
var __o_b = {c: 2};
__o.b = __o_b;

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b.c); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: { c: 2, d: 3 } };

        cases.push({
            title: "Analyze property chain #2",
            func: function () { console.log(o.b); console.log(o.b.c); },
            expectText: `exports.handler = __f0;

var __o = {};
var __o_b = {c: 2, d: 3};
__o.b = __o_b;

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b); console.log(o.b.c); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: { c: 2, d: 3 } };

        cases.push({
            title: "Analyze property chain #3",
            func: function () { console.log(o.b); },
            expectText: `exports.handler = __f0;

var __o = {};
var __o_b = {c: 2, d: 3};
__o.b = __o_b;

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: { c: 2, d: 3 } };

        cases.push({
            title: "Analyze property chain #4",
            func: function () { console.log(o.a); },
            expectText: `exports.handler = __f0;

var __o = {a: 1};

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.a); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: { c: { d: 1, e: 3 } } };

        cases.push({
            title: "Analyze property chain #5",
            func: function () { console.log(o.b.c.d); },
            expectText: `exports.handler = __f0;

var __o = {};
var __o_b = {};
var __o_b_c = {d: 1};
__o_b.c = __o_b_c;
__o.b = __o_b;

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b.c.d); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: { c: { d: 1, e: 3 } } };

        cases.push({
            title: "Analyze property chain #6",
            func: function () { console.log(o.b.c.d); console.log(o.b); },
            expectText: `exports.handler = __f0;

var __o = {};
var __o_b = {};
var __o_b_c = {d: 1, e: 3};
__o_b.c = __o_b_c;
__o.b = __o_b;

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b.c.d); console.log(o.b); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: { c: { d: 1, e: 3 } } };

        cases.push({
            title: "Analyze property chain #7",
            func: function () { console.log(o.b.c.d); console.log(o.b.c); },
            expectText: `exports.handler = __f0;

var __o = {};
var __o_b = {};
var __o_b_c = {d: 1, e: 3};
__o_b.c = __o_b_c;
__o.b = __o_b;

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b.c.d); console.log(o.b.c); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: { c: { d: 1, e: 3 } } };

        cases.push({
            title: "Analyze property chain #8",
            func: function () { console.log(o.b.c.d); console.log(o.b.c); console.log(o.b); },
            expectText: `exports.handler = __f0;

var __o = {};
var __o_b = {};
var __o_b_c = {d: 1, e: 3};
__o_b.c = __o_b_c;
__o.b = __o_b;

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b.c.d); console.log(o.b.c); console.log(o.b); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: function () { } };

        cases.push({
            title: "Analyze property chain #9",
            func: function () { console.log(o.b.name); },
            expectText: `exports.handler = __f0;

var __o = {b: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function () { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b.name); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: function () { } };

        cases.push({
            title: "Analyze property chain #10",
            func: function () { console.log(o.b.name); console.log(o.b()); },
            expectText: `exports.handler = __f0;

var __o = {b: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function () { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b.name); console.log(o.b()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: function () { } };

        cases.push({
            title: "Analyze property chain #11",
            func: function () { console.log(o.b()); console.log(o.b.name); },
            expectText: `exports.handler = __f0;

var __o = {b: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function () { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b()); console.log(o.b.name); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: function () { return this; } };

        cases.push({
            title: "Analyze property chain #12",
            func: function () { console.log(o.b.name); console.log(o.b()); },
            expectText: `exports.handler = __f0;

var __o = {a: 1, b: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function () { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b.name); console.log(o.b()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: function () { return this; } };

        cases.push({
            title: "Analyze property chain #13",
            func: function () { console.log(o.b()); console.log(o.b.name); },
            expectText: `exports.handler = __f0;

var __o = {a: 1, b: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function () { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __o }) {

return function () { console.log(o.b()); console.log(o.b.name); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o1 = { c: 2, d: 3 };
        const o2 = { a: 1, b: o1 };

        cases.push({
            title: "Analyze property chain #14",
            func: function () { console.log(o2.b.d); console.log(o1); },
            expectText: `exports.handler = __f0;

var __o2 = {};
var __o2_b = {d: 3, c: 2};
__o2.b = __o2_b;

function __f0() {
  return (function() {
    with({ o2: __o2, o1: __o2_b }) {

return function () { console.log(o2.b.d); console.log(o1); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o1 = { c: 2, d: 3 };
        const o2 = { a: 1, b: o1 };

        cases.push({
            title: "Analyze property chain #15",
            func: function () { console.log(o1); console.log(o2.b.d); },
            expectText: `exports.handler = __f0;

var __o1 = {c: 2, d: 3};
var __o2 = {};
__o2.b = __o1;

function __f0() {
  return (function() {
    with({ o1: __o1, o2: __o2 }) {

return function () { console.log(o1); console.log(o2.b.d); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o1 = { c: 2, d: 3 };
        const o2 = { a: 1, b: o1 };
        const o3 = { a: 1, b: o1 };

        cases.push({
            title: "Analyze property chain #16",
            func: function () { console.log(o2.b.c); console.log(o3.b.d); },
            expectText: `exports.handler = __f0;

var __o2 = {};
var __o2_b = {c: 2, d: 3};
__o2.b = __o2_b;
var __o3 = {};
__o3.b = __o2_b;

function __f0() {
  return (function() {
    with({ o2: __o2, o3: __o3 }) {

return function () { console.log(o2.b.c); console.log(o3.b.d); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o1 = { c: 2, d: 3 };
        const o2 = { a: 1, b: o1 };
        const o3 = { a: 1, b: o1 };

        cases.push({
            title: "Analyze property chain #17",
            func: function () { console.log(o2.b.d); console.log(o3.b.d); },
            expectText: `exports.handler = __f0;

var __o2 = {};
var __o2_b = {d: 3};
__o2.b = __o2_b;
var __o3 = {};
__o3.b = __o2_b;

function __f0() {
  return (function() {
    with({ o2: __o2, o3: __o3 }) {

return function () { console.log(o2.b.d); console.log(o3.b.d); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o1 = { c: 2, d: 3 };
        const o2 = { a: 1, b: o1 };
        const o3 = { a: 1, b: o1 };

        cases.push({
            title: "Analyze property chain #18",
            func: function () { console.log(o2.b); console.log(o2.b.d); console.log(o3.b.d); },
            expectText: `exports.handler = __f0;

var __o2 = {};
var __o2_b = {c: 2, d: 3};
__o2.b = __o2_b;
var __o3 = {};
__o3.b = __o2_b;

function __f0() {
  return (function() {
    with({ o2: __o2, o3: __o3 }) {

return function () { console.log(o2.b); console.log(o2.b.d); console.log(o3.b.d); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o1 = { c: 2, d: 3 };
        const o2 = { a: 1, b: o1 };
        const o3 = { a: 1, b: o1 };

        cases.push({
            title: "Analyze property chain #19",
            func: function () { console.log(o2.b.d); console.log(o3.b.d); console.log(o2.b); },
            expectText: `exports.handler = __f0;

var __o2 = {};
var __o2_b = {c: 2, d: 3};
__o2.b = __o2_b;
var __o3 = {};
__o3.b = __o2_b;

function __f0() {
  return (function() {
    with({ o2: __o2, o3: __o3 }) {

return function () { console.log(o2.b.d); console.log(o3.b.d); console.log(o2.b); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o1 = { c: 2, d: 3 };
        const o2 = { a: 1, b: o1 };
        const o3 = { a: 1, b: o1 };

        cases.push({
            title: "Analyze property chain #20",
            func: function () { console.log(o2.b.d); console.log(o3.b.d); console.log(o1); },
            expectText: `exports.handler = __f0;

var __o2 = {};
var __o2_b = {d: 3, c: 2};
__o2.b = __o2_b;
var __o3 = {};
__o3.b = __o2_b;

function __f0() {
  return (function() {
    with({ o2: __o2, o3: __o3, o1: __o2_b }) {

return function () { console.log(o2.b.d); console.log(o3.b.d); console.log(o1); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o1 = { c: 2, d: 3 };
        const o2 = { a: 1, b: o1 };
        const o3 = { a: 1, b: o1 };

        cases.push({
            title: "Analyze property chain #21",
            func: function () { console.log(o1); console.log(o2.b.d); console.log(o3.b.d);  },
            expectText: `exports.handler = __f0;

var __o1 = {c: 2, d: 3};
var __o2 = {};
__o2.b = __o1;
var __o3 = {};
__o3.b = __o1;

function __f0() {
  return (function() {
    with({ o1: __o1, o2: __o2, o3: __o3 }) {

return function () { console.log(o1); console.log(o2.b.d); console.log(o3.b.d); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const defaultsForThing = { config: { x: "x", y: "y" } };
        function getX() { return defaultsForThing.config.x }
        function getAll() { const x = getX(); return { x, y: defaultsForThing.config.y } }

        cases.push({
            title: "Analyze property chain #22",
            func: function () { console.log(getAll()); },
            expectText: `exports.handler = __f0;

var __defaultsForThing = {};
var __defaultsForThing_config = {x: "x", y: "y"};
__defaultsForThing.config = __defaultsForThing_config;

function __getX() {
  return (function() {
    with({ defaultsForThing: __defaultsForThing, getX: __getX }) {

return function /*getX*/() { return defaultsForThing.config.x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __getAll() {
  return (function() {
    with({ getX: __getX, defaultsForThing: __defaultsForThing, getAll: __getAll }) {

return function /*getAll*/() { const x = getX(); return { x, y: defaultsForThing.config.y }; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ getAll: __getAll }) {

return function () { console.log(getAll()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const defaultsForThing = { config: { x: "x", y: "y" } };
        function getAll() { return { y: defaultsForThing.config.y } }

        cases.push({
            title: "Analyze property chain #23",
            func: function () { console.log(getAll()); },
            expectText: `exports.handler = __f0;

var __defaultsForThing = {};
var __defaultsForThing_config = {y: "y"};
__defaultsForThing.config = __defaultsForThing_config;

function __getAll() {
  return (function() {
    with({ defaultsForThing: __defaultsForThing, getAll: __getAll }) {

return function /*getAll*/() { return { y: defaultsForThing.config.y }; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ getAll: __getAll }) {

return function () { console.log(getAll()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const config = { x: "x", y: "y" };
        function getX() { return config.x }
        function getAll() { const x = getX(); return { x, y: config.y } }

        cases.push({
            title: "Analyze property chain #24",
            func: function () { console.log(getAll()); },
            expectText: `exports.handler = __f0;

var __config = {x: "x", y: "y"};

function __getX() {
  return (function() {
    with({ config: __config, getX: __getX }) {

return function /*getX*/() { return config.x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __getAll() {
  return (function() {
    with({ getX: __getX, config: __config, getAll: __getAll }) {

return function /*getAll*/() { const x = getX(); return { x, y: config.y }; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ getAll: __getAll }) {

return function () { console.log(getAll()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const defaultsForThing = { config: { x: "x", y: "y" } };
        function getX() { return defaultsForThing }
        function getAll() { const x = getX(); return { y: defaultsForThing.config.y } }

        cases.push({
            title: "Analyze property chain #25",
            func: function () { console.log(getAll()); },
            expectText: `exports.handler = __f0;

var __defaultsForThing = {};
var __defaultsForThing_config = {x: "x", y: "y"};
__defaultsForThing.config = __defaultsForThing_config;

function __getX() {
  return (function() {
    with({ defaultsForThing: __defaultsForThing, getX: __getX }) {

return function /*getX*/() { return defaultsForThing; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __getAll() {
  return (function() {
    with({ getX: __getX, defaultsForThing: __defaultsForThing, getAll: __getAll }) {

return function /*getAll*/() { const x = getX(); return { y: defaultsForThing.config.y }; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ getAll: __getAll }) {

return function () { console.log(getAll()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const defaultsForThing = { config: { x: "x", y: "y" } };
        function getX() { return defaultsForThing.config }
        function getAll() { const x = getX(); return { y: defaultsForThing.config.y } }

        cases.push({
            title: "Analyze property chain #26",
            func: function () { console.log(getAll()); },
            expectText: `exports.handler = __f0;

var __defaultsForThing = {};
var __defaultsForThing_config = {x: "x", y: "y"};
__defaultsForThing.config = __defaultsForThing_config;

function __getX() {
  return (function() {
    with({ defaultsForThing: __defaultsForThing, getX: __getX }) {

return function /*getX*/() { return defaultsForThing.config; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __getAll() {
  return (function() {
    with({ getX: __getX, defaultsForThing: __defaultsForThing, getAll: __getAll }) {

return function /*getAll*/() { const x = getX(); return { y: defaultsForThing.config.y }; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ getAll: __getAll }) {

return function () { console.log(getAll()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const defaultsForThing = { config: { x: "x", y: "y" } };
        function getX() { return defaultsForThing.config.x }
        function getAll() { const x = getX(); return { y: defaultsForThing } }

        cases.push({
            title: "Analyze property chain #27",
            func: function () { console.log(getAll()); },
            expectText: `exports.handler = __f0;

var __defaultsForThing = {};
var __defaultsForThing_config = {x: "x", y: "y"};
__defaultsForThing.config = __defaultsForThing_config;

function __getX() {
  return (function() {
    with({ defaultsForThing: __defaultsForThing, getX: __getX }) {

return function /*getX*/() { return defaultsForThing.config.x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __getAll() {
  return (function() {
    with({ getX: __getX, defaultsForThing: __defaultsForThing, getAll: __getAll }) {

return function /*getAll*/() { const x = getX(); return { y: defaultsForThing }; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ getAll: __getAll }) {

return function () { console.log(getAll()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const defaultsForThing = { config: { x: "x", y: "y" } };
        function getX() { return defaultsForThing.config.x }
        function getAll() { const x = getX(); return { y: defaultsForThing.config } }

        cases.push({
            title: "Analyze property chain #28",
            func: function () { console.log(getAll()); },
            expectText: `exports.handler = __f0;

var __defaultsForThing = {};
var __defaultsForThing_config = {x: "x", y: "y"};
__defaultsForThing.config = __defaultsForThing_config;

function __getX() {
  return (function() {
    with({ defaultsForThing: __defaultsForThing, getX: __getX }) {

return function /*getX*/() { return defaultsForThing.config.x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __getAll() {
  return (function() {
    with({ getX: __getX, defaultsForThing: __defaultsForThing, getAll: __getAll }) {

return function /*getAll*/() { const x = getX(); return { y: defaultsForThing.config }; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ getAll: __getAll }) {

return function () { console.log(getAll()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        cases.push({
            title: "Capture non-built-in module",
            func: function () { typescript.parseCommandLine([""]); },
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ typescript: require("typescript/lib/typescript.js") }) {

return function () { typescript.parseCommandLine([""]); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

//     {
//         cases.push({
//             title: "Fail to capture non-deployment module due to native code",
//             func: function () { console.log(pulumi); },
//             error: `Error serializing function 'func': tsClosureCases.js(0,0)

// function 'func':(...)
//   module './bin/index.js' which indirectly referenced
//     function 'debug':(...)
// (...)
// Function code:
//   function (...)() { [native code] }

// Module './bin/index.js' is a 'deployment only' module. In general these cannot be captured inside a 'run time' function.`
//         });
//     }

    {
       // Used just to validate that if we capture a Config object we see these values serialized over.
       // Specifically, the module that Config uses needs to be captured by value and not be
       // 'require-reference'.
       deploymentOnlyModule.setConfig("test:TestingKey1", "TestingValue1");
       const testConfig = new deploymentOnlyModule.Config("test");

       cases.push({
           title: "Capture config created on the outside",
           func: function () { const v = testConfig.get("TestingKey1"); console.log(v); },
           expectText: `exports.handler = __f0;

var __testConfig_proto = {};
Object.defineProperty(__f1, "prototype", { value: __testConfig_proto });
Object.defineProperty(__testConfig_proto, "constructor", { configurable: true, writable: true, value: __f1 });
var __config = {["test:TestingKey1"]: "TestingValue1", ["test:TestingKey2"]: "TestingValue2"};
var __runtimeConfig_1 = {getConfig: __getConfig};
Object.defineProperty(__testConfig_proto, "get", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__testConfig_proto, "fullKey", { configurable: true, writable: true, value: __f3 });
var __testConfig = Object.create(__testConfig_proto);
__testConfig.name = "test";

function __f1(__0) {
  return (function() {
    with({  }) {

return function /*constructor*/(name) {
        if (name.endsWith(":config")) {
            name = name.replace(/:config$/, "");
        }
        this.name = name;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __getConfig(__0) {
  return (function() {
    with({ config: __config, getConfig: __getConfig }) {

return function /*getConfig*/(k) {
    return config[k];
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2(__0) {
  return (function() {
    with({ runtimeConfig_1: __runtimeConfig_1 }) {

return function /*get*/(key) {
        const v = runtimeConfig_1.getConfig(this.fullKey(key));
        if (v === undefined) {
            return undefined;
        }
        return v;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3(__0) {
  return (function() {
    with({  }) {

return function /*fullKey*/(key) {
        return this.name + ":" + key;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ testConfig: __testConfig }) {

return function () { const v = testConfig.get("TestingKey1"); console.log(v); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
       });
    }

    {
        deploymentOnlyModule.setConfig("test:TestingKey2", "TestingValue2");

        cases.push({
           title: "Capture config created on the inside",
           func: function () { const v = new deploymentOnlyModule.Config("test").get("TestingKey2"); console.log(v); },
           expectText: `exports.handler = __f0;

var __f1_prototype = {};
Object.defineProperty(__f1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
var __config = {["test:TestingKey1"]: "TestingValue1", ["test:TestingKey2"]: "TestingValue2"};
var __runtimeConfig_1 = {getConfig: __getConfig};
Object.defineProperty(__f1_prototype, "get", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__f1_prototype, "fullKey", { configurable: true, writable: true, value: __f3 });
Object.defineProperty(__f1, "prototype", { value: __f1_prototype });
var __deploymentOnlyModule = {Config: __f1};

function __f1(__0) {
  return (function() {
    with({  }) {

return function /*constructor*/(name) {
        if (name.endsWith(":config")) {
            name = name.replace(/:config$/, "");
        }
        this.name = name;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __getConfig(__0) {
  return (function() {
    with({ config: __config, getConfig: __getConfig }) {

return function /*getConfig*/(k) {
    return config[k];
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2(__0) {
  return (function() {
    with({ runtimeConfig_1: __runtimeConfig_1 }) {

return function /*get*/(key) {
        const v = runtimeConfig_1.getConfig(this.fullKey(key));
        if (v === undefined) {
            return undefined;
        }
        return v;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3(__0) {
  return (function() {
    with({  }) {

return function /*fullKey*/(key) {
        return this.name + ":" + key;
    };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ deploymentOnlyModule: __deploymentOnlyModule }) {

return function () { const v = new deploymentOnlyModule.Config("test").get("TestingKey2"); console.log(v); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
       });
    }

    {
        cases.push({
            title: "Capture factory func #1",
            factoryFunc: () => {
                const serverlessExpress = require("aws-serverless-express");
                const express = require("express");
                const app = express();
                app.get("/", (req: any, res: any) => {
                    res.json({ succeeded: true });
                });

                const server = serverlessExpress.createServer(app);

                return (event: any, context: any) => {
                    serverlessExpress.proxy(server, event, context);
                };
            },
            expectText: `
function __f0() {
  return (function() {
    with({  }) {

return () => {
                const serverlessExpress = require("aws-serverless-express");
                const express = require("express");
                const app = express();
                app.get("/", (req, res) => {
                    res.json({ succeeded: true });
                });
                const server = serverlessExpress.createServer(app);
                return (event, context) => {
                    serverlessExpress.proxy(server, event, context);
                };
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

exports.handler = __f0();`,
        });
    }

    {
        const outerVal = [{}];
        (<any>outerVal[0]).inner = outerVal;

        function foo() {
            outerVal.pop();
        }

        function bar() {
            outerVal.join();
        }

        cases.push({
            title: "Capture factory func #2",
            factoryFunc: () => {
                outerVal.push({});
                foo();

                return (event: any, context: any) => {
                    bar();
                };
            },
            expectText: `
var __outerVal = [];
var __outerVal_0 = {};
__outerVal_0.inner = __outerVal;
__outerVal[0] = __outerVal_0;

function __foo() {
  return (function() {
    with({ outerVal: __outerVal, foo: __foo }) {

return function /*foo*/() {
            outerVal.pop();
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __bar() {
  return (function() {
    with({ outerVal: __outerVal, bar: __bar }) {

return function /*bar*/() {
            outerVal.join();
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ outerVal: __outerVal, foo: __foo, bar: __bar }) {

return () => {
                outerVal.push({});
                foo();
                return (event, context) => {
                    bar();
                };
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

exports.handler = __f0();`,
        });
    }

    cases.push({
        title: "Deconstructing function",
        // @ts-ignore
        func: function f({ whatever }) { },
        expectText: `exports.handler = __f;

function __f(__0) {
  return (function() {
    with({ f: __f }) {

return function /*f*/({ whatever }) { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Deconstructing async function",
        // @ts-ignore
        func: async function f({ whatever }) { },
        expectText: `exports.handler = __f;

function __f0(__0, __1, __2, __3) {
  return (function() {
    with({  }) {

return function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f(__0) {
  return (function() {
    with({ __awaiter: __f0, f: __f }) {

return function /*f*/({ whatever }) {
            return __awaiter(this, void 0, void 0, function* () { });
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Deconstructing arrow function",
        // @ts-ignore
        func: ({ whatever }) => { },
        expectText: `exports.handler = __f0;

function __f0(__0) {
  return (function() {
    with({  }) {

return ({ whatever }) => { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Deconstructing async arrow function",
        // @ts-ignore
        func: async ({ whatever }) => { },
        expectText: `exports.handler = __f0;

function __f1(__0, __1, __2, __3) {
  return (function() {
    with({  }) {

return function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0(__0) {
  return (function() {
    with({ __awaiter: __f1 }) {

return ({ whatever }) => __awaiter(void 0, void 0, void 0, function* () { });

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    {
        const regex = /(abc)[\(123-456]\\a\b\z/gi;

        cases.push({
            title: "Regex #1",
            // @ts-ignore
            func: function() { console.log(regex); },
            expectText: `exports.handler = __f0;

var __regex = new RegExp("(abc)[\\\\(123-456]\\\\\\\\a\\\\b\\\\z", "gi");

function __f0() {
  return (function() {
    with({ regex: __regex }) {

return function () { console.log(regex); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const regex = /(abc)/g;

        function foo() {
            console.log(regex);
        }

        cases.push({
            title: "Regex #2",
            // @ts-ignore
            func: function() { console.log(regex); foo(); },
            expectText: `exports.handler = __f0;

var __regex = new RegExp("(abc)", "g");

function __foo() {
  return (function() {
    with({ regex: __regex, foo: __foo }) {

return function /*foo*/() {
            console.log(regex);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ regex: __regex, foo: __foo }) {

return function () { console.log(regex); foo(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const regex = /(abc)/;

        function foo() {
            console.log(regex);
        }

        cases.push({
            title: "Regex #3 (no flags)",
            // @ts-ignore
            func: function() { console.log(regex); foo(); },
            expectText: `exports.handler = __f0;

var __regex = new RegExp("(abc)", "");

function __foo() {
  return (function() {
    with({ regex: __regex, foo: __foo }) {

return function /*foo*/() {
            console.log(regex);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ regex: __regex, foo: __foo }) {

return function () { console.log(regex); foo(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const s = pulumi.secret("can't capture me");

        cases.push({
            title: "Can't capture secrets without allowSecrets",
            func: function() {
                console.log(s.get());
            },
            error: "Secret outputs cannot be captured by a closure.",
        });
    }

    {
      const s = pulumi.secret("can't capture me");

      cases.push({
          title: "Can capture secrets with allowSecrets",
          func: function() {
              console.log(s.get());
          },
          allowSecrets: true,
          expectText: `(...)`,
      });
  }

    // Run a bunch of direct checks on async js functions if we're in node 8 or above.
    // We can't do this inline as node6 doesn't understand 'async functions'.  And we
    // can't do this in TS as TS will convert the async-function to be a normal non-async
    // function.
    if (semver.gte(process.version, "8.0.0")) {
        const jsCases = require("./jsClosureCases_8");
        cases.push(...jsCases.cases);
    }

    if (semver.gte(process.version, "10.4.0")) {
        const jsCases = require("./jsClosureCases_10_4");
        cases.push(...jsCases.cases);
    }

    // Make a callback to keep running tests.
    let remaining = cases;
    while (true) {
        const test = remaining.shift();
        if (!test) {
            return;
        }

        // if (test.title.indexOf("Analyze property chain #2") < 0) {
        // //if (test.title !== "Analyze property chain #23") {
        //     continue;
        // }

        it(test.title, asyncTest(async () => {
            // Run pre-actions.
            if (test.pre) {
                test.pre();
            }

            // Invoke the test case.
            if (test.expectText) {
                const sf = await serializeFunction(test);
                compareTextWithWildcards(test.expectText, sf.text);
            }
            else {
                const message = await assertAsyncThrows(async () => {
                    await serializeFunction(test);
                });

                // replace real locations with (0,0) so that our test baselines do not need to
                // updated any time this file changes.
                const regex = /\([0-9]+,[0-9]+\)/g;
                const withoutLocations = message.replace(regex, "(0,0)");
                if (test.error) {
                    compareTextWithWildcards(test.error, withoutLocations);
                }
            }
        }));

        // Schedule any additional tests.
        if (test.afters) {
            remaining = test.afters.concat(remaining);
        }
    }

    async function serializeFunction(test: ClosureCase) {
        if (test.func) {
            return await runtime.serializeFunction(test.func, {
              allowSecrets: test.allowSecrets,
            });
        }
        else if (test.factoryFunc) {
            return await runtime.serializeFunction(test.factoryFunc!, { 
              allowSecrets: test.allowSecrets,
              isFactoryFunction: true,
            });
        }
        else {
            throw new Error("Have to supply [func] or [factoryFunc]!");
        }
    }
});

/**
 * compareErrorText compares an "expected" error string and an "actual" error string
 * and issues an error if they do not match.
 *
 * This function accepts two repetition operators to make writing tests easier against
 * error messages that are dependent on the environment:
 *
 *  * (...) alone on a single line causes the matcher to accept zero or more lines
 *    between the repetition and the next line.
 *  * (...) within in the context of a line causes the matcher to accept zero or more characters
 *    between the repetition and the next character.
 *
 * This is useful when testing error messages that you get when capturing bulit-in modules,
 * because the specific error message differs between Node versions.
 * @param expected The expected error message string, potentially containing repetitions
 * @param actual The actual error message string
 */
function compareTextWithWildcards(expected: string, actual: string) {
    const wildcard = "(...)";

    if (!expected.includes(wildcard)) {
        // We get a nice diff view if we diff the entire string, so do that
        // if we didn't get a wildcard.
        assert.strictEqual(actual, expected);
        return;
    }

    const expectedLines = expected.split(EOL);
    const actualLines = actual.split(EOL);
    let actualIndex = 0;
    for (let expectedIndex = 0; expectedIndex < expectedLines.length; expectedIndex++) {
        const expectedLine = expectedLines[expectedIndex].trim();
        if (expectedLine === wildcard) {
            if (expectedIndex + 1 === expectedLines.length) {
                return;
            }

            const nextLine = expectedLines[++expectedIndex].trim();
            while (true) {
                const actualLine = actualLines[actualIndex++].trim();
                if (actualLine === nextLine) {
                    break;
                }

                if (actualIndex === actualLines.length) {
                    assert.fail(`repetition failed to find match: expected terminator ${nextLine}, received ${actual}`);
                }
            }
        } else if (expectedLine.includes(wildcard)) {
            const line = actualLines[actualIndex++].trim();
            const index = expectedLine.indexOf(wildcard);
            const indexAfter = index + wildcard.length;
            assert.strictEqual(line.substring(0, index), expectedLine.substring(0, index));

            if (indexAfter === expectedLine.length) {
                continue;
            }
            let repetitionIndex = index;
            for (; repetitionIndex < line.length; repetitionIndex++) {
                if (line[repetitionIndex] === expectedLine[indexAfter]) {
                    break;
                }
            }

            assert.strictEqual(line.substring(repetitionIndex), expectedLine.substring(indexAfter));
        } else {
            assert.strictEqual(actualLines[actualIndex++].trim(), expectedLine);
        }
    }
}
