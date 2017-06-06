// This tests out some good and bad cast cases.

let a: string = "x";
let b: any = <any>a; // ok.

// IDEA: a way to baseline expected failures.
// let c: number = <number>a; // statically rejected.

// IDEA: a way to baseline expected runtime failures.
// let d: number = <number>b; // dynamically rejected.

class C {}
let c: any = new C();
let isc: boolean = (c instanceof C);

