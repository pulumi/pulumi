// Copyright 2021, Pulumi Corporation.  All rights reserved.

for (let i = 0; i < 10; i++) {
    console.log(`Line ${i}`);
    console.error(`Errln ${i+10}`);
}
process.stdout.write("Line 10");
process.stderr.write("Errln 20");
