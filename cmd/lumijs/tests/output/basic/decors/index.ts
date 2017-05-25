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

