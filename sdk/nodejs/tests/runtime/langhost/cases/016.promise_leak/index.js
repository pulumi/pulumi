const debuggable = require("../../../../../runtime/debuggable");

const leakedPromise = debuggable.debuggablePromise(new Promise((resolve, reject) => {}));
