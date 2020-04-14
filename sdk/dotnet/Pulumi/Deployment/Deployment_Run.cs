// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Pulumi.Testing;

namespace Pulumi
{
    public partial class Deployment
    {
        /// <summary>
        /// <see cref="RunAsync(Func{Task{IDictionary{string, object}}}, StackOptions)"/> for more details.
        /// </summary>
        /// <param name="action">Callback that creates stack resources.</param>
        public static Task<int> RunAsync(Action action)
            => RunAsync(() =>
            {
                action();
                return ImmutableDictionary<string, object?>.Empty;
            });

        /// <summary>
        /// <see cref="RunAsync(Func{Task{IDictionary{string, object}}}, StackOptions)"/> for more details.
        /// </summary>
        /// <param name="func">Callback that creates stack resources.</param>
        /// <returns>A dictionary of stack outputs.</returns>
        public static Task<int> RunAsync(Func<IDictionary<string, object?>> func)
            => RunAsync(() => Task.FromResult(func()));
        
        /// <summary>
        /// <see cref="RunAsync(Func{Task{IDictionary{string, object}}}, StackOptions)"/> for more details.
        /// </summary>
        /// <param name="func">Callback that creates stack resources.</param>
        public static Task<int> RunAsync(Func<Task> func)
            => RunAsync(async () =>
            {
                await func();
                return ImmutableDictionary<string, object?>.Empty;
            });

        /// <summary>
        /// <see cref="RunAsync(Func{Task{IDictionary{string, object}}}, StackOptions)"/> is an
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
        /// The function passed to <see cref="RunAsync(Func{Task{IDictionary{string, object}}}, StackOptions)"/>
        /// can optionally return an <see cref="IDictionary{TKey, TValue}"/>.  The keys and values
        /// in this dictionary will become the outputs for the Pulumi Stack that is created.
        /// </summary>
        /// <param name="func">Callback that creates stack resources.</param>
        /// <param name="options">Stack options.</param>
        public static Task<int> RunAsync(Func<Task<IDictionary<string, object?>>> func, StackOptions? options = null)
            => CreateRunner().RunAsync(func, options);

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
        /// Note: Currently, unit tests that call <see cref="TestAsync{TStack}"/>
        /// must run serially; parallel execution is not supported.
        /// </summary>
        /// <param name="mocks">Hooks to mock the engine calls.</param>
        /// <param name="options">Optional settings for the test run.</param>
        /// <typeparam name="TStack">The type of the stack to test.</typeparam>
        /// <returns>Test result containing created resources and errors, if any.</returns>
        public static async Task<ImmutableArray<Resource>> TestAsync<TStack>(IMocks mocks, TestOptions? options = null) where TStack : Stack, new()
        {
            var engine = new MockEngine();
            var monitor = new MockMonitor(mocks);
            Deployment deployment;
            lock (_instanceLock)
            {
                if (_instance != null)
                    throw new NotSupportedException($"Mulitple executions of {nameof(TestAsync)} must run serially. Please configure your unit test suite to run tests one-by-one.");

                deployment = new Deployment(engine, monitor, options);
                Instance = new DeploymentInstance(deployment);
            }

            try
            {
                await deployment._runner.RunAsync<TStack>();
                return engine.Errors.Count switch
                {
                    1 => throw new RunException(engine.Errors.Single()),
                    int v when v > 1 => throw new AggregateException(engine.Errors.Select(e => new RunException(e))),
                    _ => monitor.Resources.ToImmutableArray()
                };
            }
            finally
            {
                lock (_instanceLock)
                {
                    _instance = null;
                }
            }
        }

        private static IRunner CreateRunner()
        {
            // Serilog.Log.Logger = new LoggerConfiguration().MinimumLevel.Debug().WriteTo.Console().CreateLogger();

            Serilog.Log.Debug("Deployment.Run called.");
            lock (_instanceLock)
            {
                if (_instance != null)
                    throw new NotSupportedException("Deployment.Run can only be called a single time.");

                Serilog.Log.Debug("Creating new Deployment.");
                var deployment = new Deployment();
                Instance = new DeploymentInstance(deployment);
                return deployment._runner;
            }
        }
    }
}
