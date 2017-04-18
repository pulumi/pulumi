// This test ensures that references to `super` (base class) are resolved and emitted correctly.

class B {
    private x: number;

    constructor(x: number) {
        this.x = x;
    }

    public gx(): number {
        return this.x;
    }
}

class C extends B {
    constructor() {
        // This `super` should resolve to the constructor function:
        super(42);
    }

    public gy(): number {
        // This `super` should just be an object reference to the base object:
        let y = super.gx();
        return y;
    }
}

let b = new B(18);
let bgx = b.gx();
if (bgx !== 18) {
    throw new Error("Expected b.gx == 18; got " + bgx);
}

let c = new C();
let cgx = c.gx();
if (cgx !== 42) {
    throw new Error("Expected c.gx == 42; got " + cgx);
}
let cgy = c.gy();
if (cgy !== 42) {
    throw new Error("Expecred c.gy == 42; got " + cgy);
}

