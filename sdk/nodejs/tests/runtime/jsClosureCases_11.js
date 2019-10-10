"use strict";
// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

const cases = [];

{
  class C {
    toString() { return "x"; }
  }

  const proxy = new Proxy(C, {
    apply(Target, thisArg, args) {
      return new Target(...args);
    },
    construct(Target, args) {
      return new Target(...args);
    },
    get(target, p) {
      return target[p];
    }
  })

  cases.push({
    title: "Proxied class",
    // tslint:disable-next-line
    func: function () { return proxy; },
    expectText: ` `,
  });
}

{
  class C {
    static toString() { return "y"; }
  }

  const proxy = new Proxy(C, {
    apply(Target, thisArg, args) {
      return new Target(...args);
    },
    construct(Target, args) {
      return new Target(...args);
    },
    get(target, p) {
      return target[p];
    }
  })

  cases.push({
    title: "Proxied class with static toString",
    // tslint:disable-next-line
    func: function () { return proxy; },
    expectText: ` `,
  });
}

module.exports.cases = cases;
