function handleSignal() {
	setTimeout(function() {
		console.errro("error: was not forced to exit");
		process.exit(2);
	}, 3000);
}

process.on('SIGINT', handleSignal);
process.on('SIGBREAK', handleSignal); // ctrl-break on windows
console.log('ready');

setTimeout(function() {
	console.error('error: did not receive signal');
	process.exit(1);
}, 3000);
