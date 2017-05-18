// This tests intra-module type references.  They should be emitted with a fully resolved module name, even though the
// referenced type isn't exported.

interface A { }
class B { }

class C {
    constructor(a: A, b: B) {
        // Intentionally left empty.
    }
}

