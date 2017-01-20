// Copyright 2016 Marapongo, Inc. All rights reserved.

import { Options } from './options';

// A collection of information pertaining to the current compilation activity, like target cloud architecture, the
// cluster name, any compile-time options, and so on.
export interface Context {
    options: Options; // any compiler options supplied.
}

