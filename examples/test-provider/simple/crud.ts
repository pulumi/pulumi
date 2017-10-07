export var add = {
	check: function() {
		return { defaults: undefined, failures: undefined };
	},
	diff: function() {
		return { replaces: undefined };
	},
	create: function(inputs: any): any {
		let left: number = inputs.left;
		let right: number = inputs.right;
		return { id: "0", resource: { sum: left + right }, outs: [ "sum" ] };
	},
	update: function(id: string, olds: any, news: any): any {
		let left: number = news.left;
		let right: number = news.right;
		return { id: id, resource: { sum: left + right }, outs: [ "sum" ] };
	},
	delete: function(r: any, properties: any): any {
	}
};

export var mul = {
	check: function() {
		return { defaults: undefined, failures: undefined };
	},
	diff: function() {
		return { replaces: undefined };
	},
	create: function(inputs: any): any {
		let left: number = inputs.left;
		let right: number = inputs.right;
		return { id: "0", resource: { product: left * right }, outs: [ "product" ] };
	},
	update: function(id: string, olds: any, news: any): any {
		let left: number = news.left;
		let right: number = news.right;
		return { id: id, resource: { product: left * right }, outs: [ "product" ] };
	},
	delete: function(r: any, properties: any): any {
	}
};


export var sub = {
	check: function() {
		return { defaults: undefined, failures: undefined };
	},
	diff: function() {
		return { replaces: undefined };
	},
	create: function(inputs: any): any {
		let left: number = inputs.left;
		let right: number = inputs.right;
		return { id: "0", resource: { difference: left - right }, outs: [ "difference" ] };
	},
	update: function(id: string, olds: any, news: any): any {
		let left: number = news.left;
		let right: number = news.right;
		return { id: id, resource: { difference: left - right }, outs: [ "difference" ] };
	},
	delete: function(r: any, properties: any): any {
	}
};

export var div = {
	check: function(inputs: any) {
		return { defaults: undefined, failures: inputs.right == 0 ? [ { property: "right", reason: "divisor must be non-zero" } ] : undefined };
	},
	diff: function() {
		return { replaces: undefined };
	},
	create: function(inputs: any): any {
		let left: number = inputs.left;
		let right: number = inputs.right;
		return { id: "0", resource: { quotient: Math.floor(left / right), remainder: left % right }, outs: [ "quotient", "remainder" ] };
	},
	update: function(id: string, olds: any, news: any): any {
		let left: number = news.left;
		let right: number = news.right;
		return { id: id, resource: { quotient: Math.floor(left / right), remainder: left % right }, outs: [ "quotient", "remainder" ] };
	},
	delete: function(r: any, properties: any): any {
	}
};
