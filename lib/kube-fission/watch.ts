// Copyright 2017 Pulumi, Inc. All rights reserved.

import {Function} from "./function";
import {Metadata} from "./metadata";
import * as coconut from "@coconut/coconut";

// Watch is a specification of a Kubernetes watch along with a URL to post events to.
export class Watch extends coconut.Resource implements WatchProperties {
    public readonly metadata: Metadata;
    public readonly namespace: string;
    public readonly objType: string;
    public readonly labelSelector: string;
    public readonly fieldSelector: string;
    public readonly function: Function;
    public readonly target: string;

    constructor(args: WatchProperties) {
        super();
        this.metadata = args.metadata;
        this.namespace = args.namespace;
        this.objType = args.objType;
        this.labelSelector = args.labelSelector;
        this.fieldSelector = args.fieldSelector;
        this.function = args.function;
        this.target = args.target;
    }
}

export interface WatchProperties {
    metadata: Metadata;
    namespace: string;
    objType: string;
    labelSelector: string;
    fieldSelector: string;
    function: Function;
    target: string; // watch publish target (URL, NATS stream, etc)
}

