// Variant #2. Go "all in" on Mu, leveraging higher level abstractions and DSL/metadata-less representation.

"use strict";

var mu = require("mu");

var app = new mu.APIGateway("app");
var votes = new mu.Table("votes");

app.get("/", (req, res) => {
    try {
        var q = await votes.
            groupBy(vote => vote.candidate).
            map(key, group => {
                candidate: key,
                count: group.count()
            });
       // TODO: sort?
       res.send(JSON.stringify(candidates, null, 4));
    }
    catch (err) {
        res.status(500).send(`An error occurred: ${err}`);
    }
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

mu.export(app);

