using System;

namespace Pulumi {
    /// <summary>
    /// RunError can be used for terminating a program abruptly, but resulting in a clean exit rather than the usual
    /// verbose unhandled error logic which emits the source program text and complete stack trace.
    /// </summary>
    public class RunError : Exception {
        public RunError() : base() {}
        public RunError(string message) : base(message) {}
        public RunError(string message, Exception innerException) : base(message, innerException) {}
    }
}