// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { asyncTest, assertAsyncThrows } from "../util";
import { runtime } from "../../index";

interface ClosureCase {
    title: string;            // a title banner for the test case.
    func: Function;           // the function whose body and closure to serialize.
    expect?: runtime.Closure; // if undefined, error expected; otherwise, the serialized shape.
    expectText?: string;      // optionally also validate the serialization to JavaScript text.
    closureHash?: string;      // hash of the closure.
}

// This group of tests ensure that we serialize closures properly.
describe("closure", () => {
    describe("hash", () => {
        it("is affected by code.", () => {
            let closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { }
            };

            let closure2: runtime.Closure = {
                code: "1",
                runtime: "",
                environment: { }
            };

            let hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            let hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });

        it("is affected by runtime.", () => {
            let closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { }
            };

            let closure2: runtime.Closure = {
                code: "",
                runtime: "1",
                environment: { }
            };

            let hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            let hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });

        it("is affected by module.", () => {
            let closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { module: "m1" } }
            };

            let closure2: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { module: "m2" } }
            };

            let hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            let hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });

        it("is affected by environment values.", () => {
            let closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { }
            };

            let closure2: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { json: 100 } }
            };

            let hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            let hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });

        it("is affected by environment names.", () => {
            let closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { json: 100 } }
            };

            let closure2: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap2: { json: 100 } }
            };

            let hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            let hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });

        it("is not affected by environment order.", () => {
            let closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { json: 100 }, cap2: { json: 200 } }
            };

            let closure2: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap2: { json: 200 }, cap1: { json: 100 } }
            };

            let hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            let hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.equal(hash1, hash2);
        });

        it("is different with cyclic and non-cyclic environments.", () => {
            let closure1: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { json: 100 } }
            };
            closure1.environment.cap1.closure = closure1;

            let closure2: runtime.Closure = {
                code: "",
                runtime: "",
                environment: { cap1: { json: 100 } }
            };

            let hash1 = runtime.getClosureHash_forTestingPurposes(closure1);
            let hash2 = runtime.getClosureHash_forTestingPurposes(closure2);
            assert.notEqual(hash1, hash2);
        });
    });

    let cases: ClosureCase[] = [];

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
  var _this;
  with({  }) {
    return (function() {

return (function () { })

    }).apply(_this).apply(this, arguments);
  }
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
        let wcap = "foo";
        let xcap = 97;
        let ycap = [ true, -1, "yup" ];
        let zcap = {
            a: "a",
            b: false,
            c: [ 0 ],
        };
        cases.push({
            title: "Serializes basic captures",
            // tslint:disable-next-line
            func: () => { console.log(wcap + `${xcap}` + ycap.length + eval(zcap.a)); },
            expect: {
                code: "(() => { console.log(wcap + `${xcap}` + ycap.length + eval(zcap.a)); })",
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
            closureHash: "__a07cae0afeaeddbb97b9f7a372b75aafd3b29d0e",
        });
    }
    {
        let nocap1 = 1, nocap2 = 2, nocap3 = 3, nocap4 = 4, nocap5 = 5, nocap6 = 6, nocap7 = 7,
            nocap8 = 8, nocap9 = 9, nocap10 = 10;
        let cap1 = 100, cap2 = 200, cap3 = 300, cap4 = 400;
        let functext = `((nocap1, nocap2) => {
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
                },
                runtime: "nodejs",
            },
            closureHash: "__2806dcb0e9b815d3ada9417edbbc7a95438196b3",
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
        let os = require("os");
        cases.push({
            title: "Capture built-in modules as stable references, not serialized values",
            func: () => os,
            expect: {
                code: `(() => os)`,
                environment: {
                    os: {
                        module: "os",
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__3fa97b166e39ae989158bb37acfa12c7abc25b53",
        });
    }
    {
        let util = require("../util");
        cases.push({
            title: "Capture user-defined modules as stable references, not serialized values",
            func: () => util,
            expect: {
                code: `(() => util)`,
                environment: {
                    util: {
                        module: "./bin/tests/util.js",
                    },
                },
                runtime: "nodejs",
            },
            closureHash: "__cd171f28483c78d2a63bdda674a8f577dd4b41db",
        });
    }
    cases.push({
        title: "Don't capture catch variables",
        // tslint:disable-next-line
        func: eval(`() => { try { } catch (err) { console.log(err); } }`),
        expect: {
            code: `(() => { try { } catch (err) { console.log(err); } })`,
            environment: {},
            runtime: "nodejs",
        },
        closureHash: "__6b8e43947115e731ff7808be1ff6bf9b18aaa67d",
    });

    // Recursive function serialization.
    {
        let fff = "fff!";
        let ggg = "ggg!";
        let xcap = {
            fff: function () { console.log(fff); },
            ggg: () => { console.log(ggg); },
            zzz: {
                a: [ (a1: any, a2: any) => { console.log(a1 + a2); } ],
            },
        };
        let functext = `(() => {
    xcap.fff();
    xcap.ggg();
    xcap.zzz.a[0]("x", "y");
})`;
        cases.push({
            title: "Serializes recursive function captures",
            // tslint:disable-next-line
            func: eval(functext),
            expect: {
                code: functext,
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
            closureHash: "__53324dfdeb155ad763635ace1384fa8e1d397e72",
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
        let cap: any = new CapCap();
        let env: runtime.Environment = { "this": {} };
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

    // Now go ahead and run the test cases, each as its own case.
    for (let test of cases) {
        it(test.title, asyncTest(async () => {
            if (test.expect) {
                let closure: runtime.Closure = await runtime.serializeClosure(test.func);
                assert.deepEqual(closure, test.expect);
                if (test.expectText) {
                    let text = runtime.serializeJavaScriptText(closure);
                    assert.equal(text, test.expectText);
                }

                let closureHash = runtime.getClosureHash_forTestingPurposes(closure);
                assert.equal(closureHash, test.closureHash);
            } else {
                await assertAsyncThrows(async () => {
                    await runtime.serializeClosure(test.func);
                });
            }
        }));
    }
});

