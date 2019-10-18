// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using System.Threading.Tasks;

namespace Pulumi
{
    public static class Output
    {
        public static Output<T> Create<T>([MaybeNull]T value)
            => Create(Task.FromResult(value));

        public static Output<T> Create<T>(Task<T> value)
            => Output<T>.Create(value);

        public static Output<ImmutableArray<T>> All<T>(params Input<T>[] inputs)
            => All(ImmutableArray.CreateRange(inputs));

        public static Output<ImmutableArray<T>> All<T>(ImmutableArray<Input<T>> inputs)
            => Output<T>.All(inputs);

        public static Output<(X, Y)> Tuple<X, Y>(Input<X> item1, Input<Y> item2)
            => Tuple<X, Y, int>(item1, item2, 0).Apply(v => (v.Item1, v.Item2));

        public static Output<(X, Y, Z)> Tuple<X, Y, Z>(Input<X> item1, Input<Y> item2, Input<Z> item3)
            => Output<(X, Y, Z)>.Tuple(item1, item2, item3);

        public static Output<string> Format(FormattableString formattableString)
        {
            var arguments = formattableString.GetArguments();
            var inputs = new Input<object?>[arguments.Length];

            for (var i = 0; i < arguments.Length; i++)
            {
                var arg = arguments[i];
                inputs[i] = arg is IProvidesOutputOfObj provider
                    ? provider.OutputOfObj
                    : Create<object?>(arg);
            }

            return All(inputs).Apply(objs =>
                string.Format(formattableString.Format, objs.ToArray()));
         }
    }

    public class Output<T> : IProvidesOutputOfObj
    {
        private readonly Task<OutputData<T>> _dataTask;

        internal async Task<T> GetValue()
        {
            var data = await _dataTask.ConfigureAwait(false);
            return data.Value;
        }

        Output<object?> IProvidesOutputOfObj.OutputOfObj
            => this.Apply(v => (object?)v);

        internal async Task<bool> IsKnown()
        {
            var data = await _dataTask.ConfigureAwait(false);
            return data.IsKnown;
        }

        internal async Task<bool> IsSecret()
        {
            var data = await _dataTask.ConfigureAwait(false);
            return data.IsSecret;
        }

        private Output(Task<OutputData<T>> dataTask)
            => _dataTask = dataTask;

        public static Output<T> Create(Task<T> value)
        {
            var tcs = new TaskCompletionSource<OutputData<T>>();
            value.Assign(tcs, t => new OutputData<T>(t, isKnown: true, isSecret: false));
            return new Output<T>(tcs.Task);
        }

        public Output<U> Apply<U>(Func<T, U> func)
            => Apply(t => Output.Create(func(t)));

        public Output<U> Apply<U>(Func<T, Output<U>> func)
            => new Output<U>(ApplyHelperAsync(_dataTask, func));

        private static async Task<OutputData<U>> ApplyHelperAsync<U>(
            Task<OutputData<T>> dataTask, Func<T, Output<U>> func)
        {
            var data = await dataTask.ConfigureAwait(false);

            var inner = func(data.Value);
            var innerData = await inner._dataTask.ConfigureAwait(false);

            return new OutputData<U>(
                innerData.Value, data.IsKnown && innerData.IsKnown, data.IsSecret || innerData.IsSecret);
        }

        internal static Output<ImmutableArray<T>> All(ImmutableArray<Input<T>> inputs)
            => new Output<ImmutableArray<T>>(AllHelperAsync(inputs));

        private static async Task<OutputData<ImmutableArray<T>>> AllHelperAsync(ImmutableArray<Input<T>> inputs)
        {
            var values = ImmutableArray.CreateBuilder<T>(inputs.Length);
            var isKnown = true;
            var isSecret = false;
            foreach (var input in inputs)
            {
                var output = (Output<T>)input;
                var data = await output._dataTask.ConfigureAwait(false);

                values.Add(data.Value);
                (isKnown, isSecret) = Combine(data, isKnown, isSecret);
            }

            return new OutputData<ImmutableArray<T>>(values.MoveToImmutable(), isKnown, isSecret);
        }

        private static (bool isKnown, bool isSecret) Combine<X>(OutputData<X> data, bool isKnown, bool isSecret)
            => (isKnown && data.IsKnown, isSecret || data.IsSecret);

        internal static Output<(X, Y, Z)> Tuple<X, Y, Z>(Input<X> item1, Input<Y> item2, Input<Z> item3)
            => new Output<(X, Y, Z)>(TupleHelperAsync(item1, item2, item3));

        private static async Task<OutputData<(X, Y, Z)>> TupleHelperAsync<X, Y, Z>(Input<X> item1, Input<Y> item2, Input<Z> item3)
        {
            (X, Y, Z) tuple;
            var isKnown = true;
            var isSecret = false;

            {
                var output = (Output<X>)item1;
                var data = await output._dataTask.ConfigureAwait(false);
                tuple.Item1 = data.Value;
                (isKnown, isSecret) = Combine(data, isKnown, isSecret);
            }

            {
                var output = (Output<Y>)item2;
                var data = await output._dataTask.ConfigureAwait(false);
                tuple.Item2 = data.Value;
                (isKnown, isSecret) = Combine(data, isKnown, isSecret);
            }

            {
                var output = (Output<Z>)item3;
                var data = await output._dataTask.ConfigureAwait(false);
                tuple.Item3 = data.Value;
                (isKnown, isSecret) = Combine(data, isKnown, isSecret);
            }

            return new OutputData<(X, Y, Z)>(tuple, isKnown, isSecret);
        }
    }

    internal struct OutputData<X>
    {
        public readonly X Value;
        public readonly bool IsKnown;
        public readonly bool IsSecret;

        public OutputData(X value, bool isKnown, bool isSecret)
        {
            Value = value;
            IsKnown = isKnown;
            IsSecret = isSecret;
        }
    }
}
