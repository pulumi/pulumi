var mu = require("mu");

// Define a Service that creates a Stack, wires up Functions to Triggers, and exposes an API:
class VotingService {
    constructor() {
        this.votes = new mu.Table();
        this.voteCounts = new mu.Table();
        this.votes.forEach(vote => {
            // Keep our aggregated counts up-to-date:
            this.voteCounts.updateIncrement(vote.color, vote.count);
        });
    }
    
    vote(info) {
        this.votes.push({ color: info.color, count: 1 });
    }
}

// Create a HTTP endpoint Service that receives votes from an API:
var voteAPI = new mu.HTTPGateway();

for (var state of [ "AL", "AK", ... "WI", "WY" ]) {
    var votingService = new VotingService();
    voteAPI.register(`/${state}`, votingService);
}

