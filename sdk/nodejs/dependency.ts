export abstract class Resource {
    private _field: any;
}

export type DependencyVal<T> = T | Dependency<T>;

export abstract class Dependency<T> {
    private readonly __resourcesData: Set<Resource>;

    /* @internal */ public constructor(resources: Set<Resource>) {
        this.__called = false;
        this.__resourcesData = resources;
    }

    // Public API methods.

    // Transforms the data of the dependency with the provided func.  The result remains a Dependency 
    // so that dependent resources can be properly tracked.
    //
    // The inner func should not return a Dependency itself. (TODO: can we check for that?)
    //
    // 'func' is not allowed to make resources.
    //
    // Outside only.  Note: this is the *only* outside public API.
    public apply<U>(func: (t: T) => U): Dependency<U> {
        return new ApplyDependency<T, U>(this, func);
    }

    // Retrieves the underlying value of this dependency.
    //
    // Inside only.  Note: this is the *only* inside API available.
    public get(): T {
        throw new Error("Cannot call during deployment.");
    }

    // Internal implementation details. Hidden from the .d.ts file by using @internal and also
    // naming with __. Users are not allowed to call these methods.  If they do, pulumi cannot
    // provide any guarantees.  TODO: is there any way to make it so that users can't even get
    // access to these members?  Maybe by using Symbols or WeakMaps and the like.

    // The single task we produce that represents the computation of getting the concrete value
    // We use a singleton task so that we don't end up causing computation (especially user provided 
    // funcs) from executing many times over.  We also lazily create this as we don't want apply-funcs
    // to run until necessary.
    /* @internal */ private __getValueTask: Promise<T>;

    // Method that actually produces the concrete value of this dependency, as well as the total
    // deployment-time set of resources this dependency depends on.  This code path will end up 
    // executing apply funcs, and should not be called during preview, only during realy
    // deployment.
    //
    // This function is implemented lazily.  i.e. when the result is awaited, 'apply' funcs 
    // will only ever execute once.  Values are then automatically cached and will be returned
    // if called and awaited again.
    /* @internal */ public __getValue(): Promise<T> {
        // Ensure that we always hand out the same task.  This way, no matter how many
        // times anyone calls __getValueAndResourcesAsync or awaits it, the computation is only
        // executed once.
        if (!this.__getValueTask) {
            this.__getValueTask = this.__computeValue();
        }

        return this.__getValueTask;
    }

    // The list of resource that this dependency value depends on.
    // Only callable on the outside.
    /* @internal */ public __resources(): Set<Resource> {
        // Always create a copy so that no one accidentally modifies our Resource list.
        return new Set<Resource>(this.__resourcesData);
    }

    // Abstract methods for our subclasses to implement.  They will only ever be called once.

    /* @internal */ protected abstract __computeValue(): Promise<T>;

    // Helper that our subclasses can use to assert that __getValueAndResourcesWorkerAsync
    // is only called once.

    /* @internal */ private __called;
    /* @internal */ protected __ensuredCalledOnlyOnce(): void {
        if (this.__called) {
            throw new Error("Should not be possible to call this multiple times.");
        }
        this.__called = true;
    }
}

class ApplyDependency<T, U> extends Dependency<U> {
    public constructor(
            private readonly source: Dependency<T>,
            private readonly func: (t: T) => U) {
        super(source.__resources());
    }

    protected async __computeValue(): Promise<U> {
        this.__ensuredCalledOnlyOnce();
        return this.source.__getValue().then(t => this.func(t));
    }
}

class SimpleDependency<T> extends Dependency<T> {
    public constructor(
        private readonly resources: Set<Resource>,
        private readonly val: Promise<T>) {

        super(resources);
    }

    protected async __computeValue(): Promise<T> {
        this.__ensuredCalledOnlyOnce();
        return this.val;
    }
}

// Helper function actually allow Resource to create Dependency objects for its output properties.
// Should only be called by pulumi, not by users (TODO: i think).
//
// TODO: this probably should only take "value: Promise<T>".  Output properties of a resource 
// will always be promises...
export function createDependency<T>(value: Promise<T>, resource: Resource): Dependency<T> {
    return new SimpleDependency<T>(new Set<Resource>([resource]), value);
}

export type D<T> = Dependency<T>;

