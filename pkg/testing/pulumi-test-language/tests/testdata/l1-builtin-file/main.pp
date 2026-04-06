fileContent = readFile("testfile.txt")
fileB64 = filebase64("testfile.txt")
fileSha = filebase64sha256("testfile.txt")

output "fileContent" {
    value = fileContent
}

output "fileB64" {
    value = fileB64
}

output "fileSha" {
    value = fileSha
}
