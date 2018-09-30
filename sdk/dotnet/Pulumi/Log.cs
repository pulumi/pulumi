using Pulumirpc;

namespace Pulumi {
    public static class Log {

        public static void Debug(string message) {
            Runtime.Engine.Log(new Pulumirpc.LogRequest {
                Severity = LogSeverity.Debug,
                Message = message
            });
        }

        public static void Info(string message) {
            Runtime.Engine.Log(new Pulumirpc.LogRequest {
                Severity = LogSeverity.Info,
                Message = message
            });
        }

        public static void Warning(string message) {
            Runtime.Engine.Log(new Pulumirpc.LogRequest {
                Severity = LogSeverity.Warning,
                Message = message
            });
        }

        public static void Error(string message) {
            Runtime.Engine.Log(new Pulumirpc.LogRequest {
                Severity = LogSeverity.Error,
                Message = message
            });
        }

        public static void Debug(string format, params object[] args) {
            Runtime.Engine.Log(new Pulumirpc.LogRequest {
                Severity = LogSeverity.Debug,
                Message = string.Format(format, args)
            });
        }

        public static void Info(string format, params object[] args) {
            Runtime.Engine.Log(new Pulumirpc.LogRequest {
                Severity = LogSeverity.Info,
                Message = string.Format(format, args)
            });
        }

        public static void Warning(string format, params object[] args) {
            Runtime.Engine.Log(new Pulumirpc.LogRequest {
                Severity = LogSeverity.Warning,
                Message = string.Format(format, args)
            });
        }

        public static void Error(string format, params object[] args) {
            Runtime.Engine.Log(new Pulumirpc.LogRequest {
                Severity = LogSeverity.Error,
                Message = string.Format(format, args)
            });
        }
    }
}
