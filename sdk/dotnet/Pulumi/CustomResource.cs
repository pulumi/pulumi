using Google.Protobuf.WellKnownTypes;
using Pulumirpc;
using System;
using System.Collections.Generic;
using System.Threading.Tasks;

namespace Pulumi {

    public abstract class CustomResource : Resource {

        protected Task<Pulumirpc.RegisterResourceResponse> m_registrationResponse;
        public Task<string> Id { get; private set;}
        TaskCompletionSource<string> m_IdCompletionSoruce;
        public CustomResource()  {
            m_IdCompletionSoruce = new TaskCompletionSource<string>();
            Id = m_IdCompletionSoruce.Task;
        }

        protected override void OnResourceRegistrationCompete(Task<RegisterResourceResponse> resp) {
            base.OnResourceRegistrationCompete(resp);
            if (resp.IsCanceled) {
                m_IdCompletionSoruce.SetCanceled();
            } else if (resp.IsFaulted) {
                m_IdCompletionSoruce.SetException(resp.Exception);
            } else {
                m_IdCompletionSoruce.SetResult(resp.Result.Id);
            }
        }
    }
}