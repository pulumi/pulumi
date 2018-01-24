export abstract class Resource {
    private _field: any;
}

export type DependencyVal<T> = T | Dependency<T>;

export abstract class Dependency<T> {
    // Public API methods.

    // Transforms the data of the dependency with the provided func.  The result remains a Dependency 
    // so that dependent resources can be properly tracked.
    //
    // The inner func can itself return a Dependency.  If it does, the returned Dependency will
    // include all the Resources of 'this' Dependency and the inner Dependency combined.
    //
    // 'func' is not allowed to make resources.
    //
    // Outside only.  Note: this is the *only* outside public API.
    public apply<U>(func: (t: T) => DependencyVal<U>): Dependency<U> {
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


    // Gets the Resources that this Dependency depends on, for the purposes of Preview.  This
    // is different than the 'true' set of Resources that this Dependency actually depends on
    // as this will not actually calling into any user provided 'apply' funcs.  Because 
    // no 'apply' funcs are actually called, any inner dependencies taken will be unknown
    // during preview time.
    //
    // During normal deployment the actual apply funcs will be called, and may end up producing
    // more dependencies.
    //
    // TODO: We could restrict this and not allow inner Dependency objects to be automatically
    // unwrapped in 'apply'.  This would make the impl much simpler, but might end up being 
    // too unpleasant to use.  i.e. users would need to continually use 'unwrap' to convert a 
    // Dependency<Dependency<T>> to a Dependency<T>
    //
    /* @internal */ private __resourcesForPreviewData: Set<Resource>;
    /* @internal */ public __resourcesForPreview(): Set<Resource> {
        if (!this.__resourcesForPreviewData) {
            // Only compute the set of resources once.
            this.__resourcesForPreviewData = this.__computeResourcesForPreview();
        }

        // Always create a copy so that no one accidentally modifies our Resource list.
        return new Set<Resource>(this.__resourcesForPreviewData);
    }

    // The single task we produce that represents the computation of getting the concrete value
    // and full set of dependency resources.  We use a singleton task so that we don't end
    // up causing computation (especially user provided funcs) from executing many times over).
    /* @internal */ private __getValueAndResourcesTask: Promise<[T, Set<Resource>]>;

    // Method that actually produces the concrete value of this dependency, as well as the total
    // deployment-time set of resources this dependency depends on.  This code path will end up 
    // executing apply funcs, and should not be called during preview, only during realy
    // deployment.
    //
    // This function is implemented lazily.  i.e. when the result is awaited, 'apply' funcs 
    // will only ever execute once.  Values are then automatically cached and will be returned
    // if called and awaited again.
    /* @internal */ public async __getValueAndResourcesAsync(): Promise<[T, Set<Resource>]> {
        // Ensure that we always hand out the same task.  This way, no matter how many
        // times anyone calls __getValueAndResourcesAsync or awaits it, the computation is only
        // executed once.
        if (!this.__getValueAndResourcesTask) {
            this.__getValueAndResourcesTask = this.__computeValueAndResourcesAsync();
        }

        var [val, resources] = await this.__getValueAndResourcesTask;

        // Always create a copy so that no one accidentally modifies our Resource list.
        return [val, new Set<Resource>(resources)];
    }

    /* @internal */ protected abstract __computeValueAndResourcesAsync(): Promise<[T, Set<Resource>]>;
    /* @internal */ protected abstract __computeResourcesForPreview(): Set<Resource>;

    // The list of resource that this dependency value depends on.
    // Only callable on the outside.
    /* @internal */ public async __valueAsync(): Promise<T> {
        const tuple = await this.__getValueAndResourcesAsync();
        return tuple[0];
    }

    /* @internal */ private __resources: Set<Resource>;

    // The list of resource that this dependency value depends on.
    // Only callable on the outside.
    /* @internal */ public async __resourcesAsync(): Promise<Set<Resource>> {
        if (!this.__resources) {
            const tuple = await this.__getValueAndResourcesAsync();
            this.__resources = tuple[1];
        }

        // Always create a copy so that no one accidentally modifies our Resource list.
        return new Set<Resource>(this.__resources);
    }

    // Abstract methods for our subclasses to implement.  They will only ever be called once.

    // Helper that our subclasses can use to assert that __getValueAndResourcesWorkerAsync
    // is only called once.

    /* @internal */ private _called = false;
    /* @internal */ protected ensuredCalledOnlyOnce(): void {
        if (this._called) {
            throw new Error("Should not be possible to call this multiple times.");
        }
        this._called = true;
    }
}

declare global {
    interface Set<T> {
        unionWith(other: Set<T>): Set<T>;
    }
}

Set.prototype.unionWith = function unionWith<T>(other: Set<T>): Set<T> {
    for (const v of other) {
        this.add(v);
    }

    return this;
}

class ApplyDependency<T, U> extends Dependency<U> {
    public constructor(
            private readonly source: Dependency<T>,
            private readonly func: (t: T) => DependencyVal<U>) {
        super();
    }

    protected __computeResourcesForPreview(): Set<Resource> {
        // During preview, the resources for a transformed dependency are the same
        // as the original source's preview dependencies.
        return this.source.__resourcesForPreview();
    }

