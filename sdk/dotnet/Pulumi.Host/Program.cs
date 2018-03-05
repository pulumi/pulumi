using Grpc.Core;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp.Scripting;
using Microsoft.CodeAnalysis.Scripting;
using Microsoft.CodeAnalysis.Scripting.Hosting;
using Mono.Options;
using Pulumi;
using Pulumirpc;
using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.IO;
using System.Runtime.InteropServices;

namespace Pulumi.Host
{
    class Program
    {
        static void Main(string[] args)
        {
            string monitor = "";
            string engine = "";
            string project = "";
            string stack = "";
            string pwd = "";
            string dryRun = "";
            int parallel = 1;
            string tracing = "";

            OptionSet o = new OptionSet {
                {"monitor=", "", m => monitor = m },
                {"engine=", "", e => engine = e},
                {"project=", "", p => project = p},
                {"stack=", "", s => stack = s },
                {"pwd=", "", wd => pwd = wd},
                {"dry_run=", dry => dryRun = dry},
                {"parallel=", (int n) => parallel = n},
                {"tracing=", t => tracing = t},
            };

            List<string> extra = o.Parse(args);

            Channel engineChannel = new Channel(engine, ChannelCredentials.Insecure);
            Channel monitorChannel = new Channel(monitor, ChannelCredentials.Insecure);

            Runtime.Initialize(new Runtime.Settings(new Engine.EngineClient(engineChannel),
                               new ResourceMonitor.ResourceMonitorClient(monitorChannel),
                               stack, project, parallel, true));

            Console.WriteLine($"Running with \U0001F379 on {RuntimeInformation.FrameworkDescription} on {RuntimeInformation.OSDescription}");

            Script<object> script = CSharpScript.Create(File.OpenRead("main.csx"));
            script.Compile();
            Runtime.RunInStack(() => {
                script.RunAsync().Wait();
            });

            engineChannel.ShutdownAsync().Wait();
            monitorChannel.ShutdownAsync().Wait();
        }
    }
}

