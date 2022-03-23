// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Concurrent;
using System.Linq;
using System.Threading.Tasks;

using Pulumi;
using Pulumi.Testing;

namespace Pulumi.Mypkg
{
    public static class TestHelpers
    {
        public static async Task<T> Run<T>(IMocks mocks, Func<Output<T>> outputGenerator, TestOptions? options = null)
        {
            options = options ?? new TestOptions();
            options.ProjectName = HelperStack.RegisterBuilderAsProjectName(outputGenerator);
            var resources = await Deployment.TestAsync<HelperStack>(mocks, options);
            var stack = resources.Where(x => x is HelperStack).First() as HelperStack;
            if (stack != null)
            {
                var result = await AwaitOutput(stack.Result);
                if (result is T)
                {
                    return (T)result;
                }
                else
                {
                    throw new Exception($"The output did not resolve to the correct type: {result}");
                }
            }
            else
            {
                throw new Exception("Did not find stack");
            }
        }

        public static Output<T> Out<T>(T x)
        {
            return Output.Create<T>(x);
        }

        public static Task<T> AwaitOutput<T>(Output<T> output)
        {
            var tcs = new TaskCompletionSource<T>();
            Task.Delay(1000).ContinueWith(_ =>
            {
                tcs.TrySetException(new Exception("Output resolution has timed out"));
            });
            output.Apply(v =>
            {
                tcs.TrySetResult(v);
                return v;
            });
            return tcs.Task;
        }

        public class HelperStack : Stack
        {
            private static ConcurrentDictionary<string,Func<Output<object?>>> registry =
                new ConcurrentDictionary<string,Func<Output<object?>>>();

            [Output]
            public Output<object?> Result { get; private set; }

            public HelperStack()
            {
                Console.WriteLine(Deployment.Instance.ProjectName);
                Func<Output<object?>>? outputBuilder;
                if (!registry.TryGetValue(Deployment.Instance.ProjectName, out outputBuilder))
                {
                    throw new Exception("Incorrect use of HelperStack");
                }
                this.Result = outputBuilder.Invoke();
            }

            public static string RegisterBuilderAsProjectName<T>(Func<Output<T>> builder)
            {
                var projectName = Guid.NewGuid().ToString();
                if (!registry.TryAdd(projectName, () => builder().Apply(x => (object?)x)))
                {
                    throw new Exception("Impossible");
                }
                return projectName;
            }
        }
    }
}
