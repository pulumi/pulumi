// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Diagnostics;
using System.Linq;
using System.Threading.Tasks;
using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// Useful static utility methods for both creating and working with <see cref="Output{T}"/>s.
    /// </summary>
    public static partial class Output
    {
        public static Output<T> Create<T>(T value)
            => Create(Task.FromResult(value));

        public static Output<T> Create<T>(Task<T> value)
            => Output<T>.Create(value);

        public static Output<T> CreateSecret<T>(T value)
            => CreateSecret(Task.FromResult(value));

        public static Output<T> CreateSecret<T>(Task<T> value)
            => Output<T>.CreateSecret(value);

        /// <summary>
        /// Returns a new <see cref="Output{T}"/> which is a copy of the existing output but marked as
        /// a non-secret. The original output is not modified in any way.
        /// </summary>
        public static Output<T> Unsecret<T>(Output<T> output)
            => output.WithIsSecret(Task.FromResult(false));

        /// <summary>
        /// Retrieves the secretness status of the given output.
        /// </summary>
        public static async Task<bool> IsSecretAsync<T>(Output<T> output)
        {
            var dataTask = await output.DataTask.ConfigureAwait(false);
            return dataTask.IsSecret;
        }

        /// <summary>
        /// Combines all the <see cref="Input{T}"/> values in <paramref name="inputs"/>
        /// into a single <see cref="Output{T}"/> with an <see cref="ImmutableArray{T}"/>
        /// containing all their underlying values.  If any of the <see cref="Input{T}"/>s are not
        /// known, the final result will be not known.  Similarly, if any of the <see
        /// cref="Input{T}"/>s are secrets, then the final result will be a secret.
        /// </summary>
        public static Output<ImmutableArray<T>> All<T>(params Input<T>[] inputs)
            => Output<T>.All(ImmutableArray.CreateRange(inputs));

        /// <summary>
        /// <see cref="All{T}(Input{T}[])"/> for more details.
        /// </summary>
        public static Output<ImmutableArray<T>> All<T>(IEnumerable<Input<T>> inputs)
            => Output<T>.All(ImmutableArray.CreateRange(inputs));

        /// <summary>
        /// Combines all the <see cref="Output{T}"/> values in <paramref name="outputs"/>
        /// into a single <see cref="Output{T}"/> with an <see cref="ImmutableArray{T}"/>
        /// containing all their underlying values.  If any of the <see cref="Output{T}"/>s are not
        /// known, the final result will be not known.  Similarly, if any of the <see
        /// cref="Output{T}"/>s are secrets, then the final result will be a secret.
        /// </summary>
        public static Output<ImmutableArray<T>> All<T>(params Output<T>[] outputs)
            => All(outputs.AsEnumerable());

        /// <summary>
        /// <see cref="All{T}(Output{T}[])"/> for more details.
        /// </summary>
        public static Output<ImmutableArray<T>> All<T>(IEnumerable<Output<T>> outputs)
            => Output<T>.All(ImmutableArray.CreateRange(outputs.Select(o => (Input<T>)o)));

        /// <summary>
        /// Takes in a <see cref="FormattableString"/> with potential <see cref="Input{T}"/>s or
        /// <see cref="Output{T}"/> in the 'placeholder holes'.  Conceptually, this method unwraps
        /// all the underlying values in the holes, combines them appropriately with the <see
        /// cref="FormattableString.Format"/> string, and produces an <see cref="Output{T}"/>
        /// containing the final result.
        /// <para/>
        /// If any of the <see cref="Input{T}"/>s or <see cref="Output{T}"/>s are not known, the
        /// final result will be not known.  Similarly, if any of the <see cref="Input{T}"/>s or
        /// <see cref="Output{T}"/>s are secrets, then the final result will be a secret.
        /// </summary>
        public static Output<string> Format(FormattableString formattableString)
        {
            var arguments = formattableString.GetArguments();
            var inputs = new Input<object?>[arguments.Length];

            for (var i = 0; i < arguments.Length; i++)
            {
                var arg = arguments[i];
                inputs[i] = arg.ToObjectOutput();
            }

            return All(inputs).Apply(objs =>
                string.Format(formattableString.Format, objs.ToArray()));
        }

        internal static Output<ImmutableArray<T>> Concat<T>(Output<ImmutableArray<T>> values1, Output<ImmutableArray<T>> values2)
            => Tuple(values1, values2).Apply(a => a.Item1.AddRange(a.Item2));
    }

    /// <summary>
    /// Internal interface to allow our code to operate on outputs in an untyped manner. Necessary
    /// as there is no reasonable way to write algorithms over heterogeneous instantiations of
    /// generic types.
    /// </summary>
    internal interface IOutput
    {
        Task<ImmutableHashSet<Resource>> GetResourcesAsync();

        /// <summary>
        /// Returns an <see cref="Output{T}"/> equivalent to this, except with our
        /// <see cref="OutputData{X}.Value"/> casted to an object.
        /// </summary>
        Task<OutputData<object?>> GetDataAsync();
    }

    /// <summary>
    /// <see cref="Output{T}"/>s are a key part of how Pulumi tracks dependencies between <see
    /// cref="Resource"/>s. Because the values of outputs are not available until resources are
    /// created, these are represented using the special <see cref="Output{T}"/>s type, which
    /// internally represents two things:
    /// <list type="number">
    /// <item>An eventually available value of the output</item>
    /// <item>The dependency on the source(s) of the output value</item>
    /// </list>
    /// In fact, <see cref="Output{T}"/>s is quite similar to <see cref="Task{TResult}"/>.
    /// Additionally, they carry along dependency information.
    /// <para/>
    /// The output properties of all resource objects in Pulumi have type <see cref="Output{T}"/>.
    /// </summary>
    public sealed class Output<T> : IOutput
    {
        internal Task<OutputData<T>> DataTask { get; private set; }

        internal Output(Task<OutputData<T>> dataTask) {
            this.DataTask = dataTask;

            if (Deployment.TryGetInternalInstance(out var instance))
            {
                instance.Runner.RegisterTask(TypeNameHelper.GetTypeDisplayName(GetType(), false), dataTask);
            }
        }

        internal async Task<T> GetValueAsync()
        {
            var data = await DataTask.ConfigureAwait(false);
            return data.Value;
        }

        async Task<ImmutableHashSet<Resource>> IOutput.GetResourcesAsync()
        {
            var data = await DataTask.ConfigureAwait(false);
            return data.Resources;
        }

        async Task<OutputData<object?>> IOutput.GetDataAsync()
            => await DataTask.ConfigureAwait(false);

        public static Output<T> Create(Task<T> value)
            => Create(value, isSecret: false);

        internal static Output<T> CreateSecret(Task<T> value)
            => Create(value, isSecret: true);

        internal Output<T> WithIsSecret(Task<bool> isSecret)
        {
            async Task<OutputData<T>> GetData()
            {
                var data = await this.DataTask.ConfigureAwait(false);
                return new OutputData<T>(data.Resources, data.Value, data.IsKnown, await isSecret.ConfigureAwait(false));
            }

            return new Output<T>(GetData());
        }

        private static Output<T> Create(Task<T> value, bool isSecret)
        {
            if (value == null)
            {
                throw new ArgumentNullException(nameof(value));
            }

            var tcs = new TaskCompletionSource<OutputData<T>>();
            value.Assign(tcs, t => OutputData.Create(ImmutableHashSet<Resource>.Empty, t, isKnown: true, isSecret: isSecret));
            return new Output<T>(tcs.Task);
        }

        internal static Output<T> CreateUnknown(T value)
            => Unknown(value);

        internal static Output<T> CreateUnknown(Func<Task<T>> valueFactory)
            => Unknown(default!).Apply(_ => valueFactory());

        /// <summary>
        /// <see cref="Output{T}.Apply{U}(Func{T, Output{U}})"/> for more details.
        /// </summary>
        public Output<U> Apply<U>(Func<T, U> func)
            => Apply(t => Output.Create(func(t)));

        /// <summary>
        /// <see cref="Output{T}.Apply{U}(Func{T, Output{U}})"/> for more details.
        /// </summary>
        public Output<U> Apply<U>(Func<T, Task<U>> func)
            => Apply(t => Output.Create(func(t)));

        /// <summary>
        /// <see cref="Output{T}.Apply{U}(Func{T, Output{U}})"/> for more details.
        /// </summary>
        public Output<U> Apply<U>(Func<T, Input<U>?> func)
            => Apply(t => func(t).ToOutput());

        /// <summary>
        /// Transforms the data of this <see cref="Output{T}"/> with the provided <paramref
        /// name="func"/>. The result remains an <see cref="Output{T}"/> so that dependent resources
        /// can be properly tracked.
        /// <para/>
        /// <paramref name="func"/> is not allowed to make resources.
        /// <para/>
        /// <paramref name="func"/> can return other <see cref="Output{T}"/>s.  This can be handy if
        /// you have an <c>Output&lt;SomeType&gt;</c> and you want to get a transitive dependency of
        /// it.  i.e.:
        ///
        /// <code>
        /// Output&lt;SomeType&gt; d1 = ...;
        /// Output&lt;OtherType&gt; d2 = d1.Apply(v => v.OtherOutput); // getting an output off of 'v'.
        /// </code>
        ///
        /// In this example, taking a dependency on d2 means a resource will depend on all the resources
        /// of d1.  It will <b>not</b> depend on the resources of v.x.y.OtherDep.
        /// <para/>
        /// Importantly, the Resources that d2 feels like it will depend on are the same resources
        /// as d1. If you need have multiple <see cref="Output{T}"/>s and a single <see
        /// cref="Output{T}"/> is needed that combines both set of resources, then <see
        /// cref="Output.All{T}(Input{T}[])"/> or <see cref="Output.Tuple{X, Y, Z}(Input{X}, Input{Y}, Input{Z})"/>
        /// should be used instead.
        /// <para/>
        /// This function will only be called execution of a <c>pulumi up</c> request.  It will not
        /// run during <c>pulumi preview</c> (as the values of resources are of course not known
        /// then).
        /// </summary>
        public Output<U> Apply<U>(Func<T, Output<U>?> func)
            => new Output<U>(ApplyHelperAsync(DataTask, func));

        private static async Task<OutputData<U>> ApplyHelperAsync<U>(
            Task<OutputData<T>> dataTask, Func<T, Output<U>?> func)
        {
            var data = await dataTask.ConfigureAwait(false);
            var resources = data.Resources;
            // During previews only perform the apply if the engine was able to
            // give us an actual value for this Output.
            if (!data.IsKnown && Deployment.Instance.IsDryRun)
            {
                return new OutputData<U>(resources, default!, isKnown: false, data.IsSecret);
            }

            var inner = func(data.Value);
            if (inner == null)
            {
                return OutputData.Create(resources, default(U)!, data.IsKnown, data.IsSecret);
            }

            var innerData = await inner.DataTask.ConfigureAwait(false);

            return OutputData.Create(
                data.Resources.Union(innerData.Resources), innerData.Value,
                data.IsKnown && innerData.IsKnown, data.IsSecret || innerData.IsSecret);
        }

        internal static Output<ImmutableArray<T>> All(ImmutableArray<Input<T>> inputs)
            => new Output<ImmutableArray<T>>(AllHelperAsync(inputs));

        private static async Task<OutputData<ImmutableArray<T>>> AllHelperAsync(ImmutableArray<Input<T>> inputs)
        {
            var resources = ImmutableHashSet.CreateBuilder<Resource>();
            var values = ImmutableArray.CreateBuilder<T>(inputs.Length);
            var isKnown = true;
            var isSecret = false;
            foreach (var input in inputs)
            {
                var output = (Output<T>)input;
                var data = await output.DataTask.ConfigureAwait(false);

                values.Add(data.Value);
                resources.UnionWith(data.Resources);
                (isKnown, isSecret) = OutputData.Combine(data, isKnown, isSecret);
            }

            return OutputData.Create(resources.ToImmutable(), values.MoveToImmutable(), isKnown, isSecret);
        }

        internal static Output<(T1, T2, T3, T4, T5, T6, T7, T8)> Tuple<T1, T2, T3, T4, T5, T6, T7, T8>(
            Input<T1> item1, Input<T2> item2, Input<T3> item3, Input<T4> item4,
            Input<T5> item5, Input<T6> item6, Input<T7> item7, Input<T8> item8)
            => new Output<(T1, T2, T3, T4, T5, T6, T7, T8)>(
                TupleHelperAsync(item1, item2, item3, item4, item5, item6, item7, item8));

        private static async Task<OutputData<(T1, T2, T3, T4, T5, T6, T7, T8)>> TupleHelperAsync<T1, T2, T3, T4, T5, T6, T7, T8>(
            Input<T1> item1, Input<T2> item2, Input<T3> item3, Input<T4> item4,
            Input<T5> item5, Input<T6> item6, Input<T7> item7, Input<T8> item8)
        {
            var resources = ImmutableHashSet.CreateBuilder<Resource>();
            (T1, T2, T3, T4, T5, T6, T7, T8) tuple = default;
            var isKnown = true;
            var isSecret = false;

#pragma warning disable 8601
            Update(await GetData(item1).ConfigureAwait(false), ref tuple.Item1);
            Update(await GetData(item2).ConfigureAwait(false), ref tuple.Item2);
            Update(await GetData(item3).ConfigureAwait(false), ref tuple.Item3);
            Update(await GetData(item4).ConfigureAwait(false), ref tuple.Item4);
            Update(await GetData(item5).ConfigureAwait(false), ref tuple.Item5);
            Update(await GetData(item6).ConfigureAwait(false), ref tuple.Item6);
            Update(await GetData(item7).ConfigureAwait(false), ref tuple.Item7);
            Update(await GetData(item8).ConfigureAwait(false), ref tuple.Item8);
#pragma warning restore 8601

            return OutputData.Create(resources.ToImmutable(), tuple, isKnown, isSecret);

            static async Task<OutputData<X>> GetData<X>(Input<X> input)
            {
                var output = (Output<X>)input;
                return await output.DataTask.ConfigureAwait(false);
            }

            void Update<X>(OutputData<X> data, ref X location)
            {
                resources.UnionWith(data.Resources);
                location = data.Value;
                (isKnown, isSecret) = OutputData.Combine(data, isKnown, isSecret);
            }
        }

        internal static Output<T> Unknown(T value) => new Output<T>(UnknownHelperAsync(value));

        private static Task<OutputData<T>> UnknownHelperAsync(T value)
            => Task.FromResult(new OutputData<T>(ImmutableHashSet<Resource>.Empty, value, isKnown: false, isSecret: false));
    }
}
