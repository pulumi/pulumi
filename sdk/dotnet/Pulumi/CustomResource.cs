//using Google.Protobuf.WellKnownTypes;
//using Pulumirpc;
//using System;
//using System.Collections.Generic;
//using System.Threading.Tasks;

//namespace Pulumi {

//    public abstract class CustomResource : Resource {

//        protected Task<Pulumirpc.RegisterResourceResponse> m_registrationResponse;
//        public Output<string> Id { get; private set;}
//        TaskCompletionSource<OutputState<string>> m_IdCompletionSoruce;
//        public CustomResource()  {
//            m_IdCompletionSoruce = new TaskCompletionSource<OutputState<string>>();
//            Id = new Output<string>(m_IdCompletionSoruce.Task);
//        }

//        protected override void OnResourceRegistrationComplete(Task<RegisterResourceResponse> resp) {
//            base.OnResourceRegistrationComplete(resp);
//            if (resp.IsCanceled) {
//                m_IdCompletionSoruce.SetCanceled();
//            } else if (resp.IsFaulted) {
//                m_IdCompletionSoruce.SetException(resp.Exception);
//            } else {
//                Serilog.Log.Debug("Setting id to {id} for {urn} with dependency {this}", resp.Result.Id, resp.Result.Urn, this);
//                m_IdCompletionSoruce.SetResult(new OutputState<string>(resp.Result.Id, !string.IsNullOrEmpty(resp.Result.Id), this));
//            }
//        }
//    }
//}