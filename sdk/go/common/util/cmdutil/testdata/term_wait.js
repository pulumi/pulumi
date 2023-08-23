function handleSignal() {
	console.log('exiting cleanly');
	process.exit(0);
}

process.on('SIGINT', handleSignal);
console.log('ready');

setTimeout(function() {
	console.error('error: did not receive signal');
	process.exit(1);
}, 3000);
