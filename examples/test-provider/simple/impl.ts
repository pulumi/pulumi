let fs = require("fs");

const ResourceFileName: string = "resources.json";
let resources: any;

exports.configure = function() {
	resources = JSON.parse(fs.readFileSync(ResourceFileName));
}

exports.invoke = function() {}
exports.check = function() {}
exports.diff = function() {}
exports.update = function() {}
exports.delete = function() {}

exports.create = function() {
	
}
