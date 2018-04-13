import { Resource, Provider } from "./resource";

Provider.instance.injectFault("oh no");
const a = new Resource("a", { replace: 1 });

