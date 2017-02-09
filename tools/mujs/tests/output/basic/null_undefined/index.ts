// This test just ensures that undefined expands correctly.

let au = undefined;
let nu: number = undefined;
let su: string = undefined;

function f(x: string | undefined) {
    // Intentionally blank.
}

f(undefined);

let an = null;
let nn: number = null;
let sn: string = null;

function g(x: string | null) {
    // Intentionally blank.
}

g(null);

class C {
    pu: string | undefined;
    pn: number | null;
    constructor() {
        this.pu = undefined;
        this.pn = null;
    }
}