export function combine<T1, T2>(d1: D<T1>, d2: D<T2>): D<[T1, T2]>;
export function combine<T1, T2, T3>(d1: D<T1>, d2: D<T2>, d3: D<T3>): D<[T1, T2, T3]>;
export function combine<T1, T2, T3, T4>(d1: D<T1>, d2: D<T2>, d3: D<T3>, d4: D<T4>): D<[T1, T2, T3, T4]>;
export function combine<T1, T2, T3, T4, T5>(d1: D<T1>, d2: D<T2>, d3: D<T3>, d4: D<T4>, d5: D<T5>): D<[T1, T2, T3, T4, T5]>;
export function combine<T1, T2, T3, T4, T5, T6>(d1: D<T1>, d2: D<T2>, d3: D<T3>, d4: D<T4>, d5: D<T5>, d6: D<T6>): D<[T1, T2, T3, T4, T5, T6]>;
export function combine<T1, T2, T3, T4, T5, T6, T7>(d1: D<T1>, d2: D<T2>, d3: D<T3>, d4: D<T4>, d5: D<T5>, d6: D<T6>, d7: D<T7>): D<[T1, T2, T3, T4, T5, T6, T7]>;
export function combine<T1, T2, T3, T4, T5, T6, T7, T8>(d1: D<T1>, d2: D<T2>, d3: D<T3>, d4: D<T4>, d5: D<T5>, d6: D<T6>, d7: D<T7>, d8: D<T8>): D<[T1, T2, T3, T4, T5, T6, T7, T8]>;
export function combine<T>(...ds: D<T>[]): D<T[]>;
export function combine(...ds: D<{}>[]): D<{}[]> {
    return new CombinedDependency<{}[]>(ds);
}

function getAllResources(ds: Dependency<{}>[]): Set<Resource> {
    const result = new Set<Resource>();

    for (const d of ds) {
        for (const r of d.__resources()) {
            result.add(r);
        }
    }

    return result;
}

class CombinedDependency<T> extends Dependency<T> {
    public constructor(private readonly ds: Dependency<{}>[]) {
        super(getAllResources(ds));
    }

    protected __computeValue(): Promise<T> {
        this.__ensuredCalledOnlyOnce();

        const allValues = this.ds.map(d => d.__getValue());
        const all = Promise.all(allValues);

        // Nasty sideways cast.  Basically, we know from the construction of the 'combine'
        // functions that T will be the tuple type corresponding to the types of each of
        // our ds '{}' types.  i.e. if we're combining Dependency<string> and Dependency<number>
        // then T will be [string, number].   The promise value we're returning is just {}[]
        // so we blindly cast that away into [...types] since we know it's safe.
        return <Promise<T>><Promise<any>>all;
    }
}

// Note: these are now no longer safe.  During preview we do not run .apply funcs.  As such, we
// won't know what additional dependencies are being taken down the func paths. The right way
// to handle these situations is to capture any inner dependencies beforehand using .combine
// then call .apply to do any transformations of all those values.

//export function dep_if<T>(
//        d: Dependency<boolean>,
//        whenTrue: () => DependencyVal<T>,
//        whenFalse: () => DependencyVal<T>): Dependency<T> {

//    return d.apply(b => b ? whenTrue() : whenFalse());
//}

//export function dep_forOf<T extends Iterable<TItem>, U, TItem>(
//        source: Dependency<T>,
//        eachVal: (t: T, item: TItem) => DependencyVal<U>): Dependency<U[]> {

//    return source.apply(t => {
//        const subDeps = Array.from(t).map(item => eachVal(t, item));
//        const combined = combine(...subDeps);
//        return combined;
//    });
//}

//export function dep_forIn<T, U>(
//        source: Dependency<T>,
//        eachVal: (t: T, key: string) => DependencyVal<U>): Dependency<U[]> {

//    return source.apply(t => {
//        const subDeps = Object.keys(t).map(key => eachVal(t, key));
//        const combined = combine(...subDeps);
//        return combined;
//    });
//}

//export function unwrap<T>(d: Dependency<Dependency<T>>): Dependency<T> {
//    return d.apply(subD => subD);
//}

//export function unwrapN<T>(d: Dependency<Dependency<T>[]>): Dependency<T[]> {
//    return d.apply(subDs => combine(...subDs));
//}