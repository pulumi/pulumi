// simple function lambdas (empty bodies)
let lamb01 = function() {};
let lamb02 = function(x) {};
let lamb03 = function(x: number) {};
let lamb04 = function(x, y, z) {};
let lamb05 = function(x: number, y: string, z: boolean): void {};
// simple arrow lambdas (empty bodies)
let lamb11 = () => {};
let lamb12 = (x) => {};
let lamb13 = (x: number) => {};
let lamb14 = (x, y, z) => {};
let lamb15 = (x: number, y: string, z: boolean): void => {};
// more function lambdas (non-empty bodies)
let lamb21 = function() {
    if ("foo" === "foo") {
        return 42;
    }
    return 0;
};
let lamb22 = function(x) {
    for (let i = 0; i < x; i++) {
        if (i === x - 1) {
            return 42;
        }
    }
    return 0;
};
let lamb23 = function(x: number): number {
    for (let i = 0; i < x; i++) {
        if (i === x - 1) {
            return 42;
        }
    }
    return 0;
};
let lamb24 = function(x, y, z) {
    for (let i = x; x < y; x += z) {
        if (i === y - 1) {
            return 42;
        }
    }
    return 0;
};
let lamb25 = function(x: number, y: string, z: boolean): string {
    if (z) {
        return y;
    }
    return "foo";
};
// more arrow lambdas (non-empty bodies)
let lamb31 = () => {
    if ("foo" === "foo") {
        return 42;
    }
    return 0;
};
let lamb32 = (x) => {
    for (let i = 0; i < x; i++) {
        if (i === x - 1) {
            return 42;
        }
    }
    return 0;
};
let lamb33 = (x: number): number => {
    for (let i = 0; i < x; i++) {
        if (i === x - 1) {
            return 42;
        }
    }
    return 0;
};
let lamb34 = (x, y, z) => {
    for (let i = x; x < y; x += z) {
        if (i === y - 1) {
            return 42;
        }
    }
    return 0;
};
let lamb35 = (x: number, y: string, z: boolean): string => {
    if (z) {
        return y;
    }
    else if (x < 42) {
        return "foo";
    }
    return "bar";
};

