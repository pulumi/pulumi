import * as mu from "mu";

let queue = new mu.x.Queue("sites_to_process");
let documents = new mu.x.Table("documents", "id");

let frontEnd = new mu.x.Website("crawler_front_end", (app) => {
    app.get("/", (_, res) => res.render("index.html"));
    app.post("/queue", (req, res) => res.json(queue.push(req.json.url)));
    app.get("/documents/stats", (_, res) => res.json(documents.count()));
});

console.log("Launched crawler front end @ " + frontEnd.url);

queue.forEach((url) => {
    console.log("Handling: " + url);
    let found = documents.get({ id: url });
    if (found) {
        console.log("Already found " + url);
        return false;
    }
    console.log("Getting url: " + url);
    let res = fetch(url);
    if (!res) {
        console.log("Failed to GET " + url);
        return false;
    }
    let html = res.body;
    let contentType = res.headers["content-type"];
    documents.insert({ id: url, contentType: contentType, crawlDate: Date.now(), crawlInProgress: true });
    if (!(contentType && contentType.indexOf("text/html") > -1)) {
        console.log("Skipping non-HTML");
        return false;
    };
    for (let href of Set($("a", "href", html))) {
        if (href && (href.indexOf("visualstudio.com") > -1)) {
            let found = documents.get([id: href]);
            if (!found) queue.push(href);
        }
    };
    documents.update({ id: url }, { crawlInProgress: false });
    console.log("Succeed url: " + url);
});

let job = new mu.x.Job(() => {
    documents.delete({ id: "http://visualstudio.com" });
    queue.push("http://visualstudio.com");
});

