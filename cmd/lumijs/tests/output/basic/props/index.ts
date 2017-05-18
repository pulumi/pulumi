// This tests that class and module properties are emitted with correctly qualified token names.

// First, define some properties:

let modprop: string = "Foo";

class C {
    public static clastaprop: number = 42;
    public claprop: boolean = true;
    public get custprop(): string {
        return "getting a custom property";
    }
    public set custprop(v: string) {
    }
}

class D extends C {
    public cladprop: string = "yeah d!";
}

// Now create some references to those properties:

let a: string = modprop;
let b: number = C.clastaprop;
let c = new C();
if (c !== undefined) {
    let d: boolean = c.claprop;
    let e = {
        f: modprop,
        g: C.clastaprop,
        h: c.claprop,
        "i": "i",
        [j()]: "j",
    };
    let cust: string = c.custprop;
    c.custprop = "setting a custom property";
}

function j(): string { return "j"; }

// Define a local variable at the module's top-level within a block (should not be a module member).
{
    let notprop: string = "notprop";
    let notpropcop: string = notprop;
}

