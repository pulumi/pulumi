TODO:
- [ ] Serialize `opts` through to remote process
- [ ] Support `Resource`s as outputs (both `CustomResource` and `ComponentResource`)
- [ ] Support `Resource`s as inputs (both `CustomResource` and `ComponentResource`)
- [ ] Real multi-lang: replace `remote.construct` with an engine invoke (or RPC) that spwans
  language host, loads the requested module, evals the requested constructor, returns back the URN.
- [ ] Support client runtime in Python (and .NET and Go)
- [ ] Generate proxies from schema
