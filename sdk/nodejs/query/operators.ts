// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { Enumerator } from "./interfaces";
import { ListEnumerator } from "./sources";

export class Filter<T> implements Enumerator<T> {
    constructor(private readonly source: Enumerator<T>, private readonly f: (t: T) => boolean) {}

    public current(): T {
        return this.source.current();
    }

    public moveNext(): boolean {
        while (this.source.moveNext()) {
            if (this.f(this.source.current())) {
                return true;
            }
        }
        return false;
    }
}

export class FlatMap<T, U> implements Enumerator<U> {
    private inner: Enumerator<U> = ListEnumerator.from([]);

    constructor(private readonly source: Enumerator<T>, private readonly f: (t: T) => U[]) {}

    public current(): U {
        return this.inner.current();
    }

    public moveNext(): boolean {
        while (true) {
            if (this.inner.moveNext()) {
                return true;
            }

            if (!this.source.moveNext()) {
                return false;
            }
            const inner = this.f(this.source.current());
            if (Array.isArray(inner)) {
                this.inner = ListEnumerator.from(inner);
            } else {
                this.inner = inner;
            }
        }
    }
}

export class Map<T, U> implements Enumerator<U> {
    constructor(private readonly source: Enumerator<T>, private readonly f: (t: T) => U) {}

    public current(): U {
        return this.f(this.source.current());
    }

    public moveNext(): boolean {
        return this.source.moveNext();
    }
}

export class Take<T> implements Enumerator<T> {
    private index: number = 0;

    constructor(private readonly source: Enumerator<T>, private readonly n: number) {}

    public current(): T {
        return this.source.current();
    }

    public moveNext(): boolean {
        this.index++;
        if (this.index <= this.n && this.source.moveNext()) {
            return true;
        } else {
            return false;
        }
    }
}
