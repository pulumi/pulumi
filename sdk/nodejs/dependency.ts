export abstract class Resource {
    private _field: any;
}

export type DependencyVal<T> = T | Dependency<T>;

export class Dependency<T> {
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
        return new Dependency<U>(
            this.__resourcesData,
            () => this.__getValue().then(func));
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

    private readonly __resourcesData: Set<Resource>;

    // The function we use to lazily create __computeValueTask.
    private readonly __createComputeValueTask: () => Promise<T>;

    // The single task we produce that represents the computation of getting the concrete value
    // We use a singleton task so that we don't end up causing computation (especially user provided 
    // funcs) from executing many times over.  We also lazily create this as we don't want apply-funcs
    // to run until necessary.
    private __computeValueTask: Promise<T>;

    /* @internal */ public constructor(resources: Set<Resource>, createComputeValueTask: () => Promise<T>) {
        this.__resourcesData = resources;
        this.__createComputeValueTask = createComputeValueTask;
    }


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
        if (!this.__computeValueTask) {
            this.__computeValueTask = this.__createComputeValueTask();
        }

        return this.__computeValueTask;
    }

    // The list of resource that this dependency value depends on.
    // Only callable on the outside.
    /* @internal */ public __resources(): Set<Resource> {
        // Always create a copy so that no one accidentally modifies our Resource list.
        return new Set<Resource>(this.__resourcesData);
    }
}

// Helper function actually allow Resource to create Dependency objects for its output properties.
// Should only be called by pulumi, not by users (TODO: i think).
//
// TODO: this probably should only take "value: Promise<T>".  Output properties of a resource 
// will always be promises...
export function createDependency<T>(value: Promise<T>, resource: Resource): Dependency<T> {
    return new Dependency<T>(new Set<Resource>([resource]), () => value);
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
    const allResources = new Set<Resource>();
    for (const d of ds) {
        for (const r of d.__resources()) {
            allResources.add(r);
        }
    }

    return new Dependency<{}[]>(
        allResources,
        () => Promise.all(this.ds.map(d => d.__getValue())));
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