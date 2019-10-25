// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    public partial class Deployment
    {
        /// <summary>
        /// <see cref="RunAsync(Func{Task{IDictionary{string, object}}})"/> for more details.
        /// </summary>
        public static Task<int> RunAsync(Action action)
            => RunAsync(() =>
            {
                action();
                return ImmutableDictionary<string, object>.Empty;
            });

        /// <summary>
        /// <see cref="RunAsync(Func{Task{IDictionary{string, object}}})"/> for more details.
        /// </summary>
        /// <param name="func"></param>
        /// <returns></returns>
        public static Task<int> RunAsync(Func<IDictionary<string, object>> func)
            => RunAsync(() => Task.FromResult(func()));

        /// <summary>
        /// <see cref="RunAsync(Func{Task{IDictionary{string, object}}})"/> is the
        /// entry-point to a Pulumi application. .NET applications should perform all startup logic
        /// they need in their <c>Main</c> method and then end with:
        /// <para>
        /// <c>
        /// static Task&lt;int&gt; Main(string[] args)
        /// {
        ///     // program initialization code ...
        ///     
        ///     return Deployment.Run(async () =>
        ///     {
        ///         // Code that creates resources.
        ///     });
        /// }
        /// </c>
        /// </para>
        /// Importantly: Cloud resources cannot be created outside of the lambda passed to any of the
        /// <see cref="Deployment.RunAsync(Action)"/> overloads.  Because cloud Resource construction is
        /// inherently asynchronous, the result of this function is a <see cref="Task{T}"/> which should
        /// then be returned or awaited.  This will ensure that any problems that are encountered during
        /// the running of the program are properly reported.  Failure to do this may lead to the
        /// program ending early before all resources are properly registered.
        /// <para/>
        /// The function passed to <see cref="RunAsync(Func{Task{IDictionary{string, object}}})"/>
        /// can optionally return an <see cref="IDictionary{TKey, TValue}"/>.  The keys and values
        /// in this dictionary will become the outputs for the Pulumi Stack that is created.
        /// </summary>
        public static Task<int> RunAsync(Func<Task<IDictionary<string, object>>> func)
        {
            // Serilog.Log.Logger = new LoggerConfiguration().MinimumLevel.Debug().WriteTo.Console().CreateLogger();

            Serilog.Log.Debug("Deployment.Run called.");
            if (_instance != null)
            {
                throw new NotSupportedException("Deployment.Run can only be called a single time.");
            }

            Serilog.Log.Debug("Creating new Deployment.");
            var deployment = new Deployment();
            Instance = deployment;
            return deployment._runner.RunAsync(func);
        }
    }
}
