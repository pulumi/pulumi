"use strict";
// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

const cases = [];
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


module.exports.cases = cases;

