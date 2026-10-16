function handleSignal() {
	console.log('exiting cleanly');
	process.exit(0);
}

process.on('SIGINT', handleSignal);
process.on('SIGBREAK', handleSignal); // ctrl-break on windows
console.log('ready');

setTimeout(function() {
	console.error('error: did not receive signal');
	process.exit(1);
}, 3000);
