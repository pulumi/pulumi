TODO:
- [x] Support `ComponentResource`s as outputs
- [x] Support `CustomResource`s as outputs 
- [x] Serialize `opts` through to remote process
- [x] First class `Resource` on RPC (instead of string matching `urn:pulumi` )
- [x] Replace `remote.construct` with a real process spawn and gRPC `construct` API
- [ ] Support client runtime in Python (and .NET and Go)
- [ ] Support EKS package
- [ ] Support `provider`/`providers` opts (ProviderResource hydration)
- [ ] Support `parent` opts (hydration of arbitrary Resources, even if no proxy exists?)
- [ ] Support `ComponentResource`s as inputs
- [ ] Support `CustomResource`s as inputs
- [ ] Generate proxies from schema

