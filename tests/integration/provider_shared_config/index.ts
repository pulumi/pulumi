import * as random from "@pulumi/random";
import * as tls from "@pulumi/tls";

// Create an explicit Provider.
new tls.Provider("test", {
    proxy: {
        url: "http://override",
    },
});

// Create a resource that uses a default Provider.
new random.RandomString("example", {length: 8});
