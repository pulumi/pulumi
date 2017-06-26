// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// local functions (named and anonymous)
let func = () => {
    let inner = (): number => { return 42; };
    let x = inner();
    function innerFunc() {}
    innerFunc();
    let anonyFunc = function() {};
    anonyFunc();
    let namedFuncExpr = function realNamedFunc(): number { return 99; }
    let z = namedFuncExpr();
};

