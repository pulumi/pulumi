// First, define a bunch of no-op decorators.
function classDecorate(target: Object) {}
function propertyDecorate(target: Object, propertyKey: string) {}
function methodDecorate(target: Object, propertyKey: any, descriptor: any) {}
function parameterDecorate(target: Object, propertyKey: string, parameterIndex: number) {}

// Now test that each of the cases works and leads to the right attributes in the resulting metadata.
@classDecorate
class TestDecorators {
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


