// Copyright 2016 Marapongo, Inc. All rights reserved.

// Enable source map support so we get good stack-traces.
import "source-map-support/register";

// And now just re-export all submodules.
import * as ast from "./ast";
import * as compiler from "./compiler";
import * as diag from "./diag";
import * as pack from "./pack";
import * as symbols from "./symbols";
export { ast, compiler, diag, pack, symbols };

