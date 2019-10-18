// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    internal static class Extensions
    {
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
                    case TaskStatus.Canceled: tcs.SetCanceled(); break;
                    case TaskStatus.Faulted: tcs.SetException(t.Exception.InnerExceptions); break;
                    case TaskStatus.RanToCompletion:
                        try
                        {
                            tcs.SetResult(extract(t.Result));
                        }
                        catch (Exception e)
                        {
                            tcs.TrySetException(e);
                        }
                        break;
                }


                if (t.Status == TaskStatus.Canceled)
                {
                    // TODO tcs.set
                }
            });
        }
    }
}