    protected async __computeValueAndResourcesAsync(): Promise<[U, Set<Resource>]> {
        this.ensuredCalledOnlyOnce();

        const [value, resourcesCopy] = await this.source.__getValueAndResourcesAsync();

        const transformedDepVal = this.func(value);

        // Applying the transformation to the source value may have produced a new Dependency.
        // If so, we Hhave to combine the 'source's resources, with the resources from this 
        // inner dependency.
        return combineResources(resourcesCopy, transformedDepVal);
    }
}

async function combineResources<T>(resourcesCopy: Set<Resource>, depVal: DependencyVal<T>): Promise<[T, Set<Resource>]> {
    if (depVal instanceof Dependency) {
        const [underlyingVal, underlyingResources] = await depVal.__getValueAndResourcesAsync();
        return [underlyingVal, resourcesCopy.unionWith(underlyingResources)];
    } else {
        return [depVal, resourcesCopy]
    }
}

class SimpleDependency<T> extends Dependency<T> {
    public constructor(private readonly res: Set<Resource>,
        private readonly val: T | Promise<T>) {
        super();
    }

    protected __computeResourcesForPreview(): Set<Resource> {
        return this.res;
    }

    protected async __computeValueAndResourcesAsync(): Promise<[T, Set<Resource>]> {
        this.ensuredCalledOnlyOnce();
        const val = await this.val;
        return [val, this.res];
    }
}

// Helper function actually allow Resource to create Dependency objects for its output properties.
// Should only be called by pulumi, not by users (TODO: i think).
//
// TODO: this probably should only take "value: Promise<T>".  Output properties of a resource 
// will always be promises...
export function createDependency<T>(value: T | Promise<T>, resource: Resource): Dependency<T> {
    return new SimpleDependency<T>(new Set<Resource>([resource]), value);
}

export type D<T> = DependencyVal<T>;

export function combine<T1, T2>(d1: D<T1>, d2: D<T2>): Dependency<[T1, T2]>;
export function combine<T1, T2, T3>(d1: D<T1>, d2: D<T2>, d3: D<T3>): Dependency<[T1, T2, T3]>;
export function combine<T1, T2, T3, T4>(d1: D<T1>, d2: D<T2>, d3: D<T3>, d4: D<T4>): Dependency<[T1, T2, T3, T4]>;
export function combine<T1, T2, T3, T4, T5>(d1: D<T1>, d2: D<T2>, d3: D<T3>, d4: D<T4>, d5: D<T5>): Dependency<[T1, T2, T3, T4, T5]>;
export function combine<T1, T2, T3, T4, T5, T6>(d1: D<T1>, d2: D<T2>, d3: D<T3>, d4: D<T4>, d5: D<T5>, d6: D<T6>): Dependency<[T1, T2, T3, T4, T5, T6]>;
export function combine<T1, T2, T3, T4, T5, T6, T7>(d1: D<T1>, d2: D<T2>, d3: D<T3>, d4: D<T4>, d5: D<T5>, d6: D<T6>, d7: D<T7>): Dependency<[T1, T2, T3, T4, T5, T6, T7]>;
export function combine<T1, T2, T3, T4, T5, T6, T7, T8>(d1: D<T1>, d2: D<T2>, d3: D<T3>, d4: D<T4>, d5: D<T5>, d6: D<T6>, d7: D<T7>, d8: D<T8>): Dependency<[T1, T2, T3, T4, T5, T6, T7, T8]>;
export function combine<T>(...ds: D<T>[]): Dependency<T[]>;
export function combine(...ds: D<{}>[]): Dependency<{}[]> {
    return new CombinedDependency<{}[]>(ds);
}

class CombinedDependency<T> extends Dependency<T> {
    public constructor(private readonly ds: DependencyVal<{}>[]) {
        super();
    }

    protected __computeResourcesForPreview(): Set<Resource> {
        var result = new Set<Resource>();

        for (const d of this.ds) {
            if (d instanceof Dependency) {
                for (const r of d.__resourcesForPreview()) {
                    result.add(r);
                }
            }
        }

        return result;
    }

    protected async __computeValueAndResourcesAsync(): Promise<[T, Set<Resource>]> {
        this.ensuredCalledOnlyOnce();

        const result: any[] = [];
        const resources = new Set<Resource>();

        for (const d of this.ds) {
            if (d instanceof Dependency) {
                const [innerValue, innerResources] = await d.__getValueAndResourcesAsync();
                resources.unionWith(innerResources);
                result.push(innerValue);
            } else {
                result.push(d);
            }
        }

        return [<T><any>result, resources];
    }
}

export function dep_if<T>(
    d: Dependency<boolean>,
    whenTrue: () => DependencyVal<T>,
    whenFalse: () => DependencyVal<T>): Dependency<T> {

    return d.apply<T>(b => b ? whenTrue() : whenFalse());
}

export function dep_forOf<T extends Iterable<TItem>, U, TItem>(
    source: Dependency<T>,
    eachVal: (t: T, item: TItem) => DependencyVal<U>): Dependency<U[]> {

    return source.apply<U[]>(t => {
        const subDeps: DependencyVal<U>[] = [];
        for (const item of t) {
            subDeps.push(eachVal(t, item));
        }

        const combined = combine(...subDeps);
        return combined;
    });
}

export function dep_forIn<T, U>(
    source: Dependency<T>,
    eachVal: (t: T, key: keyof T) => DependencyVal<U>): Dependency<U[]> {

    return source.apply<U[]>(t => {
        const subDeps: DependencyVal<U>[] = [];
        for (const key in t) {
            subDeps.push(eachVal(t, key));
        }

        const combined = combine(...subDeps);
        return combined;
    });
}

export function unwrap<T>(d: Dependency<Dependency<T>>): Dependency<T> {
    return d.apply(subD => subD);
}

export function unwrapN<T>(d: Dependency<Dependency<T>[]>): Dependency<T[]> {
    return d.apply(subDs => combine(...subDs));
}