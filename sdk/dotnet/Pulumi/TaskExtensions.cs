using System;
using System.Threading.Tasks;

namespace Pulumi
{
    public static class TaskExtensions
    {
        public static async Task<TResult> Select<TSource, TResult>(this Task<TSource> source, Func<TSource, TResult> selector) {
            return selector(await source);
        }
        public static async Task<TResult> SelectMany<TSource, TResult>(this Task<TSource> source, Func<TSource, Task<TResult>> selector) {
            return await selector(await source);
        }

        public static async Task<TResult> Zip<TFirst, TSecond, TResult>(
            this Task<TFirst> first, Task<TSecond> second, Func<TFirst, TSecond, TResult> resultSelector) {
            return resultSelector(await first, await second);
        }

        public static async Task<T> Catch<T>(this Task<T> source, Func<Exception, T> handler) {
            try {
                return await source;
            } catch (Exception e) {
                return handler(e);
            }
        }
    }
}