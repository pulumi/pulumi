// Standard Express and MongoDB app.

"use strict";

var express = require("express");
var bodyParser = require("body-parser");
var mongodb = require("mongodb");

// First read in the arguments.
if (process.argv.length < 3) {
    console.log("Missing required database argument");
    process.exit(-1);
}

var db = process.argv[2];

// Connect to the database and then fire up an Express app.
mongodb.MongoClient.connect(`mongodb://${db}`, (err, conn) => {
    if (err) {
        console.log(`Problem connecting to database ${db}:`);
        console.log(err);
        process.exit(-1);
    }

    var votes = conn.collection("votes");

    var app = express();
    app.use(bodyParser.json());

    app.get("/", (req, res) => {
        votes.aggregate([ { "$group" : { _id: "$candidate", count: { $sum: 1 } } } ]).toArray(
            (err, candidates) => {
                // TODO: sort?
                if (err) {
                    res.status(500).send(`An error occurred: ${err}`);
                } else {
                    res.send(JSON.stringify(candidates, null, 4));
                }
            }
        );
    });

    app.post("/vote", (req, res) => {
        var vote = req.body;
        if (!vote || !vote.candidate) {
            res.status(500).send("Missing candidate in POST body");
        } else {
            votes.insertOne(
                { candidate: vote.candidate, time: Date.now() },
                (err) => {
                    if (err) {
                        res.status(500).send(`An error occurred: ${err}`);
                    } else {
                        res.status(200);
                    }
                }
            );
        }
    });

    app.listen(8080, () => {
        console.log("App listening on port 8080");
    });
});

