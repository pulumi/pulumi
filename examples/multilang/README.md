TODO:
- [x] Serialize `opts` through to remote process
- [x] Support `ComponentResource`s as outputs
- [x] Support `CustomResource`s as outputs 
- [ ] Support `ComponentResource`s as inputs
- [ ] Support `CustomResource`s as inputs
- [ ] Support `provider`/`providers` opts (ProviderResource hydration)
- [ ] Support `parent` opts (hydration of arbitrary Resources, even if no proxy exists?)
- [x] First class `Resource` on RPC (instead of string matching `urn:pulumi` )
- [ ] Real multi-lang: replace `remote.construct` with an engine invoke (or RPC) that spwans
  language host, loads the requested module, evals the requested constructor, returns back the URN.
- [ ] Support client runtime in Python (and .NET and Go)
- [ ] Generate proxies from schema
