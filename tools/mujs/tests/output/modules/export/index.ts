// Export a whole submodule:
import * as other from "./other";
export {other};

// Manually export C, I, and v without using export declarations:
class C {}
interface I {}
let v = 42;
export {C, I, v};

