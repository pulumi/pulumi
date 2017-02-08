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

        // This `super` should just be an object reference to the base object:
        let y = super.gx();
    }
}

