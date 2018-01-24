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
        // Wrap the display with <> to indicate that it's been transformed in some manner.
        // However, don't bother doing this if we're already wrapping some transformed 
        // dependency.  i.e. we'll only ever show 'table.prop' or '<table.prop>', not 
        // '<<<<table.prop>>>>'.
        const display = this.__previewDisplay.length > 0 && this.__previewDisplay.charAt(0) == '<'
            ? this.__previewDisplay
            : "<" + this.__previewDisplay + ">";

        return new Dependency<U>(
            display,
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

    /* @internal */ public readonly __previewDisplay: string;

    /* @internal */ private readonly __resourcesData: Set<Resource>;

    // Method that actually produces the concrete value of this dependency, as well as the total
    // deployment-time set of resources this dependency depends on.  This code path will end up 
    // executing apply funcs, and should only be called during real deployment and not during
    // previews.
    /* @internal */ public readonly __getValue: () => Promise<T>;

    /* @internal */ public constructor(previewDisplay: string, resources: Set<Resource>, createComputeValueTask: () => Promise<T>) {
        this.__previewDisplay = previewDisplay;
        this.__resourcesData = resources;

        // __getValue lazily.  i.e. we will only apply funcs when asked the first time, and we will
        // also only apply them once (no matter how many times __getValue() is called).

        let __computeValueTask: Promise<T> = undefined;
        this.__getValue = () => {
            if (!__computeValueTask) {
                __computeValueTask = createComputeValueTask();
            }

            return __computeValueTask;
        };
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
export function createDependency<T>(previewDisplay: string, resource: Resource, value: Promise<T>): Dependency<T> {
    return new Dependency<T>(previewDisplay, new Set<Resource>([resource]), () => value);
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

    const previewDisplay = "(" + ds.map(d => d.__previewDisplay).join(", ") + ")"

    return new Dependency<{}[]>(
        previewDisplay,
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