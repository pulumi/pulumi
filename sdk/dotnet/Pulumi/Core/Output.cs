// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using System.Threading.Tasks;
using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// Useful static utility methods for both creating and working wit <see cref="Output{T}"/>s.
    /// </summary>
    public static class Output
    {
        public static Output<T> Create<T>([MaybeNull]T value)
            => Create(Task.FromResult(value));

        public static Output<T> Create<T>(Task<T> value)
            => Output<T>.Create(value);

        public static Output<T> CreateSecret<T>([MaybeNull]T value)
            => CreateSecret(Task.FromResult(value));

        public static Output<T> CreateSecret<T>(Task<T> value)
            => Output<T>.CreateSecret(value);

        /// <summary>
        /// Combines all the <see cref="Input{T}"/> values in <paramref name="inputs"/> and combines
        /// them all into a single <see cref="Output{T}"/> with an <see cref="ImmutableArray{T}"/>
        /// containing all their underlying values.  If any of the <see cref="Input{T}"/>s are not
        /// known, the final result will be not known.  Similarly, if any of the <see
        /// cref="Input{T}"/>s are secrets, then the final result will be a secret.
        /// </summary>
        public static Output<ImmutableArray<T>> All<T>(params Input<T>[] inputs)
            => All(ImmutableArray.CreateRange(inputs));

        /// <summary>
        /// <see cref="All{T}(Input{T}[])"/> for more details.
        /// </summary>
        public static Output<ImmutableArray<T>> All<T>(ImmutableArray<Input<T>> inputs)
            => Output<T>.All(inputs);

        /// <summary>
        /// <see cref="Tuple{X, Y, Z}(Input{X}, Input{Y}, Input{Z})"/> for more details.
        /// </summary>
        public static Output<(X, Y)> Tuple<X, Y>(Output<X> item1, Output<Y> item2)
            => Tuple((Input<X>)item1, (Input<Y>)item2);

        /// <summary>
        /// <see cref="Tuple{X, Y, Z}(Input{X}, Input{Y}, Input{Z})"/> for more details.
        /// </summary>
        public static Output<(X, Y)> Tuple<X, Y>(Input<X> item1, Input<Y> item2)
            => Tuple<X, Y, int>(item1, item2, 0).Apply(v => (v.Item1, v.Item2));

        /// <summary>
        /// Combines all the <see cref="Input{T}"/> values in the provided parameters and combines
        /// them all into a single tuple containing each of their underlying values.  If any of the
        /// <see cref="Input{T}"/>s are not known, the final result will be not known.  Similarly,
        /// if any of the <see cref="Input{T}"/>s are secrets, then the final result will be a
        /// secret.
        /// </summary>
        public static Output<(X, Y, Z)> Tuple<X, Y, Z>(Input<X> item1, Input<Y> item2, Input<Z> item3)
            => Output<(X, Y, Z)>.Tuple(item1, item2, item3);

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
        ImmutableHashSet<Resource> Resources { get; }

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
        internal ImmutableHashSet<Resource> Resources { get; private set; }
        internal Task<OutputData<T>> DataTask { get; private set; }

        internal Output(ImmutableHashSet<Resource> resources, Task<OutputData<T>> dataTask)
        {
            Resources = resources;
            DataTask = dataTask;
        }

        internal async Task<T> GetValueAsync()
        {
            var data = await DataTask.ConfigureAwait(false);
            return data.Value;
        }

        ImmutableHashSet<Resource> IOutput.Resources => this.Resources;

        async Task<OutputData<object?>> IOutput.GetDataAsync()
            => await DataTask.ConfigureAwait(false);

        public static Output<T> Create(Task<T> value)
            => Create(value, isSecret: false);

        internal static Output<T> CreateSecret(Task<T> value)
            => Create(value, isSecret: true);

        private static Output<T> Create(Task<T> value, bool isSecret)
        {
            if (value == null)
            {
                throw new ArgumentNullException(nameof(value));
            }

            var tcs = new TaskCompletionSource<OutputData<T>>();
            value.Assign(tcs, t => OutputData.Create(t, isKnown: true, isSecret: isSecret));
            return new Output<T>(ImmutableHashSet<Resource>.Empty, tcs.Task);
        }

        /// <summary>
        /// <see cref="Apply{U}(Func{T, Output{U}})"/> for more details.
        /// </summary>
        public Output<U> Apply<U>(Func<T, U> func)
            => Apply(t => Output.Create(func(t)));

        /// <summary>
        /// <see cref="Apply{U}(Func{T, Output{U}})"/> for more details.
        /// </summary>
        public Output<U> Apply<U>(Func<T, Task<U>> func)
            => Apply(t => Output.Create(func(t)));

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
        public Output<U> Apply<U>(Func<T, Output<U>> func)
            => new Output<U>(Resources, ApplyHelperAsync(DataTask, func));

        private static async Task<OutputData<U>> ApplyHelperAsync<U>(
            Task<OutputData<T>> dataTask, Func<T, Output<U>> func)
        {
            var data = await dataTask.ConfigureAwait(false);

            // During previews only perform the apply if the engine was able to
            // give us an actual value for this Output.
            if (!data.IsKnown && Deployment.Instance.IsDryRun)
            {
                return new OutputData<U>(default!, isKnown: false, data.IsSecret);
            }

            var inner = func(data.Value);
            var innerData = await inner.DataTask.ConfigureAwait(false);

            return OutputData.Create(
                innerData.Value, data.IsKnown && innerData.IsKnown, data.IsSecret || innerData.IsSecret);
        }

        internal static Output<ImmutableArray<T>> All(ImmutableArray<Input<T>> inputs)
            => new Output<ImmutableArray<T>>(GetAllResources(inputs), AllHelperAsync(inputs));

        private static async Task<OutputData<ImmutableArray<T>>> AllHelperAsync(ImmutableArray<Input<T>> inputs)
        {
            var values = ImmutableArray.CreateBuilder<T>(inputs.Length);
            var isKnown = true;
            var isSecret = false;
            foreach (var input in inputs)
            {
                var output = (Output<T>)input;
                var data = await output.DataTask.ConfigureAwait(false);

                values.Add(data.Value);
                (isKnown, isSecret) = OutputData.Combine(data, isKnown, isSecret);
            }

            return OutputData.Create(values.MoveToImmutable(), isKnown, isSecret);
        }

        internal static Output<(X, Y, Z)> Tuple<X, Y, Z>(Input<X> item1, Input<Y> item2, Input<Z> item3)
            => new Output<(X, Y, Z)>(
                GetAllResources(new IInput[] { item1, item2, item3 }),
                TupleHelperAsync(item1, item2, item3));

        private static ImmutableHashSet<Resource> GetAllResources(IEnumerable<IInput> inputs)
            => ImmutableHashSet.CreateRange(inputs.SelectMany(i => i.ToOutput().Resources));

        private static async Task<OutputData<(X, Y, Z)>> TupleHelperAsync<X, Y, Z>(Input<X> item1, Input<Y> item2, Input<Z> item3)
        {
            (X, Y, Z) tuple;
            var isKnown = true;
            var isSecret = false;

            {
                var output = (Output<X>)item1;
                var data = await output.DataTask.ConfigureAwait(false);
                tuple.Item1 = data.Value;
                (isKnown, isSecret) = OutputData.Combine(data, isKnown, isSecret);
            }

            {
                var output = (Output<Y>)item2;
                var data = await output.DataTask.ConfigureAwait(false);
                tuple.Item2 = data.Value;
                (isKnown, isSecret) = OutputData.Combine(data, isKnown, isSecret);
            }

            {
                var output = (Output<Z>)item3;
                var data = await output.DataTask.ConfigureAwait(false);
                tuple.Item3 = data.Value;
                (isKnown, isSecret) = OutputData.Combine(data, isKnown, isSecret);
            }

            return OutputData.Create(tuple, isKnown, isSecret);
        }
    }
}
