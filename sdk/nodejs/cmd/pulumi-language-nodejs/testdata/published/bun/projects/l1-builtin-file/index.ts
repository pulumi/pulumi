import * as pulumi from "@pulumi/pulumi";
import * as crypto from "crypto";
import * as fs from "fs";

function computeFilebase64sha256(path: string): string {
	const fileData = Buffer.from(fs.readFileSync(path, 'binary'))
	return crypto.createHash('sha256').update(fileData).digest('base64')
}

export default async () => {
    const fileContent = fs.readFileSync("testfile.txt", "utf8");
    const fileB64 = fs.readFileSync("testfile.txt", { encoding: "base64" });
    const fileSha = computeFilebase64sha256("testfile.txt");
    return {
        fileContent: fileContent,
        fileB64: fileB64,
        fileSha: fileSha,
    };
}
