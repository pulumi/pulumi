// Copyright 2016-2017, Pulumi Corporation
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

import {classDecorate, propertyDecorate, methodDecorate, parameterDecorate} from "./decors";
import * as decors from "./decors";

// Test that each of the cases works and leads to the right attributes in the resulting metadata.

// First, using "simple" names.
@classDecorate
class TestSimpleDecorators {
    @propertyDecorate a: string;
    @propertyDecorate public b: string;
    @propertyDecorate public c: string = "test";

    @methodDecorate
    m1(): string { return ""; }
    @methodDecorate
    public m2(): string { return ""; }
    @methodDecorate
    get p1(): string { return ""; }
    set p1(v: string) {}
    get p2(): string { return ""; }
    @methodDecorate
    set p2(v: string) {}
    @methodDecorate
    public get p3() { return "" }
    public set p3(v: string) {}

    mparam1(@parameterDecorate x, y, @parameterDecorate z): void { }
    @methodDecorate
    mparam2(@parameterDecorate x, y, @parameterDecorate z): void { }
}

// Next, using "qualified" names.
@decors.classDecorate
class TestQualifiedDecorators {
    @decors.propertyDecorate a: string;
    @decors.propertyDecorate public b: string;
    @decors.propertyDecorate public c: string = "test";

    @decors.methodDecorate
    m1(): string { return ""; }
    @decors.methodDecorate
    public m2(): string { return ""; }
    @decors.methodDecorate
    get p1(): string { return ""; }
    set p1(v: string) {}
    get p2(): string { return ""; }
    @decors.methodDecorate
    set p2(v: string) {}
    @decors.methodDecorate
    public get p3() { return "" }
    public set p3(v: string) {}

    mparam1(@decors.parameterDecorate x, y, @decors.parameterDecorate z): void { }
    @decors.methodDecorate
    mparam2(@decors.parameterDecorate x, y, @decors.parameterDecorate z): void { }
}

