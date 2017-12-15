// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// tslint:disable:max-line-length

import * as assert from "assert";
import { runtime } from "../../index";
import { assertAsyncThrows, asyncTest } from "../util";

interface ClosureCase {
    pre?: () => void;         // an optional function to run before this case.
    title: string;            // a title banner for the test case.
    func: Function;           // the function whose body and closure to serialize.
    expect?: runtime.Closure; // if undefined, error expected; otherwise, the serialized shape.
    expectText?: string;      // optionally also validate the serialization to JavaScript text.
    ignoreHash?: boolean;     // if hashes should be ignored when comparing results.
    closureHash?: string;     // hash of the closure.
    afters?: ClosureCase[];   // an optional list of test cases to run afterwards.
}

// This group of tests ensure that we serialize closures properly.
describe("closure", () => {
    describe("hash", () => {
        it("is affected by code.", () => {
            const closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { },
            };

            const closure2: runtime.Closure = {
                code: "1",
                runtime: "",
                environment: { },
            };

            const hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            const hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });

        it("is affected by runtime.", () => {
            const closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { },
            };

            const closure2: runtime.Closure = {
                code: "",
                runtime: "1",
                environment: { },
            };

            const hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            const hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });

        it("is affected by module.", () => {
            const closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { module: "m1" } },
            };

            const closure2: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { module: "m2" } },
            };

            const hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            const hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });

        it("is affected by environment values.", () => {
            const closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { },
            };

            const closure2: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { json: 100 } },
            };

            const hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            const hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });

        it("is affected by environment names.", () => {
            const closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { json: 100 } },
            };

            const closure2: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap2: { json: 100 } },
            };

            const hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            const hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });

        it("is not affected by environment order.", () => {
            const closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { json: 100 }, cap2: { json: 200 } },
            };

            const closure2: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap2: { json: 200 }, cap1: { json: 100 } },
            };

            const hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            const hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.equal(hash1, hash2);
        });

        it("is different with cyclic and non-cyclic environments.", () => {
            const closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { json: 100 } },
            };
            closure1.environment.cap1.closure = closure1;

            const closure2: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { json: 100 } },
            };

            const hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            const hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });
    });

    const cases: ClosureCase[] = [];
    {
        // Ensure we reject function declarations.
        class C {
            // tslint:disable-next-line
            public m(): void { }
        }
        cases.push({
            title: "Reject non-expression function objects",
            func: new C().m,
            closureHash: "",
        });
    }
    // A few simple positive cases for functions/arrows (no captures).
    cases.push({
        title: "Empty function closure",
        // tslint:disable-next-line
        func: function () { },
        expect: {
            code: "function () { }",
            environment: {},
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({  }) {
      return (/*<user-code>*/function () { }/*</user-code>*/);
    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });
    cases.push({
        title: "Function closure with this capture",
        // tslint:disable-next-line
        func: function () { console.log(this); },
        expect: {
            code: "function () { console.log(this); }",
            environment: {},
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({  }) {
      return (/*<user-code>*/function () { console.log(this); }/*</user-code>*/);
    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Function closure with this and arguments capture",
        // tslint:disable-next-line
        func: function () { console.log(this + arguments); },
        expect: {
            code: "function () { console.log(this + arguments); }",
            environment: {},
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({  }) {
      return (/*<user-code>*/function () { console.log(this + arguments); }/*</user-code>*/);
    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });
    cases.push({
        title: "Empty arrow closure",
        // tslint:disable-next-line
        func: () => { },
        expect: {
            code: "() => { }",
            environment: {},
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({  }) {
      return (/*<user-code>*/() => { }/*</user-code>*/);
    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });
    cases.push({
        title: "Arrow closure with this capture",
        // tslint:disable-next-line
        func: () => { console.log(this); },
        expect: {
            code: "() => { console.log(this); }",
            environment: { "this": { "module": "./bin/tests/runtime/closure.spec.js" } },
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({  }) {
      return (/*<user-code>*/() => { console.log(this); }/*</user-code>*/);
    }
  }).apply(require("./bin/tests/runtime/closure.spec.js"), undefined).apply(this, arguments);
}
`,
    });

    const awaiterClosure = {
        closure: {
            code: "function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n}",
            environment: {},
            runtime: "nodejs",
        },
    };

    const awaiterCode =
`function __shaHash() {
  return (function() {
    with({  }) {
      return (/*<user-code>*/function (thisArg, _arguments, P, generator) {
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
}/*</user-code>*/);
    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`;

    cases.push({
        title: "Async lambda that does not capture this",
        // tslint:disable-next-line
        func: async () => { },
        expect: {
            code: "() => __awaiter(this, void 0, void 0, function* () { })",
            environment: { "__awaiter": awaiterClosure },
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({ __awaiter: __shaHash }) {
      return (/*<user-code>*/() => __awaiter(this, void 0, void 0, function* () { })/*</user-code>*/);
    }
  }).apply(undefined, undefined).apply(this, arguments);
}
${awaiterCode}`,
    });

    cases.push({
        title: "Async lambda that does capture this",
        // tslint:disable-next-line
        func: async () => { console.log(this); },
        expect: {
            code: "() => __awaiter(this, void 0, void 0, function* () { console.log(this); })",
            environment: {
                "__awaiter": awaiterClosure,
                "this": { "module": "./bin/tests/runtime/closure.spec.js" },
            },
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({ __awaiter: __shaHash }) {
      return (/*<user-code>*/() => __awaiter(this, void 0, void 0, function* () { console.log(this); })/*</user-code>*/);
    }
  }).apply(require("./bin/tests/runtime/closure.spec.js"), undefined).apply(this, arguments);
}
${awaiterCode}`,
    });

    cases.push({
        title: "Async function that does not capture this",
        // tslint:disable-next-line
        func: async function() { },
        expect: {
            code: "function () {\n            return __awaiter(this, void 0, void 0, function* () { });\n        }",
            environment: { "__awaiter": awaiterClosure },
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({ __awaiter: __shaHash }) {
      return (/*<user-code>*/function () {
            return __awaiter(this, void 0, void 0, function* () { });
        }/*</user-code>*/);
    }
  }).apply(undefined, undefined).apply(this, arguments);
}
${awaiterCode}`,
    });

    cases.push({
        title: "Async function that does capture this",
        // tslint:disable-next-line
        func: async function () { console.log(this); },
        expect: {
            code: "function () {\n            return __awaiter(this, void 0, void 0, function* () { console.log(this); });\n        }",
            environment: { "__awaiter": awaiterClosure },
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({ __awaiter: __shaHash }) {
      return (/*<user-code>*/function () {
            return __awaiter(this, void 0, void 0, function* () { console.log(this); });
        }/*</user-code>*/);
    }
  }).apply(undefined, undefined).apply(this, arguments);
}
${awaiterCode}`,
    });

    cases.push({
        title: "Arrow closure with this and arguments capture",
        // tslint:disable-next-line
        func: (function() { return () => { console.log(this + arguments); } }).apply(this, [0, 1]),
        expect: {
            code: "() => { console.log(this + arguments); }",
            environment: {
                this: { module: "./bin/tests/runtime/closure.spec.js" },
                arguments: { arr: [{ json: 0 }, { json: 1 }] },
            },
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({  }) {
      return (/*<user-code>*/() => { console.log(this + arguments); }/*</user-code>*/);
    }
  }).apply(require("./bin/tests/runtime/closure.spec.js"), [ 0, 1 ]).apply(this, arguments);
}
`,
    });

    cases.push({
        title: "Arrow closure with this capture inside function closure",
        // tslint:disable-next-line
        func: function () { () => { console.log(this); } },
        expect: {
            code: "function () { () => { console.log(this); }; }",
            environment: {},
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({  }) {
      return (/*<user-code>*/function () { () => { console.log(this); }; }/*</user-code>*/);
    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });
    cases.push({
        title: "Arrow closure with this and arguments capture inside function closure",
        // tslint:disable-next-line
        func: function () { () => { console.log(this + arguments); } },
        expect: {
            code: "function () { () => { console.log(this + arguments); }; }",
            environment: {},
            runtime: "nodejs",
        },
        ignoreHash: true,
        expectText: `exports.handler = __shaHash;

function __shaHash() {
  return (function() {
    with({  }) {
      return (/*<user-code>*/function () { () => { console.log(this + arguments); }; }/*</user-code>*/);
    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });
    cases.push({
        title: "Empty function closure w/ args",
        // tslint:disable-next-line
        func: function (x: any, y: any, z: any) { },
        expect: {
            code: "function (x, y, z) { }",
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__9384cc259e35918caeec847fa45296ee2874df24",
    });
    cases.push({
        title: "Empty arrow closure w/ args",
        // tslint:disable-next-line
        func: (x: any, y: any, z: any) => { },
        expect: {
            code: "(x, y, z) => { }",
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__1437d72874b6fcaa5499df8b5c81ccbfd2e430ab",
    });

    // Serialize captures.
    cases.push({
        title: "Doesn't serialize global captures",
        func: () => { console.log("Just a global object reference"); },
        expect: {
            code: `() => { console.log("Just a global object reference"); }`,
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__5dd8f7cd0bbeb697618809452273579658362777",
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
            func: () => { console.log(wcap + `${xcap}` + ycap.length + eval(zcap.a)); },
            expect: {
                code: "() => { console.log(wcap + `${xcap}` + ycap.length + eval(zcap.a)); }",
                environment: {
                    wcap: {
                        json: "foo",
                    },
                    xcap: {
                        json: 97,
                    },
                    ycap: {
                        arr: [
                            { json: true },
                            { json: -1 },
                            { json: "yup" },
                        ],
                    },
                    zcap: {
                        obj: {
                            a: { json: "a" },
                            b: { json: false },
                            c: { arr: [ { json: 0 } ] },
                        },
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__4084c64fe93bf10279b32b41c71b066b4f2d7c3a",
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
            expect: {
                code: functext,
                environment: {
                    cap1: { json: 100 },
                    cap2: { json: 200 },
                    cap3: { json: 300 },
                    cap4: { json: 400 },
                    cap5: { json: 500 },
                    cap6: { json: 600 },
                    cap7: { json: 700 },
                    cap8: { json: 800 },
                },
                runtime: "nodejs",
            },
            closureHash: "__e5d35647927d115f910962f109cdca8daf4f3784",
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
            expect: {
                code: `() => {
                // cap1 is captured here.
                // nocap1 introduces a new variable that shadows the outer one.
                // tslint:disable-next-line
                let [nocap1 = cap1] = [];
                console.log(nocap1);
            }`,
                environment: {
                    cap1: { json: 100 },
                },
                runtime: "nodejs",
            },
            closureHash: "__1d04ab67a5f91f7bf6704a01ff5eeaeead5fe1e7",
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
            expect: {
                code: `() => {
                // cap1 is captured here.
                // nocap1 introduces a new variable that shadows the outer one.
                // tslint:disable-next-line
                let { nocap1 = cap1 } = {};
                console.log(nocap1);
            }`,
                environment: {
                    cap1: { json: 100 },
                },
                runtime: "nodejs",
            },
            closureHash: "__9046e9c2b90fc20b45a5aa3f0af3c5417f93944d",
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
            expect: {
                code: `() => {
                // cap1 is captured here.
                // nocap1 introduces a new variable that shadows the outer one.
                // tslint:disable-next-line
                let { x: nocap1 = cap1 } = {};
                console.log(nocap1);
            }`,
                environment: {
                    cap1: { json: 100 },
                },
                runtime: "nodejs",
            },
            closureHash: "__08a2fdc6059f41d90800a41d9ebcfccb659cd37d",
        });
    }
    cases.push({
        title: "Don't capture built-ins",
        // tslint:disable-next-line
        func: () => { let x: any = eval("undefined + null + NaN + Infinity + __filename"); require("os"); },
        expect: {
            code: `() => { let x = eval("undefined + null + NaN + Infinity + __filename"); require("os"); }`,
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__4506a072e05b10a02fb44f539e774583b29a6b00",
    });
    {
        const os = require("os");
        cases.push({
            title: "Capture built-in modules as stable references, not serialized values",
            func: () => os,
            expect: {
                code: `() => os`,
                environment: {
                    os: {
                        module: "os",
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__44456d9439502b912741dfe1365e842bc18f4fe4",
        });
    }
    {
        const util = require("../util");
        cases.push({
            title: "Capture user-defined modules as stable references, not serialized values",
            func: () => util,
            expect: {
                code: `() => util`,
                environment: {
                    util: {
                        module: "./bin/tests/util.js",
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__5980c5e8823024941cca18e35cd1832fb1296f62",
        });
    }
    cases.push({
        title: "Don't capture catch variables",
        // tslint:disable-next-line
        func: () => { try { } catch (err) { console.log(err); } },
        expect: {
            code:
`() => { try { }
        catch (err) {
            console.log(err);
        } }`,
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__acde9340a4e7ee0cfba6a21dd4b0f8ac01e7658b",
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
            expect: {
                code: `() => {
            xcap.fff();
            xcap.ggg();
            xcap.zzz.a[0]("x", "y");
        }`,
                environment: {
                    xcap: {
                        obj: {
                            fff: {
                                closure: {
                                    code: "function () { console.log(fff); }",
                                    environment: { fff: { json: "fff!" } },
                                    runtime: "nodejs",
                                },
                            },
                            ggg: {
                                closure: {
                                    code: "() => { console.log(ggg); }",
                                    environment: { ggg: { json: "ggg!" } },
                                    runtime: "nodejs",
                                },
                            },
                            zzz: {
                                obj: {
                                    a: {
                                        arr: [
                                            {
                                                closure: {
                                                    code: "(a1, a2) => { console.log(a1 + a2); }",
                                                    environment: {},
                                                    runtime: "nodejs",
                                                },
                                            },
                                        ],
                                    },
                                },
                            },
                        },
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__6753e0a4e4d2966ce4953a123268e3bdff260471",
        });
    }

    {
        class CapCap {
            constructor() {
                (<any>this).x = 42;
                (<any>this).f = () => { console.log((<any>this).x); };
            }
        }

        // Closing over 'this'.  This yields a circular closure.
        const cap: any = new CapCap();
        const env: runtime.Environment = { "this": {} };
        env["this"].obj = {
            f: {
                closure: {
                    code: "() => { console.log(this.x); }",
                    environment: {
                        "this": env["this"],
                    },
                    runtime: "nodejs",
                },
            },
            x: {
                json: 42,
            },
        };
        cases.push({
            title: "Serializes `this` capturing arrow functions",
            func: cap.f,
            expect: {
                code: "() => { console.log(this.x); }",
                environment: env,
                runtime: "nodejs",
            },
            closureHash: "__eb5a24be2a79b30be759e50960640ca84476039b",
        });
    }
    cases.push({
        title: "Don't serialize `this` in function expressions",
        func: function() { return this; },
        expect: {
            code: `function () { return this; }`,
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__84add4af6a910e8aea236c946f7ec750ebe524e5",
    });
    const mutable: any = {};
    cases.push({
        title: "Serialize mutable objects by value at the time of capture (pre-mutation)",
        func: function() { return mutable; },
        expect: {
            code: `function () { return mutable; }`,
            environment: {
                "mutable": {
                    obj: {},
                },
            },
            runtime: "nodejs",
        },
        closureHash: "__4aefe580cfb45e0c3e24306ef1e33826d0e84b94",
        afters: [{
            pre: () => { mutable.timesTheyAreAChangin = true; },
            title: "Serialize mutable objects by value at the time of capture (post-mutation)",
            func: function() { return mutable; },
            expect: {
                code: `function () { return mutable; }`,
                environment: {
                    "mutable": {
                        obj: {
                            "timesTheyAreAChangin": {
                                json: true,
                            },
                        },
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__d8183142416f706793912f70c05852c54dc31c68",
        }],
    });

    // Make a callback to keep running tests.
    let remaining = cases;
    while (true) {
        const test = remaining.shift();
        if (!test) {
            return;
        }
        it(test.title, asyncTest(async () => {
            // Run pre-actions.
            if (test.pre) {
                test.pre();
            }

            // Invoke the test case.
            if (test.expect) {
                const closure: runtime.Closure = await runtime.serializeClosure(test.func);
                assert.deepEqual(closure, test.expect);
                if (test.expectText) {
                    let text = runtime.serializeJavaScriptText(closure);

                    if (test.ignoreHash) {
                        text = text.replace(/__[a-zA-Z0-9]{40}/g, "__shaHash");
                    }

                    assert.equal(text, test.expectText);
                }

                if (test.closureHash) {
                    const closureHash = runtime.getClosureHash_forTestingPurposes(closure);
                    assert.equal(closureHash, test.closureHash);
                }
            } else {
                await assertAsyncThrows(async () => {
                    await runtime.serializeClosure(test.func);
                });
            }
        }));

        // Schedule any additional tests.
        if (test.afters) {
            remaining = test.afters.concat(remaining);
        }
    }
});
