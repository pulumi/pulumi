// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// tslint:disable:max-line-length

import * as assert from "assert";
import { runtime } from "../../index";
import * as resource from "../../resource";
import { Output, output } from "../../resource";
import { assertAsyncThrows, asyncTest } from "../util";

interface ClosureCase {
    pre?: () => void;         // an optional function to run before this case.
    title: string;            // a title banner for the test case.
    func: Function;           // the function whose body and closure to serialize.
    expectText: string;      // optionally also validate the serialization to JavaScript text.
    afters?: ClosureCase[];   // an optional list of test cases to run afterwards.
}

// This group of tests ensure that we serialize closures properly.
describe("closure", () => {
    const cases: ClosureCase[] = [];

    cases.push({
        title: "Empty function closure",
        // tslint:disable-next-line
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
        // tslint:disable-next-line
        func: function f() { },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ f: __f0 }) {

return function /*f*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Named function with self-reference",
        // tslint:disable-next-line
        func: function f() { f(); },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ f: __f0 }) {

return function /*f*/() { f(); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Function closure with this capture",
        // tslint:disable-next-line
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
        // tslint:disable-next-line
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
        // tslint:disable-next-line
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
        // tslint:disable-next-line
        func: () => { console.log(this); },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({  }) {

return () => { console.log(this); };

    }
  }).apply(require("./bin/tests/runtime/closure.spec.js"), undefined).apply(this, arguments);
}
`,
    });

    const awaiterCode =
`
function __f1() {
  return (function() {
    with({  }) {

return function (thisArg, _arguments, P, generator) {
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`;

    cases.push({
        title: "Async lambda that does not capture this",
        // tslint:disable-next-line
        func: async () => { },
        expectText: `exports.handler = __f0;
${awaiterCode}
function __f0() {
  return (function() {
    with({ __awaiter: __f1 }) {

return () => __awaiter(this, void 0, void 0, function* () { });

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Async lambda that does capture this",
        // tslint:disable-next-line
        func: async () => { console.log(this); },
        expectText: `exports.handler = __f0;
${awaiterCode}
function __f0() {
  return (function() {
    with({ __awaiter: __f1 }) {

return () => __awaiter(this, void 0, void 0, function* () { console.log(this); });

    }
  }).apply(require("./bin/tests/runtime/closure.spec.js"), undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Async function that does not capture this",
        // tslint:disable-next-line
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
        // tslint:disable-next-line
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
        // tslint:disable-next-line
        func: (function() { return () => { console.log(this + arguments); } }).apply(this, [0, 1]),
        expectText: `exports.handler = __f0;

var __e0_arguments = [0, 1];

function __f0() {
  return (function() {
    with({  }) {

return () => { console.log(this + arguments); };

    }
  }).apply(require("./bin/tests/runtime/closure.spec.js"), __e0_arguments).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Arrow closure with this capture inside function closure",
        // tslint:disable-next-line
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
        // tslint:disable-next-line
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

    cases.push({
        title: "Empty function closure w/ args",
        // tslint:disable-next-line
        func: function (x: any, y: any, z: any) { },
        expectText: `exports.handler = __f0;

function __f0() {
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
        // tslint:disable-next-line
        func: (x: any, y: any, z: any) => { },
        expectText: `exports.handler = __f0;

function __f0() {
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
            // tslint:disable-next-line
            func: () => { console.log(wcap + `${xcap}` + ycap.length + eval(zcap.a + zcap.b + zcap.c)); },
            expectText: `exports.handler = __f0;

var __e0_ycap = [true, -1, "yup"];
var __e1_zcap = {};
__e1_zcap.a = "a";
__e1_zcap.b = false;
var __e2_c = [0];
__e1_zcap.c = __e2_c;

function __f0() {
  return (function() {
    with({ wcap: "foo", xcap: 97, ycap: __e0_ycap, zcap: __e1_zcap }) {

return () => { console.log(wcap + \`$\{xcap\}\` + ycap.length + eval(zcap.a + zcap.b + zcap.c)); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }
    {
        // tslint:disable-next-line
        let nocap1 = 1, nocap2 = 2, nocap3 = 3, nocap4 = 4, nocap5 = 5, nocap6 = 6, nocap7 = 7;
        // tslint:disable-next-line
        let nocap8 = 8, nocap9 = 9, nocap10 = 10;
        // tslint:disable-next-line
        let cap1 = 100, cap2 = 200, cap3 = 300, cap4 = 400, cap5 = 500, cap6 = 600, cap7 = 700;
        // tslint:disable-next-line
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
            // tslint:disable-next-line
            func: eval(functext),
            expectText: `exports.handler = __f0;

function __f0() {
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
        // tslint:disable-next-line
        let nocap1 = 1;
        // tslint:disable-next-line
        let cap1 = 100;

        cases.push({
            title: "Complex capturing cases #1",
            func: () => {
                // cap1 is captured here.
                // nocap1 introduces a new variable that shadows the outer one.
                // tslint:disable-next-line
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
                // tslint:disable-next-line
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
        // tslint:disable-next-line
        let nocap1 = 1;
        // tslint:disable-next-line
        let cap1 = 100;

        cases.push({
            title: "Complex capturing cases #2",
            func: () => {
    // cap1 is captured here.
    // nocap1 introduces a new variable that shadows the outer one.
    // tslint:disable-next-line
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
                // tslint:disable-next-line
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
        // tslint:disable-next-line
        let nocap1 = 1;
        // tslint:disable-next-line
        let cap1 = 100;

        cases.push({
            title: "Complex capturing cases #3",
            func: () => {
    // cap1 is captured here.
    // nocap1 introduces a new variable that shadows the outer one.
    // tslint:disable-next-line
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
                // tslint:disable-next-line
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
        // tslint:disable-next-line
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
            title: "Capture built-in modules as stable references, not serialized values",
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
        const util = require("../util");
        cases.push({
            title: "Capture user-defined modules as stable references, not serialized values",
            func: () => util,
            expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ util: require("./bin/tests/util.js") }) {

return () => util;

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    cases.push({
        title: "Don't capture catch variables",
        // tslint:disable-next-line
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
            // tslint:disable-next-line
            func: func,
            expectText: `exports.handler = __f0;

var __e0_xcap = {};
__e0_xcap.fff = __f1;
__e0_xcap.ggg = __f2;
var __e1_zzz = {};
var __e2_a = [__f3];
__e1_zzz.a = __e2_a;
__e0_xcap.zzz = __e1_zzz;

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

function __f3() {
  return (function() {
    with({  }) {

return (a1, a2) => { console.log(a1 + a2); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ xcap: __e0_xcap }) {

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
            expectText: `exports.handler = __f0;

var __e0_this = {x: 42};

function __f0() {
  return (function() {
    with({  }) {

return () => { console.log(this.x); };

    }
  }).apply(__e0_this, undefined).apply(this, arguments);
}
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

var __e0_mutable = {};

function __f0() {
  return (function() {
    with({ mutable: __e0_mutable }) {

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

var __e0_mutable = {timesTheyAreAChangin: true};

function __f0() {
  return (function() {
    with({ mutable: __e0_mutable }) {

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
            // tslint:disable-next-line
            func: function () { console.log(v); },
            expectText: `exports.handler = __f0;

var __e0_v = {d: { get: () => 4 }};

function __f0() {
  return (function() {
    with({ v: __e0_v }) {

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
            // tslint:disable-next-line
            func: function () { console.log(v); },
            expectText: `exports.handler = __f0;

var __e0_v = {};
__e0_v.d1 = { get: () => 4 };
__e0_v.d2 = { get: () => "str" };
__e0_v.d3 = { get: () => undefined };
var __e1_d4 = {a: 1, b: true};
__e0_v.d4 = { get: () => __e1_d4 };

function __f0() {
  return (function() {
    with({ v: __e0_v }) {

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
            // tslint:disable-next-line
            func: function () { console.log(v); },
            expectText: `exports.handler = __f0;

var __e0_v = {};
__e0_v.d1 = { get: () => 4 };
var __e1_d2 = {};
__e1_d2.a = 1;
__e1_d2.b = __e0_v;
__e0_v.d2 = { get: () => __e1_d2 };

function __f0() {
  return (function() {
    with({ v: __e0_v }) {

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
            // tslint:disable-next-line
            func: function () { console.log(obj); },
            expectText: `exports.handler = __f0;

var __e0_obj = {method1: __f1, method2: __f2};

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
    with({ obj: __e0_obj }) {

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
            // tslint:disable-next-line
            func: function () { return array; },
            expectText: `exports.handler = __f0;

var __e0_array = [];
var __e1_array_0 = {};
__e1_array_0["80"] = "foo";
__e1_array_0.arr = __e0_array;
__e0_array[0] = __e1_array_0;

function __f0() {
  return (function() {
    with({ array: __e0_array }) {

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
            // tslint:disable-next-line:no-empty no-shadowed-variable
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

var __e0_prototype = {};
Object.defineProperty(__e0_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
var __e1_outer = {};
__e1_outer.o = 1;
var __e2_b = [];
__e2_b[0] = __e1_outer;
__e1_outer.b = __e2_b;
__e0_prototype.m = __f2;
__e0_prototype.n = __f3;
__f1.prototype = __e0_prototype;
__f1.m = __f4;

function __f1() {
  return (function() {
    with({ C: __f1 }) {

return function /*C*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ outer: __e1_outer }) {

return function () { return this.n() + outer; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({ array: __e2_b }) {

return function () { return array; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4() {
  return (function() {
    with({  }) {

return function () { return this.n(); };

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

var __e0_prototype = {};
Object.defineProperty(__e0_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__e0_prototype, "m", { configurable: true, writable: true, value: __f2 });
var __e1_outer = {};
__e1_outer.o = 1;
var __e2_b = [];
__e2_b[0] = __e1_outer;
__e1_outer.b = __e2_b;
Object.defineProperty(__e0_prototype, "n", { configurable: true, writable: true, value: __f3 });
__f1.prototype = __e0_prototype;
__f1.s = __f4;

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
    with({ outer: __e1_outer }) {

return function /*n*/() { return outer; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4() {
  return (function() {
    with({ array: __e2_b }) {

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

var __e0_prototype = {};
Object.defineProperty(__e0_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__e0_prototype, "m", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__e0_prototype, "n", { configurable: true, writable: true, value: __f3 });
__f1.prototype = __e0_prototype;
__f1.s = __f4;

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

var __e0_prototype = {};
Object.defineProperty(__e0_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__e0_prototype, "m", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__e0_prototype, "n", { configurable: true, writable: true, value: __f3 });
__f1.prototype = __e0_prototype;
__f1.s = __f4;

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
            // tslint:disable-next-line:no-shadowed-variable
            function D() {
                // tslint:disable-next-line:semicolon
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

var __e0_array = [];
__e0_array[0] = 1;
__e0_array[1] = __e0_array;

function __f0() {
  return (function() {
    with({ array: __e0_array }) {

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

var __e0_obj = {};
__e0_obj.a = 1;
__e0_obj.b = __e0_obj;

function __f0() {
  return (function() {
    with({ obj: __e0_obj }) {

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

var __e0_obj = {};
var __e1_a = [];
__e1_a[0] = __e0_obj;
__e0_obj.a = __e1_a;
__e0_obj.b = __e1_a;

function __f0() {
  return (function() {
    with({ obj: __e0_obj }) {

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

var __e0_obj2 = [];
var __e1_obj2_0 = {};
var __e2_a = [];
__e2_a[0] = __e1_obj2_0;
__e1_obj2_0.a = __e2_a;
__e1_obj2_0.b = __e2_a;
__e0_obj2[0] = __e1_obj2_0;
__e0_obj2[1] = __e1_obj2_0;

function __f0() {
  return (function() {
    with({ obj2: __e0_obj2 }) {

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

var __e0_obj = {a: 1};

function __f1() {
  return (function() {
    with({ obj: __e0_obj, f1: __f1 }) {

return function /*f1*/() { return obj; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ obj: __e0_obj, f2: __f2 }) {

return function /*f2*/() { console.log(obj); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ f1: __f1, obj: __e0_obj, f2: __f2 }) {

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

var __e0_v = {};
Object.defineProperty(__e0_v, "key", { configurable: true, value: 1 });

function __f0() {
  return (function() {
    with({ v: __e0_v }) {

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

var __e0_v = {};
Object.defineProperty(__e0_v, "key", { enumerable: true, writable: true, value: 1 });

function __f0() {
  return (function() {
    with({ v: __e0_v }) {

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

var __e0_v = [];
__e0_v[0] = 1;
__e0_v[2] = 3;

function __f0() {
  return (function() {
    with({ v: __e0_v }) {

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

var __e0_v = [];
__e0_v[0] = 1;
__e0_v[2] = 3;
__e0_v.foo = "";

function __f0() {
  return (function() {
    with({ v: __e0_v }) {

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

var __e0_x = Object.create(null);

function __f0() {
  return (function() {
    with({ x: __e0_x }) {

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

var __e0_x = Object.create(global.Number.prototype);

function __f0() {
  return (function() {
    with({ x: __e0_x }) {

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

var __e1_x_proto = {x: __f1};
var __e0_x = Object.create(__e1_x_proto);

function __f1() {
  return (function() {
    with({ v: __f0 }) {

return function /*x*/() { return v; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ x: __e0_x }) {

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
            expectText: `exports.handler = __f0;

var __e0_prototype = Object.create(Object.getPrototypeOf((function*(){}).prototype));
__f0.prototype = __e0_prototype;
Object.setPrototypeOf(__f0, Object.getPrototypeOf(function*(){}));

function __f0() {
  return (function() {
    with({ f: __f0 }) {

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

var __e0_prototype = Object.create(Object.getPrototypeOf((function*(){}).prototype));
__f0.prototype = __e0_prototype;
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

            // tslint:disable-next-line:no-empty
            set foo(v: number) {
                this._x = v;
            }
        }

        cases.push({
            title: "Test getter/setter #1",
            func: () => C,
            expectText: `exports.handler = __f0;

var __e0_prototype = {};
Object.defineProperty(__e0_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__e0_prototype, "foo", { configurable: true, get: __f2, set: __f3 });
__f1.prototype = __e0_prototype;

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

function __f3() {
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

var __e0_prototype = {};
Object.defineProperty(__e0_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__e0_prototype, "method name", { configurable: true, writable: true, value: __f2 });
__f1.prototype = __e0_prototype;

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

var __e0_prototype = {};
Object.defineProperty(__e0_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
var __e1_sym = Object.create(global.Symbol.prototype);
Object.defineProperty(__e0_prototype, "getSym", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__e0_prototype, __e1_sym, { configurable: true, writable: true, value: __f3 });
__f1.prototype = __e0_prototype;

function __f1() {
  return (function() {
    with({  }) {

return function /*constructor*/() { };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ sym: __e1_sym }) {

return function /*getSym*/() { return sym; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
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

var __e0_prototype = {};
Object.defineProperty(__e0_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__e0_prototype, Symbol.iterator, { configurable: true, writable: true, value: __f2 });
__f1.prototype = __e0_prototype;

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

var __e0_prototype = {};
Object.defineProperty(__e0_prototype, "constructor", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__e0_prototype, "dMethod", { configurable: true, writable: true, value: __f3 });
Object.defineProperty(__e0_prototype, "dVirtual", { configurable: true, writable: true, value: __f4 });
__f2.prototype = __e0_prototype;
var __e1_prototype = Object.create(__e0_prototype);
Object.defineProperty(__e1_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__e1_prototype, "cMethod", { configurable: true, writable: true, value: __f5 });
Object.defineProperty(__e1_prototype, "dVirtual", { configurable: true, writable: true, value: __f6 });
__f1.prototype = __e1_prototype;
Object.setPrototypeOf(__f1, __f2);

function __f2() {
  return (function() {
    with({  }) {

return function /*constructor*/(n) {
                this.n = n;
                console.log("DConstructor");
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
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

function __f1() {
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

var __e0_prototype = {};
Object.defineProperty(__e0_prototype, "constructor", { configurable: true, writable: true, value: __f3 });
Object.defineProperty(__e0_prototype, "method", { configurable: true, writable: true, value: __f4 });
__f3.prototype = __e0_prototype;
var __e1_prototype = Object.create(__e0_prototype);
Object.defineProperty(__e1_prototype, "constructor", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__e1_prototype, "method", { configurable: true, writable: true, value: __f5 });
__f2.prototype = __e1_prototype;
Object.setPrototypeOf(__f2, __f3);
var __e2_prototype = Object.create(__e1_prototype);
Object.defineProperty(__e2_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__e2_prototype, "method", { configurable: true, writable: true, value: __f6 });
__f1.prototype = __e2_prototype;
Object.setPrototypeOf(__f1, __f2);

function __f3() {
  return (function() {
    with({  }) {

return function /*constructor*/(n) {
                this.n = n;
                console.log("AConstruction");
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4() {
  return (function() {
    with({  }) {

return function /*method*/(x) { return x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ __super: __f3 }) {

return function /*constructor*/(n) {
    __super.call(this, n + 1);
    console.log("BConstructor");
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f5() {
  return (function() {
    with({ __super: __f3 }) {

return function /*method*/(n) { return 1 + __super.prototype.method.call(this, n + 1); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({ __super: __f2 }) {

return function /*constructor*/(n) {
    __super.call(this, n * 2);
    console.log("CConstructor");
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f6() {
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

var __e0_prototype = {};
Object.defineProperty(__e0_prototype, "constructor", { configurable: true, writable: true, value: __f3 });
var __e1_sym = Object.create(global.Symbol.prototype);
Object.defineProperty(__e0_prototype, __e1_sym, { configurable: true, writable: true, value: __f4 });
__f3.prototype = __e0_prototype;
var __e2_prototype = Object.create(__e0_prototype);
Object.defineProperty(__e2_prototype, "constructor", { configurable: true, writable: true, value: __f2 });
Object.defineProperty(__e2_prototype, __e1_sym, { configurable: true, writable: true, value: __f5 });
__f2.prototype = __e2_prototype;
Object.setPrototypeOf(__f2, __f3);
var __e3_prototype = Object.create(__e2_prototype);
Object.defineProperty(__e3_prototype, "constructor", { configurable: true, writable: true, value: __f1 });
Object.defineProperty(__e3_prototype, __e1_sym, { configurable: true, writable: true, value: __f6 });
__f1.prototype = __e3_prototype;
Object.setPrototypeOf(__f1, __f2);

function __f3() {
  return (function() {
    with({  }) {

return function /*constructor*/(n) {
                this.n = n;
                console.log("AConstruction");
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4() {
  return (function() {
    with({  }) {

return function (x) { return x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f2() {
  return (function() {
    with({ __super: __f3 }) {

return function /*constructor*/(n) {
    __super.call(this, n + 1);
    console.log("BConstructor");
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f5() {
  return (function() {
    with({ sym: __e1_sym, __super: __f3 }) {

return function /*__computed*/(n) { return 1 + __super.prototype[sym].call(this, n + 1); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({ __super: __f2 }) {

return function /*constructor*/(n) {
    __super.call(this, n * 2);
    console.log("CConstructor");
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f6() {
  return (function() {
    with({ sym: __e1_sym, __super: __f2 }) {

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

__f2.method = __f3;
var __e0_sym = Object.create(global.Symbol.prototype);
__f2[__e0_sym] = __f4;
__f1.method = __f5;
__f1[__e0_sym] = __f6;
Object.setPrototypeOf(__f1, __f2);

function __f2() {
  return (function() {
    with({  }) {

return function /*constructor*/(n) {
                this.n = n;
                console.log("AConstruction");
            };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f3() {
  return (function() {
    with({  }) {

return function /*method*/(x) { return x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f4() {
  return (function() {
    with({  }) {

return function (x) { return x * x; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f1() {
  return (function() {
    with({ __super: __f2 }) {

return function /*constructor*/(n) {
    __super.call(this, n + 1);
    console.log("BConstructor");
};

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f5() {
  return (function() {
    with({ __super: __f2 }) {

return function /*method*/(n) { return 1 + __super.method.call(this, n + 1); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f6() {
  return (function() {
    with({ sym: __e0_sym, __super: __f2 }) {

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
            // tslint:disable-next-line
            func: function () { console.log(o.a); },
            expectText: `exports.handler = __f0;

var __e0_o = {a: 1};

function __f0() {
  return (function() {
    with({ o: __e0_o }) {

return function () { console.log(o.a); };

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
            // tslint:disable-next-line
            func: function () { console.log(o.b + o.c); },
            expectText: `exports.handler = __f0;

var __e0_o = {b: 2, c: 3};

function __f0() {
  return (function() {
    with({ o: __e0_o }) {

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
            title: "Capture all if object is used as is.",
            // tslint:disable-next-line
            func: function () { console.log(o); },
            expectText: `exports.handler = __f0;

var __e0_o = {a: 1, b: 2, c: 3};

function __f0() {
  return (function() {
    with({ o: __e0_o }) {

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
            title: "Capture all if object property is invoked, and it uses this.",
            // tslint:disable-next-line
            func: function () { console.log(o.c()); },
            expectText: `exports.handler = __f0;

var __e0_o = {c: __f1, a: 1, b: 2};

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __e0_o }) {

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
            title: "Capture one if object property is invoked, and it does not use this.",
            // tslint:disable-next-line
            func: function () { console.log(o.c()); },
            expectText: `exports.handler = __f0;

var __e0_o = {c: __f1};

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { return 0; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __e0_o }) {

return function () { console.log(o.c()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
        });
    }

    {
        const o = { a: 1, b: { c() { return this; } } };

        cases.push({
            title: "Capture subset if sub object property is invoked.",
            // tslint:disable-next-line
            func: function () { console.log(o.b.c()); },
            expectText: `exports.handler = __f0;

var __e0_o = {};
var __e1_b = {c: __f1};
__e0_o.b = __e1_b;

function __f1() {
  return (function() {
    with({  }) {

return function /*c*/() { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __e0_o }) {

return function () { console.log(o.b.c()); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
});
    }

    {
        const o = { a: 1, get b() { return this; } };

        cases.push({
            title: "Capture all if getter and getter uses this.",
            // tslint:disable-next-line
            func: function () { console.log(o.b); },
            expectText: `exports.handler = __f0;

var __e0_o = {};
Object.defineProperty(__e0_o, "b", { configurable: true, enumerable: true, get: __f1 });
__e0_o.a = 1;

function __f1() {
  return (function() {
    with({  }) {

return function /*b*/() { return this; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __e0_o }) {

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
            title: "Capture one if getter and getter does not use this.",
            // tslint:disable-next-line
            func: function () { console.log(o.b); },
            expectText: `exports.handler = __f0;

var __e0_o = {};
Object.defineProperty(__e0_o, "b", { configurable: true, enumerable: true, get: __f1 });

function __f1() {
  return (function() {
    with({  }) {

return function /*b*/() { return 0; };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __e0_o }) {

return function () { console.log(o.b); };

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
            // tslint:disable-next-line
            func: f1,
            expectText: `exports.handler = __f0;

var __e0_o = {a: 1, c: 2};

function __f1() {
  return (function() {
    with({ o: __e0_o, f2: __f1 }) {

return function /*f2*/() {
            console.log(o.c);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __e0_o, f2: __f1, f1: __f0 }) {

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
            console.log(o.a);
            f2();
        }

        function f2() {
            console.log(o);
        }

        cases.push({
            title: "Capture all props from different contexts #1",
            // tslint:disable-next-line
            func: f1,
            expectText: `exports.handler = __f0;

var __e0_o = {a: 1, b: 1, c: 2};

function __f1() {
  return (function() {
    with({ o: __e0_o, f2: __f1 }) {

return function /*f2*/() {
            console.log(o);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __e0_o, f2: __f1, f1: __f0 }) {

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
            console.log(o);
            f2();
        }

        function f2() {
            console.log(o.a);
        }

        cases.push({
            title: "Capture all props from different contexts #2",
            // tslint:disable-next-line
            func: f1,
            expectText: `exports.handler = __f0;

var __e0_o = {a: 1, b: 1, c: 2};

function __f1() {
  return (function() {
    with({ o: __e0_o, f2: __f1 }) {

return function /*f2*/() {
            console.log(o.a);
        };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f0() {
  return (function() {
    with({ o: __e0_o, f2: __f1, f1: __f0 }) {

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

    // Make a callback to keep running tests.
    let remaining = cases;
    while (true) {
        const test = remaining.shift();
        if (!test) {
            return;
        }

        // if (test.title !== "Capture all if object property is invoked, and it uses this.") {
        //     continue;
        // }

        it(test.title, asyncTest(async () => {
            // Run pre-actions.
            if (test.pre) {
                test.pre();
            }

            // Invoke the test case.
            if (test.expectText) {
                const text = await runtime.serializeFunctionAsync(test.func);
                assert.equal(text, test.expectText);
            } else {
                await assertAsyncThrows(async () => {
                    await runtime.serializeFunctionAsync(test.func);
                });
            }
        }));

        // Schedule any additional tests.
        if (test.afters) {
            remaining = test.afters.concat(remaining);
        }
    }
});
