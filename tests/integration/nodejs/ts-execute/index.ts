// This is the provider from /tests/testprovider folder, installed by integration test code
import * as testprovider from "./sdks/testprovider";

// Create a resource
const resource = new testprovider.Named("resource", {
  name: "some-name"
});

if (resource.name === undefined) {
  throw new Error("This test has failed - the class field ended up being `undefined` because of definite assertion operator, or some other error.")
}
