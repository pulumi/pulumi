import pulumi
import base64
import hashlib

def computeFilebase64sha256(path):
	fileData = open(path).read().encode()
	hashedData = hashlib.sha256(fileData).digest()
	return base64.b64encode(hashedData).decode()

file_content = (lambda path: open(path).read())("testfile.txt")
file_b64 = (lambda path: base64.b64encode(open(path).read().encode()).decode())("testfile.txt")
file_sha = computeFilebase64sha256("testfile.txt")
pulumi.export("fileContent", file_content)
pulumi.export("fileB64", file_b64)
pulumi.export("fileSha", file_sha)
