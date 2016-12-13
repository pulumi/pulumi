// Copyright 2016 Marapongo, Inc. All rights reserved.

import { Arch } from './arch';
import { Cluster } from './cluster';
import { Options } from './options';

// A collection of information pertaining to the current compilation activity, like target cloud architecture, the
// cluster name, any compile-time options, and so on.
export interface Context {
    arch: Arch;       // the cloud architecture to target.
    cluster: Cluster; // the cluster we will be deploying to.
    options: Options; // any compiler options supplied.
}

