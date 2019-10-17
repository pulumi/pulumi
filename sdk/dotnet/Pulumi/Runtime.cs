//using System;
//using System.Threading.Tasks;
//using Pulumirpc;

//namespace Pulumi
//{
//    public static class Runtime
//    {
//        public static string Stack { get; private set; }

//        public static Engine.EngineClient Engine { get; private set; }

//        public static ResourceMonitor.ResourceMonitorClient Monitor { get; private set; }

//        public static string Project { get; private set; }

//        public static bool DryRun { get; private set; }

//        internal static ComponentResource Root {get; private set;}

//        public static void Initialize(Settings settings)
//        {
//            Engine = settings.Engine;
//            Monitor = settings.Monitor;
//            Project = settings.Project;
//            Stack = settings.Stack;
//            DryRun = DryRun;
//        }

//        public static void RunInStack(Action run)
//        {
//            Root = new ComponentResource("pulumi:pulumi:Stack", $"{Runtime.Project}-{Runtime.Stack}", null, ResourceOptions.None);
//            Task.Run(run).Wait();
//        }

//        public class Settings
//        {
//            public string Project { get; set; }
//            public Engine.EngineClient Engine { get; set; }
//            public ResourceMonitor.ResourceMonitorClient Monitor { get; set; }
//            public string Stack { get; set; }
//            public int Parallel { get; set; }
//            public bool DryRun { get; set; }

//            public Settings(Engine.EngineClient engineClient, ResourceMonitor.ResourceMonitorClient monitorClient, string stack, string project, int parallel, bool dryRun)
//            {
//                Engine = engineClient;
//                Monitor = monitorClient;
//                Stack = stack;
//                Project = project;
//                Parallel = parallel;
//                DryRun = dryRun;
//            }
//        }
//    }
//}