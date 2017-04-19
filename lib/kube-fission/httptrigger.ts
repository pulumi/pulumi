// Copyright 2017 Pulumi, Inc. All rights reserved.

import {Function} from "./function";
import {Metadata} from "./metadata";
import * as coconut from "@coconut/coconut";

// HTTPTrigger maps URL patterns to functions.  Function.UID is optional; if absent, the latest version of the function
// will automatically be selected.
export class HTTPTrigger extends coconut.Resource implements HTTPTriggerProperties {
    public readonly name: string;
    public readonly uid?: string;
    public readonly urlPattern: string;
    public readonly method: string;
    public readonly function: Function;

    constructor(args: HTTPTriggerProperties) {
        super();
        this.name = args.name;
        this.uid = args.uid;
        this.urlPattern = args.urlPattern;
        this.method = args.method;
        this.function = args.function;
    }
}

export interface HTTPTriggerProperties extends Metadata {
    urlPattern: string;
    method: string;
    function: Function;
}

