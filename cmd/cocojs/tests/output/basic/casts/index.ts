// This tests out some good and bad cast cases.

let a: string = "x";
let b: any = <any>a; // ok.

// TODO: a way to baseline expected failures.
// let c: number = <number>a; // statically rejected.

// TODO: a way to baseline expected runtime failures.
// let d: number = <number>b; // dynamically rejected.

