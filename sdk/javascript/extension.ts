// Copyright 2016 Marapongo, Inc. All rights reserved.

import { Context } from './context';
import { Stack } from './stack';

export abstract class Extension extends Stack {
    constructor(ctx: Context) {
        super(ctx);
    }

    // TODO: come up with a scheme for compile-time extensions; presumably an abstract virtual.
}

