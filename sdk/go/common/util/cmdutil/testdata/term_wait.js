function handleSignal() {
	console.log('Got SIGINT. Cleaning up...');
	setTimeout(function() {
		console.log('Exiting...');
		process.exit(0);
	}, 1000);
}

process.on('SIGINT', handleSignal);
process.on('SIGTERM', handleSignal);

console.log('Waiting for SIGINT');

setTimeout(function() {
	console.log('No SIGINT received. Exiting...');
	process.exit(1);
}, 3000);
