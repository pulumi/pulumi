// Copyright 2016 Pulumi, Inc. All rights reserved.

import { Context } from './context';
import { Stack } from './stack';

export abstract class Resource extends Stack {
    constructor() {
        super();
    }
}

