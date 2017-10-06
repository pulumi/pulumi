export var sum = {
	diff: function() {
		return { replaces: undefined };
	},
	create: function(inputs: any): any {
		let left: number = inputs.left;
		let right: number = inputs.right;
		return { resource: { sum: left + right }, outs: [ "sum" ] };
	},
	update: function(r: any, olds: any, news: any): any {
		let left: number = news.left;
		let right: number = news.right;
		r.sum = left + right;
		return { outs: ["sum"] };
	},
	delete: function(r: any, properties: any): any {
	}
};
