//using Grpc.Core;
//using Pulumi;
//using Pulumirpc;
//using Serilog;
//using System;
//using System.IO;
//using System.Threading.Tasks;
//using System.Runtime.InteropServices;

//namespace Pulumi {
//    public static class Deployment {

//        // TODO(ellismg): Perhaps we should have another overload Run<T>(Func<T> f) and we use reflection over the T
//        // to get all public fields and properties of type Input<T> and set them as outputs?
//        public static void Run(Action a) {
//            if (Environment.GetEnvironmentVariable("PULUMI_DOTNET_LOGGING") != null) {
//                Serilog.Log.Logger = new LoggerConfiguration().MinimumLevel.Debug().WriteTo.Console().CreateLogger();
//            }

//            // TODO(ellismg): any one of these could be null, and we need to guard against that for ones that must
//            // be set (I don't know the set off the top of my head.  I think that everything except tracing is
//            // required.  Also, they could be bad values (e.g. parallel may not be something that can be `bool.Parsed`
//            // and we'd like to fail in a nicer manner.
//            string monitor = Environment.GetEnvironmentVariable("PULUMI_MONITOR");
//            string engine = Environment.GetEnvironmentVariable("PULUMI_ENGINE");
//            string project = Environment.GetEnvironmentVariable("PULUMI_PROJECT");
//            string stack = Environment.GetEnvironmentVariable("PULUMI_STACK");
//            string pwd = Environment.GetEnvironmentVariable("PULUMI_PWD");
//            string dryRun = Environment.GetEnvironmentVariable("PULUMI_DRY_RUN");
//            string parallel = Environment.GetEnvironmentVariable("PULUMI_PARALLEL");
//            string tracing = Environment.GetEnvironmentVariable("PULUMI_TRACING");

//            Channel engineChannel = new Channel(engine, ChannelCredentials.Insecure);
//            Channel monitorChannel = new Channel(monitor, ChannelCredentials.Insecure);

//            Runtime.Initialize(new Runtime.Settings(new Engine.EngineClient(engineChannel),
//                               new ResourceMonitor.ResourceMonitorClient(monitorChannel),
//                               stack, project, int.Parse(parallel), bool.Parse(dryRun)));

//            Console.WriteLine($"Running with \U0001F379 on {RuntimeInformation.FrameworkDescription} on {RuntimeInformation.OSDescription}");
//            Runtime.RunInStack(a);
//        }
//    }
//}