"use strict";
// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

const cases = [];
{
    const zeroBigInt = 0n;
    const smallBigInt = 1n;
    const negativeBigInt = -1n;
    const largeBigInt = 11111111111111111111111111111111111111111n;
    const negativeLargeBigInt = -11111111111111111111111111111111111111111n;

    cases.push({
        title: "Captures bigint",
        // eslint-disable-next-line
        func: function () { console.log(zeroBigInt + smallBigInt + negativeBigInt + largeBigInt + negativeBigInt + negativeLargeBigInt); },
        expectText: `exports.handler = __f0;

function __f0() {
  return (function() {
    with({ zeroBigInt: 0n, smallBigInt: 1n, negativeBigInt: -1n, largeBigInt: 11111111111111111111111111111111111111111n, negativeLargeBigInt: -11111111111111111111111111111111111111111n }) {

return function () { console.log(zeroBigInt + smallBigInt + negativeBigInt + largeBigInt + negativeBigInt + negativeLargeBigInt); };

    }
  }).apply(undefined, undefined).apply(this, arguments);
}
`,
    });
}

module.exports.cases = cases;
