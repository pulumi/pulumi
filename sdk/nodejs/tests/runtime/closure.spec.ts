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
    expect?: runtime.Closure; // if undefined, error expected; otherwise, the serialized shape.
    expectText?: string;      // optionally also validate the serialization to JavaScript text.
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

        it("is affected by dependency.", () => {
            const closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { json: 100 } },
            };

            const closure2: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { dep: { json: 100 } } },
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

    // A few simple positive cases for functions/arrows (no captures).
    cases.push({
        title: "Empty function closure",
        // tslint:disable-next-line
        func: function () { },
        expect: {
            code: "(function () { })",
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__2b3ba3b4fb55b6fb500f9e8d7a4e132cec103fe6",
        expectText: `exports.handler = __2b3ba3b4fb55b6fb500f9e8d7a4e132cec103fe6;

function __2b3ba3b4fb55b6fb500f9e8d7a4e132cec103fe6() {
  return (function() {
    with({  }) {

return (function () { })

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
            code: "(function () { console.log(this); })",
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__cd737a7b5f0ddfaee797a6ff6c8b266051f1c30e",
        expectText: `exports.handler = __cd737a7b5f0ddfaee797a6ff6c8b266051f1c30e;

function __cd737a7b5f0ddfaee797a6ff6c8b266051f1c30e() {
  return (function() {
    with({  }) {

return (function () { console.log(this); })

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
            code: "(function () { console.log(this + arguments); })",
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__05437ec790248221e1167f1da8e9a9ffbfe11ebf",
        expectText: `exports.handler = __05437ec790248221e1167f1da8e9a9ffbfe11ebf;

function __05437ec790248221e1167f1da8e9a9ffbfe11ebf() {
  return (function() {
    with({  }) {

return (function () { console.log(this + arguments); })

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
            code: "(() => { })",
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__b135b11756da3f7aecaaa23a36898c0d6d2845ab",
        expectText: `exports.handler = __b135b11756da3f7aecaaa23a36898c0d6d2845ab;

function __b135b11756da3f7aecaaa23a36898c0d6d2845ab() {
  return (function() {
    with({  }) {

return (() => { })

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

`,
    });

    cases.push({
        title: "Arrow closure with this capture",
        // tslint:disable-next-line
        func: () => { console.log(this); },
    });

    const awaiterClosure = {
        closure: {
            code: "(function (thisArg, _arguments, P, generator) {\n    return new (P || (P = Promise))(function (resolve, reject) {\n        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }\n        function rejected(value) { try { step(generator[\"throw\"](value)); } catch (e) { reject(e); } }\n        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }\n        step((generator = generator.apply(thisArg, _arguments || [])).next());\n    });\n})",
            environment: {},
            runtime: "nodejs",
        },
    };

    const awaiterCode =
`
function __492fe142c8be132f2ccfdc443ed720d77b1ef3a6() {
  return (function() {
    with({  }) {

return (function (thisArg, _arguments, P, generator) {
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
})

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`;

    cases.push({
        title: "Async lambda that does not capture this",
        // tslint:disable-next-line
        func: async () => { },
        expect: {
            code: "(() => __awaiter(this, void 0, void 0, function* () { }))",
            environment: { "__awaiter": awaiterClosure },
            runtime: "nodejs",
        },
        closureHash: "__2a83dcc4e3c79da00ade608e1401449fd97f37fe",
        expectText: `exports.handler = __2a83dcc4e3c79da00ade608e1401449fd97f37fe;

function __2a83dcc4e3c79da00ade608e1401449fd97f37fe() {
  return (function() {
    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {

return (() => __awaiter(this, void 0, void 0, function* () { }))

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
${awaiterCode}
`,
    });

    cases.push({
        title: "Async lambda that does capture this",
        // tslint:disable-next-line
        func: async () => { console.log(this); },
    });

    cases.push({
        title: "Async function that does not capture this",
        // tslint:disable-next-line
        func: async function() { },
        expect: {
            code: "(function () {\n            return __awaiter(this, void 0, void 0, function* () { });\n        })",
            environment: { "__awaiter": awaiterClosure },
            runtime: "nodejs",
        },
        closureHash: "__777fc5424c69bbec55be2ab6c25c4f5aac7b80e6",
        expectText: `exports.handler = __777fc5424c69bbec55be2ab6c25c4f5aac7b80e6;

function __777fc5424c69bbec55be2ab6c25c4f5aac7b80e6() {
  return (function() {
    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {

return (function () {
            return __awaiter(this, void 0, void 0, function* () { });
        })

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
${awaiterCode}
`,
    });

    cases.push({
        title: "Async function that does capture this",
        // tslint:disable-next-line
        func: async function () { console.log(this); },
        expect: {
            code: "(function () {\n            return __awaiter(this, void 0, void 0, function* () { console.log(this); });\n        })",
            environment: { "__awaiter": awaiterClosure },
            runtime: "nodejs",
        },
        closureHash: "__7bddcde28730579e85ca0d9e450a65cad476232c",
        expectText: `exports.handler = __7bddcde28730579e85ca0d9e450a65cad476232c;

function __7bddcde28730579e85ca0d9e450a65cad476232c() {
  return (function() {
    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {

return (function () {
            return __awaiter(this, void 0, void 0, function* () { console.log(this); });
        })

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
${awaiterCode}
`,
    });

    cases.push({
        title: "Arrow closure with this and arguments capture",
        // tslint:disable-next-line
        func: (function() { return () => { console.log(this + arguments); } }).apply(this, [0, 1]),
    });

    cases.push({
        title: "Arrow closure with this capture inside function closure",
        // tslint:disable-next-line
        func: function () { () => { console.log(this); } },
        expect: {
            code: "(function () { () => { console.log(this); }; })",
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__6668edd6db8c98baacaf1a227150aa18ce2ae872",
        expectText: `exports.handler = __6668edd6db8c98baacaf1a227150aa18ce2ae872;

function __6668edd6db8c98baacaf1a227150aa18ce2ae872() {
  return (function() {
    with({  }) {

return (function () { () => { console.log(this); }; })

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
            code: "(function () { () => { console.log(this + arguments); }; })",
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__de8ce937834140441c7413a7e97b67bda12d7205",
        expectText: `exports.handler = __de8ce937834140441c7413a7e97b67bda12d7205;

function __de8ce937834140441c7413a7e97b67bda12d7205() {
  return (function() {
    with({  }) {

return (function () { () => { console.log(this + arguments); }; })

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
            code: "(function (x, y, z) { })",
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__e680605f156fcaa89016e23c51d3e2328602ebad",
    });

    cases.push({
        title: "Empty arrow closure w/ args",
        // tslint:disable-next-line
        func: (x: any, y: any, z: any) => { },
        expect: {
            code: "((x, y, z) => { })",
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__dd08d1034bd5f0e06f1269cb79974a636ef9cb13",
    });

    // Serialize captures.
    cases.push({
        title: "Doesn't serialize global captures",
        func: () => { console.log("Just a global object reference"); },
        expect: {
            code: `(() => { console.log("Just a global object reference"); })`,
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__47ac0033692c3101b014a1a3c17a4318cf7d4330",
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
            expect: {
                code: "(() => { console.log(wcap + `${xcap}` + ycap.length + eval(zcap.a + zcap.b + zcap.c)); })",
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
            closureHash: "__2b40784194096bd63df0a9de27efcd35f2fefbca",
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

        const functext = `((nocap1, nocap2) => {
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
})`;
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
            closureHash: "__f919744848c6471a841a1de62afe7d3d7f7f208a",
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
                code: `(() => {
                // cap1 is captured here.
                // nocap1 introduces a new variable that shadows the outer one.
                // tslint:disable-next-line
                let [nocap1 = cap1] = [];
                console.log(nocap1);
            })`,
                environment: {
                    cap1: { json: 100 },
                },
                runtime: "nodejs",
            },
            closureHash: "__ef48a2e2962bd53acef1b2cda244ae8c72972c05",
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
                code: `(() => {
                // cap1 is captured here.
                // nocap1 introduces a new variable that shadows the outer one.
                // tslint:disable-next-line
                let { nocap1 = cap1 } = {};
                console.log(nocap1);
            })`,
                environment: {
                    cap1: { json: 100 },
                },
                runtime: "nodejs",
            },
            closureHash: "__b409f3bd837d513df07525bef43e57597154625e",
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
                code: `(() => {
                // cap1 is captured here.
                // nocap1 introduces a new variable that shadows the outer one.
                // tslint:disable-next-line
                let { x: nocap1 = cap1 } = {};
                console.log(nocap1);
            })`,
                environment: {
                    cap1: { json: 100 },
                },
                runtime: "nodejs",
            },
            closureHash: "__5fa215795194604118a7543ce20b8e273837ae79",
        });
    }

    cases.push({
        title: "Don't capture built-ins",
        // tslint:disable-next-line
        func: () => { let x: any = eval("undefined + null + NaN + Infinity + __filename"); require("os"); },
        expect: {
            code: `(() => { let x = eval("undefined + null + NaN + Infinity + __filename"); require("os"); })`,
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__fa1c10acee8dd79b39d0f8109d2bc3252b19619a",
    });

    {
        const os = require("os");
        cases.push({
            title: "Fail when capturing built-in modules.",
            func: () => os,
        });
    }

    {
        cases.push({
            title: "Allow 'require' of built-in module",
            func: () => { const v = require("os"); },
            expect: {
                code: "(() => { const v = require(\"os\"); })",
                environment: {},
                runtime: "nodejs",
            },
            closureHash: "__462b48742cacdee6a332b9767a5dbb08fe2af386",
            expectText: `exports.handler = __462b48742cacdee6a332b9767a5dbb08fe2af386;

function __462b48742cacdee6a332b9767a5dbb08fe2af386() {
  return (function() {
    with({  }) {

return (() => { const v = require(\"os\"); })

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

`,
        });
    }

    {
        cases.push({
            title: "Allow 'async import' of built-in module",
            func: async () => { const v = await import("os"); },
            expect: {
                code: "(() => __awaiter(this, void 0, void 0, function* () { const v = yield Promise.resolve().then(() => require(\"os\")); }))",
                environment: { "__awaiter": awaiterClosure },
                runtime: "nodejs",
            },
            closureHash: "__ed2e0bd0fd941495843361acb848c827f83f4b37",
            expectText: `exports.handler = __ed2e0bd0fd941495843361acb848c827f83f4b37;

function __ed2e0bd0fd941495843361acb848c827f83f4b37() {
  return (function() {
    with({ __awaiter: __492fe142c8be132f2ccfdc443ed720d77b1ef3a6 }) {

return (() => __awaiter(this, void 0, void 0, function* () { const v = yield Promise.resolve().then(() => require(\"os\")); }))

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
${awaiterCode}
`,
        });
    }

    {
        const util = require("../util");
        cases.push({
            title: "Fail when capturing user module.",
            func: () => util,
        });
    }

    cases.push({
        title: "Don't capture catch variables",
        // tslint:disable-next-line
        func: () => { try { } catch (err) { console.log(err); } },
        expect: {
            code:
`(() => { try { }
        catch (err) {
            console.log(err);
        } })`,
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__040426f0dc90fa8f115c1a7ed52793515564fa98",
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
                code: `(() => {
            xcap.fff();
            xcap.ggg();
            xcap.zzz.a[0]("x", "y");
        })`,
                environment: {
                    xcap: {
                        obj: {
                            fff: {
                                closure: {
                                    code: "(function () { console.log(fff); })",
                                    environment: { fff: { json: "fff!" } },
                                    runtime: "nodejs",
                                },
                            },
                            ggg: {
                                closure: {
                                    code: "(() => { console.log(ggg); })",
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
                                                    code: "((a1, a2) => { console.log(a1 + a2); })",
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
            closureHash: "__1b6ae6abb4d8b2676bccb50507a0276f5be90371",
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
                    code: "(() => { console.log(this.x); })",
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
                code: "(() => { console.log(this.x); })",
                environment: env,
                runtime: "nodejs",
            },
            closureHash: "__8d564176f3cd517bfe3c6e9d6b4da488a1198c0d",
        });
    }

    cases.push({
        title: "Don't serialize `this` in function expressions",
        func: function() { return this; },
        expect: {
            code: `(function () { return this; })`,
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__05dabc231611ca558334d59d661ebfb242b31b5d",
    });

    const mutable: any = {};
    cases.push({
        title: "Serialize mutable objects by value at the time of capture (pre-mutation)",
        func: function() { return mutable; },
        expect: {
            code: `(function () { return mutable; })`,
            environment: {
                "mutable": {
                    obj: {},
                },
            },
            runtime: "nodejs",
        },
        closureHash: "__e4a4f2f9ad40ef73c250aa10f0277247adfae473",
        afters: [{
            pre: () => { mutable.timesTheyAreAChangin = true; },
            title: "Serialize mutable objects by value at the time of capture (post-mutation)",
            func: function() { return mutable; },
            expect: {
                code: `(function () { return mutable; })`,
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
            closureHash: "__18d08ca03253fe3dda134c1e5e5889f514cb3841",
        }],
    });

    {
        const v = { d: output(4) };
        cases.push({
            title: "Output capture",
            // tslint:disable-next-line
            func: function () { console.log(v); },
            expect: {
                code: "(function () { console.log(v); })",
                environment: {
                    "v": { "obj": { "d": { "dep": { "json": 4 } } } },
                },
                runtime: "nodejs",
            },
            closureHash: "__7e25d8245194b5762701b759324b82b2661d8df7",
            expectText: `exports.handler = __7e25d8245194b5762701b759324b82b2661d8df7;

function __7e25d8245194b5762701b759324b82b2661d8df7() {
  return (function() {
    with({ v: { d: { get: () => 4 } } }) {

return (function () { console.log(v); })

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
            expect: {
                code: "(function () { console.log(v); })",
                environment: {
                    "v": { "obj": {
                        "d1": { "dep": { "json": 4 } },
                        "d2": { "dep": { "json": "str" } },
                        "d3": { "dep": { "json": undefined } },
                        "d4": { "dep": { "obj": {
                            "a": { "json": 1 },
                            "b": { "json": true },
                        } } },
                    } },
                },
                runtime: "nodejs",
            },
            closureHash: "__21011f46c5fb3531018421f3247c3a31d97ae710",
            expectText: `exports.handler = __21011f46c5fb3531018421f3247c3a31d97ae710;

function __21011f46c5fb3531018421f3247c3a31d97ae710() {
  return (function() {
    with({ v: { d1: { get: () => 4 }, d2: { get: () => "str" }, d3: { get: () => undefined }, d4: { get: () => ({ a: 1, b: true }) } } }) {

return (function () { console.log(v); })

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
            expect: {
                code: "(function () { console.log(obj); })",
                environment: {
                    "obj": {
                        "obj": {
                            "method1": {
                                "closure": {
                                    "code": "(function method1() { return this.method2(); })",
                                    "environment": {},
                                    "runtime": "nodejs",
                                },
                            },
                            "method2": {
                                "closure": {
                                    "code": "(() => { return; })",
                                    "environment": {},
                                    "runtime": "nodejs",
                                },
                            },
                        },
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__f0b70e2ec196258725e4b2959cb5ec5b89d4c0e4",
            expectText: `exports.handler = __f0b70e2ec196258725e4b2959cb5ec5b89d4c0e4;

function __f0b70e2ec196258725e4b2959cb5ec5b89d4c0e4() {
  return (function() {
    with({ obj: { method1: __f6abbd7cb31cb232a3250be3014cee3b74db4cfa, method2: __d3e9cc89985f25c6465a39781af4eb9e1c3c7c48 } }) {

return (function () { console.log(obj); })

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __f6abbd7cb31cb232a3250be3014cee3b74db4cfa() {
  return (function() {
    with({  }) {

return (function method1() { return this.method2(); })

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __d3e9cc89985f25c6465a39781af4eb9e1c3c7c48() {
  return (function() {
    with({  }) {

return (() => { return; })

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
            expect: {
                code: "(function m() { return this.n(); })",
                environment: { },
                runtime: "nodejs",
            },
            closureHash: "__a5583812bfd698420d9f1872a30b68e59b01ac00",
            expectText: `exports.handler = __a5583812bfd698420d9f1872a30b68e59b01ac00;

function __a5583812bfd698420d9f1872a30b68e59b01ac00() {
  return (function() {
    with({  }) {

return (function m() { return this.n(); })

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
            expect: {
                code: "(function m() { return this.n(); })",
                environment: { },
                runtime: "nodejs",
            },
            closureHash: "__a5583812bfd698420d9f1872a30b68e59b01ac00",
            expectText: `exports.handler = __a5583812bfd698420d9f1872a30b68e59b01ac00;

function __a5583812bfd698420d9f1872a30b68e59b01ac00() {
  return (function() {
    with({  }) {

return (function m() { return this.n(); })

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
            expect: {
                code: "(function () { return this.n(); })",
                environment: { },
                runtime: "nodejs",
            },
            closureHash: "__4388dd82f50083d1b18aa1eb2cebd11363fedeb4",
            expectText: `exports.handler = __4388dd82f50083d1b18aa1eb2cebd11363fedeb4;

function __4388dd82f50083d1b18aa1eb2cebd11363fedeb4() {
  return (function() {
    with({  }) {

return (function () { return this.n(); })

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

`,
        });
    }

    {
        const o = { a: 1, b: 2 };

        cases.push({
            title: "Capture subset of properties #1",
            // tslint:disable-next-line
            func: function () { console.log(o.a); },
            expect: {
                code: "(function () { console.log(o.a); })",
                environment: {
                    "o": {
                        "obj": {
                            "a": {
                                "json": 1,
                            },
                        },
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__ad5da959903654accaa0df52f98d49a7fae967fc",
            expectText: `exports.handler = __ad5da959903654accaa0df52f98d49a7fae967fc;

function __ad5da959903654accaa0df52f98d49a7fae967fc() {
  return (function() {
    with({ o: { a: 1 } }) {

return (function () { console.log(o.a); })

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
            expect: {
                code: "(function () { console.log(o.b + o.c); })",
                environment: {
                    "o": {
                        "obj": {
                            "b": {
                                "json": 2,
                            },
                            "c": {
                                "json": 3,
                            },
                        },
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__7701460608958f498694bca1bd4ec53975f2bfe4",
            expectText: `exports.handler = __7701460608958f498694bca1bd4ec53975f2bfe4;

function __7701460608958f498694bca1bd4ec53975f2bfe4() {
  return (function() {
    with({ o: { b: 2, c: 3 } }) {

return (function () { console.log(o.b + o.c); })

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
            expect: {
                code: "(function () { console.log(o); })",
                environment: {
                    "o": {
                        "obj": {
                            "a": {
                                "json": 1,
                            },
                            "b": {
                                "json": 2,
                            },
                            "c": {
                                "json": 3,
                            },
                        },
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__90bd8b65a06a7bef2baafb94583eaa375fe7d03a",
            expectText: `exports.handler = __90bd8b65a06a7bef2baafb94583eaa375fe7d03a;

function __90bd8b65a06a7bef2baafb94583eaa375fe7d03a() {
  return (function() {
    with({ o: { a: 1, b: 2, c: 3 } }) {

return (function () { console.log(o); })

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

`,
        });
    }

    {
        const o = { a: 1, b: 2, c() { return this; } };

        cases.push({
            title: "Capture all if object property is invoked.",
            // tslint:disable-next-line
            func: function () { console.log(o.c()); },
            expect: {
                code: "(function () { console.log(o.c()); })",
                "environment": {
                    "o": {
                        "obj": {
                            "a": {
                                "json": 1,
                            },
                            "b": {
                                "json": 2,
                            },
                            "c": {
                                "closure": {
                                    "code": "(function c() { return this; })",
                                    "environment": {},
                                    "runtime": "nodejs",
                                },
                            },
                        },
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__f6b8f811f6a389886339c140c7481f8d575c0d8c",
            expectText: `exports.handler = __f6b8f811f6a389886339c140c7481f8d575c0d8c;

function __f6b8f811f6a389886339c140c7481f8d575c0d8c() {
  return (function() {
    with({ o: { a: 1, b: 2, c: __69aafdcc0cf5a97c8eabd2cf1ef97ca6b36a331c } }) {

return (function () { console.log(o.c()); })

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __69aafdcc0cf5a97c8eabd2cf1ef97ca6b36a331c() {
  return (function() {
    with({  }) {

return (function c() { return this; })

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
            expect: {
                code: "(function () { console.log(o.b.c()); })",
                "environment": {
                    "o": {
                        "obj": {
                            "b": {
                                "obj": {
                                    "c": {
                                        "closure": {
                                            "code": "(function c() { return this; })",
                                            "environment": {},
                                            "runtime": "nodejs",
                                        },
                                    },
                                },
                            },
                        },
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__6247ba094163bb6d9af3e536aa06a19bb8474933",
            expectText: `exports.handler = __6247ba094163bb6d9af3e536aa06a19bb8474933;

function __6247ba094163bb6d9af3e536aa06a19bb8474933() {
  return (function() {
    with({ o: { b: { c: __69aafdcc0cf5a97c8eabd2cf1ef97ca6b36a331c } } }) {

return (function () { console.log(o.b.c()); })

    }
  }).apply(undefined, undefined).apply(this, arguments);
}

function __69aafdcc0cf5a97c8eabd2cf1ef97ca6b36a331c() {
  return (function() {
    with({  }) {

return (function c() { return this; })

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
                    const text = runtime.serializeJavaScriptText(closure);
                    assert.equal(text, test.expectText);
                }

                const closureHash = runtime.getClosureHash_forTestingPurposes(closure);
                assert.equal(closureHash, test.closureHash);
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
