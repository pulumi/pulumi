// Copyright 2016 Marapongo, Inc. All rights reserved.

import { Context } from './context';

// A stack is a fundamental resource that encapsulates and exposes zero-to-many other services.
export abstract class Stack {
    protected ctx: Context;

    constructor(ctx: Context) {
        this.ctx = ctx;
    }
}

