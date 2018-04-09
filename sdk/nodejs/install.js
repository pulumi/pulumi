// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

const https = require("https");
const fs = require("fs");
const constants = require("constants");
const path = require("path");
const child_process = require("child_process");

const nodeVersion = require("./package.json").pulumi.nodeVersion;
if (!nodeVersion) {
    console.error("failed to ascertain desired node version - is pulumi.nodeVersion set in package.json?");
    process.exit(1);
}

const downloadUrl = constructDownloadUrl(nodeVersion);
const installDir = makeInstallDirectory(nodeVersion);
doDownload(downloadUrl, installDir)
    .then(decompressDownload)
    .then(validateInstall)
    .catch(err => {
        console.error("download failed: ", err);
        process.exit(1);
    })

/**
 * Constructs a URL from which to download a Node binary that we can use to execute Pulumi programs.
 * If we are running on Windows, we must download a patched version of Node that we have built ourselves
 * in order to expose some V8 internals that we link against. On non-Windows, we can use stock Node
 * without any problems and we can download an official Node release.
 * 
 * @param {string} nodeVersion The version of Node we intend to install.
 */
function constructDownloadUrl(nodeVersion) {
    let rootUrl = "https://nodejs.org/download/release/";
    if (process.platform === "win32") {
        console.error("running on Windows, downloading Pulumi Node");
        // TODO(swgillespie) Windows
        process.exit(1);
    }

    console.log("downloading NodeJS:", nodeVersion);
    rootUrl += nodeVersion + '/';

    file = "node-" + nodeVersion + "-" + process.platform + "-" + process.arch + ".tar.gz";
    rootUrl += file;
    return rootUrl;
}

/**
 * Constructs the directory that we will download our Node to.
 * 
 * @param {string} nodeVersion 
 */
function makeInstallDirectory(nodeVersion) {
    function tryMkdir(dir) {
        try {
            fs.mkdirSync(dir);
        } catch(e) {
            if (e.code != "EEXIST") {
                throw e;
            }
        }
    }

    tryMkdir("third_party");
    tryMkdir(path.join("third_party", "node"));
    return path.join("third_party", "node");
}

/**
 * Performs the actual download of a released Node, saving it to a file "node.tar.gz" in the
 * installation directory.
 * 
 * @param {string} downloadUrl The URL from which to download
 * @param {string} installDir The directory to place the downloaded Node tar
 */
function doDownload(downloadUrl, installDir) {
    return new Promise((resolve, reject) => {
        const file = path.join(installDir, "node.tar.gz");
        const stream = fs.createWriteStream(file);
        https.get(downloadUrl, response => {
            response.pipe(stream);
            stream.on('finish', () => {
                stream.close(() => {
                    resolve(file);
                });
            });
        }).on('error', err => {
            fs.unlink(file);
            reject(err);
        });
    });
}

/**
 * Decompresses the Node tar that we jsut downloaded.
 * 
 * @param {string} file The name of the tar we just downloaded.
 */
function decompressDownload(file) {
    return new Promise((resolve, reject) => {
        child_process.exec("tar -C ./third_party/node -xf " + file, (err, stdout, stderr) => {
            if (err) {
                reject(err);
                return;
            }

            fs.unlink(file, err => {
                if (err) {
                    reject(err);
                    return;
                }

                resolve();
            });
        })
    })
}

/**
 * Does a small smoke test of the node that we just downloaded to ensure that 
 * 1) it's not busted and 2) it's the version we expect.
 */
function validateInstall() {
    const nodeBin = path.join("third_party", "node",
        "node-" + nodeVersion + "-" + process.platform + "-" + process.arch,
        "bin",
        "node");

    return new Promise((resolve, reject) => {
        child_process.execFile(nodeBin, ["-p", "process.version"], (error, stdout, stderr) => {
            if (error) {
                reject(error);
                return;
            }

            if (stdout.trim() !== nodeVersion) {
                reject(new Error("downloaded node is not the expected version, expected " + 
                    nodeVersion + ", got " + stdout.trim()));
                return;
            }

            resolve();
        })
    })
}
