export var add = {
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

export var mul = {
	diff: function() {
		return { replaces: undefined };
	},
	create: function(inputs: any): any {
		let left: number = inputs.left;
		let right: number = inputs.right;
		return { resource: { product: left * right }, outs: [ "product" ] };
	},
	update: function(r: any, olds: any, news: any): any {
		let left: number = news.left;
		let right: number = news.right;
		r.product = left * right;
		return { outs: ["product"] };
	},
	delete: function(r: any, properties: any): any {
	}
};


export var sub = {
	diff: function() {
		return { replaces: undefined };
	},
	create: function(inputs: any): any {
		let left: number = inputs.left;
		let right: number = inputs.right;
		return { resource: { difference: left - right }, outs: [ "difference" ] };
	},
	update: function(r: any, olds: any, news: any): any {
		let left: number = news.left;
		let right: number = news.right;
		r.difference = left - right;
		return { outs: ["difference"] };
	},
	delete: function(r: any, properties: any): any {
	}
};

export var div = {
	diff: function() {
		return { replaces: undefined };
	},
	create: function(inputs: any): any {
		let left: number = inputs.left;
		let right: number = inputs.right;
		return { resource: { quotient: Math.floor(left / right), remainder: left % right }, outs: [ "quotient", "remainder" ] };
	},
	update: function(r: any, olds: any, news: any): any {
		let left: number = news.left;
		let right: number = news.right;
		r.quotient = Math.floor(left / right);
		r.remainder = left % right;
		return { outs: ["quotient", "remainder"] };
	},
	delete: function(r: any, properties: any): any {
	}
};
