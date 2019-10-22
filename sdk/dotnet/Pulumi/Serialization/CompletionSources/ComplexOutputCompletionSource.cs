//// Copyright 2016-2019, Pulumi Corporation

//#nullable enable

//using System;
//using System.Collections.Generic;

//namespace Pulumi.Serialization
//{
//    public class ComplexOutputCompletionSource<T> : ProtobufOutputCompletionSource<T>
//    {
//        public ComplexOutputCompletionSource(
//            Resource resource,
//            Func<IDictionary<string, object>, T> convert)
//            : base(resource, CreateDeserializeFunction(convert))
//        {
//        }

//        private static Deserializer<T> CreateDeserializeFunction(Func<IDictionary<string, object>, T> convert)
//            => v =>
//            {
//                var (unwrapped, isKnown, isSecret) = Deserializers.GenericStructDeserializer(v);
//                return !isKnown
//                    ? OutputData.Create<T>(default!, isKnown: false, isSecret)
//                    : OutputData.Create(convert(unwrapped), isKnown, isSecret);
//            };
//    }
//}
