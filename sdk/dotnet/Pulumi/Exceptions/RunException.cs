// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;

namespace Pulumi
{
    /// <summary>
    /// RunException can be used for terminating a program abruptly, but resulting in a clean exit
    /// rather than the usual verbose unhandled error logic which emits the source program text and
    /// complete stack trace.  This type should be rarely used.  Ideally <see
    /// cref="ResourceException"/> should always be used so that as many errors as possible can be
    /// associated with a Resource.
    /// </summary>
    internal class RunException : Exception
    {
        public RunException(string message)
            : base(message)
        {
        }

        public RunException(string message, Exception? innerException)
            : base(message, innerException)
        {
        }

        public static RunException OutputsHaveIncorrectType(
            IEnumerable<string> outputAttributeNames)
        {
            var message = $"Output(s) '{string.Join(", ", outputAttributeNames)}' have incorrect type. [Output] attributed properties must be instances of Output<T>.";
            return new RunException(message);
        }
    }
}
