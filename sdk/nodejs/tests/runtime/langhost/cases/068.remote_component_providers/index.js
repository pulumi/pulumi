// This tests that providers is passed to RegisterResource for remote components, whether specified via
// `provider` or `providers`.

import pulumi from "../../../../../index.js";

class Provider extends pulumi.ProviderResource {
  constructor(name, opts) {
    super("test", name, {}, opts);
  }
}

class RemoteComponent extends pulumi.ComponentResource {
  constructor(name, opts) {
    super("test:index:Component", name, {}, opts, true /*remote*/);
  }
}

const myprovider = new Provider("myprovider");

new RemoteComponent("singular", { provider: myprovider });
new RemoteComponent("map", { providers: { test: myprovider } });
new RemoteComponent("array", { providers: [myprovider] });
