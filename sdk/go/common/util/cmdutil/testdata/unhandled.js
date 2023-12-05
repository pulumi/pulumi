console.log('ready');
setTimeout(function() {
	console.error('error: was not terminated');
	process.exit(1);
}, 3000);
