// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        internal static bool _excessiveDebugOutput;

        /// <summary>
        /// serializeResourceProperties walks the props object passed in, awaiting all interior promises besides those for `id`
        /// and `urn`, creating a reasonable POCO object that can be remoted over to registerResource.
        /// </summary>
        private static Task<SerializationResult> SerializeResourcePropertiesAsync(
            string label, ResourceArgs args)
        {
            return SerializeFilteredPropertiesAsync(label, args, key => key != "id" && key != "urn");
        }

        /**
 * serializeFilteredProperties walks the props object passed in, awaiting all interior promises for
 * properties with keys that match the provided filter, creating a reasonable POJO object that can
 * be remoted over to registerResource.
 */
        private static async Task<SerializationResult> SerializeFilteredPropertiesAsync(
                string label, ResourceArgs args, Predicate<string> acceptKey)
        {
            var props = args.ToDictionary();

            var propertyToDependentResources = new Dictionary<string, HashSet<Resource>>();
            var result = new Dictionary<string, object>();

            foreach (var (key, input) in props)
            {
                if (acceptKey(key))
                {
                    // We treat properties with null values as if they do not exist.
                    var dependentResources = new HashSet<Resource>();
                    var v = await SerializePropertyAsync($"{label}.{key}", input, dependentResources).ConfigureAwait(false);
                    if (v != null)
                    {
                        result[key] = v;
                        propertyToDependentResources[key] = dependentResources;
                    }
                }
            }

            return new SerializationResult(result, propertyToDependentResources);
        }

        private static async Task<object?> SerializedPropertyAsync(
            string ctx, object prop, HashSet<Resource> dependentResources)
        {
            // IMPORTANT:
            // IMPORTANT: Keep this in sync with serializesPropertiesSync in invoke.ts
            // IMPORTANT:
            if (prop == null ||
                prop is bool ||
                prop is int ||
                prop is string)
            {
                if (_excessiveDebugOutput)
                {
                    Serilog.Log.Debug($"Serialize property[{ctx}]: primitive={prop}");
                }

                return prop;
            }

    if (asset.Asset.isInstance(prop) || asset.Archive.isInstance(prop)) {
        // Serializing an asset or archive requires the use of a magical signature key, since otherwise it would look
        // like any old weakly typed object/map when received by the other side of the RPC boundary.
        const obj: any = {
            [specialSigKey]: asset.Asset.isInstance(prop)? specialAssetSig : specialArchiveSig,
        };

        return await serializeAllKeys(prop, obj);
    }

    if (prop instanceof Promise) {
        // For a promise input, await the property and then serialize the result.
        if (excessiveDebugOutput) {
            log.debug(`Serialize property[${ctx}]: Promise<T>`);
        }

        const subctx = `Promise<${ctx}>`;
        return serializeProperty(subctx,
            await debuggablePromise(prop, `serializeProperty.await(${ subctx})`), dependentResources);
    }

    if (Output.isInstance(prop)) {
        if (excessiveDebugOutput) {
            log.debug(`Serialize property[${ctx}]: Output<T>`);
        }

        for (const resource of prop.resources()) {
            dependentResources.add(resource);
        }

        // When serializing an Output, we will either serialize it as its resolved value or the "unknown value"
        // sentinel. We will do the former for all outputs created directly by user code (such outputs always
        // resolve isKnown to true) and for any resource outputs that were resolved with known values.
        const isKnown = await prop.isKnown;

// You might think that doing an explict `=== true` here is not needed, but it is for a subtle reason. If the
// output we are serializing is a proxy itself, and it comes from a version of the SDK that did not have the
// `isSecret` member on `OutputImpl` then the call to `prop.isSecret` here will return an Output itself,
// which will wrap undefined, if it were to be resolved (since `Output` has no member named .isSecret).
// so we must compare to the literal true instead of just doing await prop.isSecret.
const isSecret = await prop.isSecret === true;
const value = await serializeProperty(`${ctx}.id`, prop.promise(), dependentResources);

        if (!isKnown) {
            return unknownValue;
        }
        if (isSecret && await monitorSupportsSecrets()) {
            return {
                [specialSigKey]: specialSecretSig,
                // coerce 'undefined' to 'null' as required by the protobuf system.
                value: value === undefined? null : value,
            };
        }
        return value;
    }

    if (CustomResource.isInstance(prop)) {
        // Resources aren't serializable; instead, we serialize them as references to the ID property.
        if (excessiveDebugOutput) {
            log.debug(`Serialize property[${ctx}]: custom resource id`);
        }

        dependentResources.add(prop);
        return serializeProperty(`${ ctx}.id`, prop.id, dependentResources);
    }

    if (ComponentResource.isInstance(prop)) {
        // Component resources often can contain cycles in them.  For example, an awsinfra
        // SecurityGroupRule can point a the awsinfra SecurityGroup, which in turn can point back to
        // its rules through its `egressRules` and `ingressRules` properties.  If serializing out
        // the `SecurityGroup` resource ends up trying to serialize out those properties, a deadlock
        // will happen, due to waiting on the child, which is waiting on the parent.
        //
        // Practically, there is no need to actually serialize out a component.  It doesn't represent
        // a real resource, nor does it have normal properties that need to be tracked for differences
        // (since changes to its properties don't represent changes to resources in the real world).
        //
        // So, to avoid these problems, while allowing a flexible and simple programming model, we
        // just serialize out the component as its urn.  This allows the component to be identified
        // and tracked in a reasonable manner, while not causing us to compute or embed information
        // about it that is not needed, and which can lead to deadlocks.
        if (excessiveDebugOutput) {
            log.debug(`Serialize property[${ctx}]: component resource urnid`);
        }

        return serializeProperty(`${ ctx}.urn`, prop.urn, dependentResources);
    }

    if (prop instanceof Array) {
        const result: any[] = [];
        for (let i = 0; i<prop.length; i++) {
            if (excessiveDebugOutput) {
                log.debug(`Serialize property[${ctx}]: array[${i}] element`);
            }
            // When serializing arrays, we serialize any undefined values as `null`. This matches JSON semantics.
            const elem = await serializeProperty(`${ctx}[${i}]`, prop[i], dependentResources);
            result.push(elem === undefined? null : elem);
        }
        return result;
    }

    return await serializeAllKeys(prop, { });

    async function serializeAllKeys(innerProp: any, obj: any)
{
    for (const k of Object.keys(innerProp)) {
        if (excessiveDebugOutput)
        {
            log.debug(`Serialize property[${ ctx}]: object.${ k}`);
        }

        // When serializing an object, we omit any keys with undefined values. This matches JSON semantics.
        const v = await serializeProperty(`${ ctx}.${ k}`, innerProp[k], dependentResources);
        if (v !== undefined)
        {
            obj[k] = v;
        }
    }

    return obj;
}
}

        private struct SerializationResult
        {
            public readonly Dictionary<string, object> Serialized;
            public readonly Dictionary<string, HashSet<Resource>> PropertyToDependentResources;

            public SerializationResult(Dictionary<string, object> result, Dictionary<string, HashSet<Resource>> propertyToDependentResources)
            {
                Serialized = result;
                PropertyToDependentResources = propertyToDependentResources;
            }
        }
    }
}
