// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

export class Boolean {
    public data: boolean;
    constructor(data: boolean) {
        this.data = data;
    }
}

export class String {
    public data: string;
    constructor(data: string) {
        this.data = data;
    }
}

export class Number {
    public data: number;
    constructor(data: number) {
        this.data = data;
    }
}

export class Promise<T> {
}
