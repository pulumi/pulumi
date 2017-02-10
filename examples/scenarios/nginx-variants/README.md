# examples/nginx

This README demonstrates a few different ways to slice and dice [the Nginx example](
https://github.com/kubernetes/helm/tree/master/docs/examples/nginx), and represent it in Mu, from the official
Kubernetes Helm repository.  We also demonstrate the metadata versions in addition to "infrastructure as code."

The Kubernetes Helm example is fairly sophisticated including several unnecessary steps that are there for illustration
only (like pre- and post-deployment steps).  So as to make the examples easier to follow, we include several examples
without this clutter.  In full admission, this means some examples are not apples-to-apples.

TODO(joe): warning!  This is very much a work in progress.  It's likely many things in here are out-of-date.

## Metadata

In these examples, a single Mu.yaml file contains the description of the entire Nginx stack.

TODO(joe): most of the examples lack mapping a volume to serve content.  That would make them far more realistic.

### Variant 1: Nginx is available in the MuHub; expose directly

In this example, there is an Nginx stack available in the MuHub, and so we just use it by name (`nginx/nginx`).
Additionally, we leverage Mu's built-in ability to project services through a load balanced API gateway automatically:

    services:
        public:
            - nginx/nginx:
                port: 80

### Variant 2: Nginx is available in the MuHub; expose and auto-scale it

In the above examples, we did not scale the frontend out.  In this example, we instance exactly four of them:

    services:
        public:
            - mu/autoscale:
                service:
                    nginx/nginx:
                        port: 80
                replicas: 4

### Variant 3: Nginx is available in the MuHub; expose and configure it

In this example, we leverage configuration to auto-scale (defaulting to 4).  In addition, we map configuration to a
volume inside of Nginx, so that it serves that content:

    services:
        public:
            - mu/autoscale:
                service:
                    nginx/nginx:
                        port: 80
                    volumes:
                        - /usr/share/nginx/html:{{.config.data}}
                replicas: {{default 4 config.replicas}}

### Variant 4: Nginx is available in the MuHub; expose via an API Gateway

In this example, Nginx is available in the MuHub.  However, rather than relying on the built-in API gateway, we create
one manually, mapping the internal port, `8080`, to the desired external one, `80`:

    services:
        private:
            - nginx/nginx:
                ports: 8080
        public:
            - nginx: 80:8080

### Variant 5: No Nginx stack in MuHub; use a container

In the following example, no Nginx stack is available in MuHub.  So, we create one by wrapping a Docker container:

    services:
        public:
            - mu/container:
                image: nginx:stable-alpine
                port: 80


### Variant 6: Full complexity from the Kubernetes Helm example

This example demonstrates the full complexity from the Kubernetes Helm example, such as pre- and post-deployment steps:

    name: nginx
    description: A basic NGINX HTTP server.
    version: 0.1.0
    keywords: [ http, nginx, www, web ]
    home: https://github.com/kubernetes/helm
    source: https://hub.docker.com/_/nginx
    maintainer: joeduffy <joeduffy@acm.org>
    config:
        data:
            index.html: {{default "Hello" .config.index | quote}}
            index.txt: test
    pre:
        {{.name}}-secret:
            type: mu/secret
            password: {{ b64enc "secret" }}
            username: {{ b64enc "user1" }}
    services:
        private:
            - nginx:
                type: mu/autoscale
                service:
                    mu/container:
                        image: {{default "nginx" .config.image}}:{{default "stable-alpine" .config.imageTag}}
                        port: 80
                        volume: /usr/share/nginx/html:{{.config.data}}
                replicas: {{default 1 .config.replicas}}
        public:
            - nginx: {{default 80 .config.httpPort}}:80
    post:
        post-install-job:
            type: mu/container
            image: "alpine:3.3"
            command: [ "/bin/sleep", "{{default 10 .config.sleepyTime}}" ]

### Random Example: Make the port parameter configurable

    name: mystack
    version: 1.0
    description: A great stack.
    parameters:
        - port
    services:
        public:
            - nginx/nginx:
                port: {{args.port}}

### Random Example: add an ELK stack.

    services:
        private:
            - elk
        public:
            - nginx/nginx:
                port: 80

### Random Example: Launch an AWS AMI

    services:
        private:
            - aws/ami:
                image: theglobalsolutions/mongodb

### Random Example: serverless vote 50 app

    name: vote50
    version: 0.0.1
    description: A serverless voting app.
    stacks:
        private:
            - voting:
                type: mu/stack
                private:
                    - votes: mu/table
                    - voteCounts: mu/table
                public:
                    - vote: mu/function
    services:
        public:
            {{range array "AL", "AK", ... "WI", "WY"}}
                - path: /{{.}}/vote
                  service: voting
            {{end}}

### Other Examples

TODO(joe): demonstrate binding to an external SaaS service.

## Infrastructure as Code

This simply shows some of the above examples in code form.  This is entirely hypothetical:

### Variant 1: Nginx is available in the MuHub; expose directly

    var mu = require("mu");
    var nginx = require("@mu/nginx");
    var nginxService = new nginx(80);
    mu.export(nginxService);

### Variant 2: Nginx is available in the MuHub; expose and auto-scale it

    var mu = require("mu");
    var nginx = require("@mu/nginx");
    var nginxService = new nginx(80);
    var nginxAutoScale = new mu.AutoScale(nginxService, mu.config.replicas || 4);
    mu.export(nginxAutoScale);

### Variant 4: Nginx is available in the MuHub; expose via an API Gateway

    var mu = require("mu");
    var nginx = require("@mu/nginx");
    var nginxService = new nginx(8080);
    var gateway = new mu.Service(nginxService, "80:8080");
    mu.export(gateway);

### Variant 5: No Nginx stack in MuHub; use a container

    var mu = require("mu");
    var nginxService = new mu.Container("nginx:stable-alpine", 80);
    mu.export(nginxService);

### Variant 6: Full complexity from the Kubernetes Helm example

    var mu = require("mu");
    var nginxService = new mu.Pod({
        containers: [
        {
            name: "nginx",
            image: `${mu.config.image || "nginx"}:${mu.config.imageTag || "stable-alpine"}`
            ports: [ 80 ],
            volumeMounts: [{
                mountPath: "/usr/share/nginx/html",
                name: "wwwdata-volume",
            }],
        },
        volumes: [{
            name: "wwwdata-volume",
            configMap: ".",
        }],
    });
    var gateway = new mu.Service(nginxService, 80);
    mu.export(gateway);

    mu.registerPostInstall(new mu.Job({
        image: "alpine:3.3",
        command: [ "/bin/sleep", `${mu.config.sleepyTime || 10}`],
    });

