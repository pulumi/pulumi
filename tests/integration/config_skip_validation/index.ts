import * as pulumi from '@pulumi/pulumi';

// Intentionally does not read the required `example` config key. The project
// config schema declares it as required, so config validation fails when it is
// unset, but the program itself runs fine. This lets us exercise
// --skip-config-validation end to end without the program failing at runtime.
console.log("config-skip-validation program ran");
