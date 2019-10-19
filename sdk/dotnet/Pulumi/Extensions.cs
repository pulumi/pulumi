// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    internal static class Extensions
    {
        public static bool AddRange<T>(this HashSet<T> to, IEnumerable<T> values)
        {
            var result = false;
            foreach (var value in values)
            {
                result |= to.Add(value);
            }

            return result;
        }

        public static void Deconstruct<K, V>(this KeyValuePair<K, V> pair, out K key, out V value)
        {
            key = pair.Key;
            value = pair.Value;
        }

        public static Output<object?> ToObjectOutput(this object obj)
        {
            var output = obj is IInput input ? input.ToOutput() : obj as IOutput;
            return output != null
                ? new Output<object?>(output.Resources, output.GetDataAsync())
                : Output.Create<object?>(obj);
        }

        public static ImmutableArray<TResult> SelectAsArray<TItem, TResult>(this ImmutableArray<TItem> items, Func<TItem, TResult> map)
            => ImmutableArray.CreateRange(items, map);

        public static void Assign<X, Y>(
            this Task<X> response, TaskCompletionSource<Y> tcs, Func<X, Y> extract)
        {
            response.ContinueWith(t =>
            {
                switch (t.Status)
                {
                    default: throw new InvalidOperationException("Task was not complete: " + t.Status);
                    case TaskStatus.Canceled: tcs.SetCanceled(); return;
                    case TaskStatus.Faulted: tcs.SetException(t.Exception.InnerExceptions); return;
                    case TaskStatus.RanToCompletion:
                        try
                        {
                            tcs.SetResult(extract(t.Result));
                        }
                        catch (Exception e)
                        {
                            tcs.TrySetException(e);
                        }
                        return;
                }
            });
        }
    }
}
