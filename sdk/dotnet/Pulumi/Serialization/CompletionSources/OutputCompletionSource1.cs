//// Copyright 2016-2019, Pulumi Corporation

//#nullable enable

//using System;
//using System.Collections.Immutable;
//using System.Threading.Tasks;

//namespace Pulumi.Serialization
//{
//    public abstract class OutputCompletionSource<T> : IOutputCompletionSource
//    {
//        private readonly TaskCompletionSource<OutputData<T>> _tcs = new TaskCompletionSource<OutputData<T>>();

//        public Output<T> Output { get; }

//        protected OutputCompletionSource(Resource? resource)
//            => this.Output = new Output<T>(
//                resource == null ? ImmutableHashSet<Resource>.Empty : ImmutableHashSet.Create(resource),
//                _tcs.Task);

//        private protected void SetResult(OutputData<T> data)
//            => _tcs.SetResult(data);

//        void IOutputCompletionSource.TrySetException(Exception exception)
//            => _tcs.TrySetException(exception);

//        void IOutputCompletionSource.SetDefaultResult(bool isKnown)
//            => _tcs.TrySetResult(new OutputData<T>(default!, isKnown, isSecret: false));
//    }
//}
