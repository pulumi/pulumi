// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi.Testing;

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
                return ImmutableDictionary<string, object?>.Empty;
            });

        /// <summary>
        /// <see cref="RunAsync(Func{Task{IDictionary{string, object}}})"/> for more details.
        /// </summary>
        /// <param name="func"></param>
        /// <returns></returns>
        public static Task<int> RunAsync(Func<IDictionary<string, object?>> func)
            => RunAsync(() => Task.FromResult(func()));

        /// <summary>
        /// <see cref="RunAsync(Func{Task{IDictionary{string, object}}})"/> is an
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
        public static Task<int> RunAsync(Func<Task<IDictionary<string, object?>>> func)
            => CreateRunner().RunAsync(func);

        /// <summary>
        /// <see cref="RunAsync{TStack}()"/> is an entry-point to a Pulumi
        /// application. .NET applications should perform all startup logic they
        /// need in their <c>Main</c> method and then end with:
        /// <para>
        /// <c>
        /// static Task&lt;int&gt; Main(string[] args) {// program
        /// initialization code ...
        ///
        ///     return Deployment.Run&lt;MyStack&gt;();}
        /// </c>
        /// </para>
        /// <para>
        /// Deployment will instantiate a new stack instance based on the type
        /// passed as TStack type parameter. Importantly, cloud resources cannot
        /// be created outside of the <see cref="Stack"/> component.
        /// </para>
        /// <para>
        /// Because cloud Resource construction is inherently asynchronous, the
        /// result of this function is a <see cref="Task{T}"/> which should then
        /// be returned or awaited.  This will ensure that any problems that are
        /// encountered during the running of the program are properly reported.
        /// Failure to do this may lead to the program ending early before all
        /// resources are properly registered.
        /// </para>
        /// </summary>
        public static Task<int> RunAsync<TStack>() where TStack : Stack, new()
            => CreateRunner().RunAsync<TStack>();

        /// <summary>
        /// Entry point to test a Pulumi application. Deployment will
        /// instantiate a new stack instance based on the type passed as TStack
        /// type parameter. This method creates no real resources.
        /// </summary>
        /// <typeparam name="TStack">The type of the stack to test.</typeparam>
        /// <param name="stub">Optional stub for the deployment hooks.</param>
        /// <returns>Test outcome, including any errors and created
        /// resources.</returns>
        public static Task<TestResult> TestAsync<TStack>(ITestContext? stub = null) where TStack : Stack, new()
        {
            var tester = new Tester(stub);
            Instance = tester;
            return tester.TestAsync<TStack>();
        }

        private static IRunner CreateRunner()
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
            return deployment._runner;
        }
    }
}
