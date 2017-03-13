# Coconut Cross-Cloud Targeting

*WARNING: this document is a little out-of-date.  The details are incorrect, but overall idea is still relevant.*

The Coconut metadata and primitives are intentionally cloud-agnostic and have been designed to support [many cloud
targets](clouds.md).  This can be used to build, share, and reuse portable abstractions.

It is easy, however, to introduce a dependency on a particular cloud provider by relying on certain stacks.  For
example, mounting an `aws/ebs/Volume` for a database volume pins it to the AWS IaaS provider; in fact, *any* such
service in the transitive closure of dependencies pins the whole stack to AWS.

On one hand, this is great, because the Coconut abstractions do not get in the way of leveraging the full power of your
cloud provider and its latest innovations.  On the other hand, it inhibits portability.

This document briefly describes how Coconut enables developers to create portable cloud infrastructure abstractions.

## Abstraction

The key to Coconut's ability to cross-target clouds is simple: abstraction.

Coconut stacks are simply ordinary classes.  As a result, they can encapsulate details about how they work.  If a
`Table` stack wants to provision a MongoDB database automatically behind the scenes, there is no need for a consumer to
know.  The properties, API, and so on, can safely hide these details behind a friendly logical abstraction.

More likely, however, such an abstraction will want to leverage a database-as-a-service (DbaaS) product in the target
cloud, like AWS DynamoDB.  But by doing that, you would pin the `Table` abstraction to AWS, which defeats the point.

This is a problem familiar to users of platform abstractions in environments like Java, .NET, and Node.js.  To create
useful low-level primitive abstractions like filesystems, process models, and timer APIs -- among other things -- they
must create an abstraction layer just above the underlying operating system.  Doing so lets 99% of the users of those
abstractions forget about the gory details of targeting Linux vs. macOS vs. Windows.  But those low-level developers
need to worry about `#ifdef`ing their code and bridging the gap.  And in Coconut, we can achieve the same economics.

To do this, the developer providing a low-level abstraction must conditionalize resource usage.  The context object
exposed to the CocoLang programming languages tells the program information about the target environment, including
whether the target is AWS, Google Cloud, Azure, and so on.  As such, the `Table` can pick DynamoDB in AWS, DocumentDB in
Azure, Bigtable in Google Cloud, and perhaps even fall back to MongoDB option as a more complex default elsewhere.

If this is done right, the 99% developer can use an elegant and simple-to-use `Table` abstraction, care-free about its
details, and in the cloud provider of their choice.  And the low-level developer pays some complexity as a result.

## Cross-Cloud Abstractions Out-of-the-Box

To facilitate cross-cloud abstractions, Coconut offers a `coconut/x` package containing a number of them.

### coconut/x

The services offered by `coconut/x` have been conditionalized internally and are guaranteed to run on all clouds,
including locally for development and testing.  The differences between them have been abstracted and unified so that
you can configure them declaratively, using a single logical set of options, and rely on the service internally mapping
to the cloud provider's specific configuration settings.

For example, `coconut/x/fs/Volume` implements the `coconut/Volume` abstract interface, and maps to an AWS Elastic Block
Store (EBS), Azure Data Disk (DD), or GCP Persistent Disk (PD) volume, depending on the IaaS target.  Although the
details for each of these differs, a standard set of options -- like capacity, filesystem type, reclamation policy,
storage class, and so on -- and `coconut/x` handles mapping these standard options to the specific underlying ones.

The goal for the `coconut/x` package is to facilitate a higher-level ecosystem of cloud-agnostic services and libraries.

### Services

This section contains a full list of the `coconut/x` cloud-agnostic services:

* Apps
    - Containers
    - Logging
    - Queueing
    - Pub/sub
    - RPC
    - Service discovery
    - Serverless / API Gateway
* Data
    - Cache
    - Blob Store
    - Key/value Store
    - SQL Database
    - NoSQL Database
    - NoSQL Data Warehouse
    - Secret Store
* Services
    - Email
    - SMS
    - Search
    - Job Scheduling
    - Workflow
    - MapReduce
* Infra Services
    - DNS
    - Load Balancing
    - CDN
    - Container Registry

TODO(joe): hand wavy; flesh this out more.

TODO(joe): need to figure out the distinction between { design-time, runtime } X { dev, ops }.

## Appendix A: Cloud Catalog

Here's an exhaustive list of services offered by "the big three," and an attempt to correlate them.

| COMPUTE                | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| IaaS                   | EC2                            | Virtual Machines               | Compute Engine
| Container Registry     | EC2 Container Registry         |                                | Container Registry
| Container Service      | EC2 Container Service          | Container Service              | Container Engine
| PaaS                   | Elastic Beanstalk              | Cloud Services / Service Fabric| App Engine
| Serverless             | Lambda                         | Functions / WebJobs            | Cloud Functions
| Job Scheduling         |                                | Scheduler / Batch              | Compute Engine Tasks
| Queueing               | Simple Queueing Service (SQS)  | Queue Storage / ServiceBus     |
| Workflow               | Simple Workflow Service (SWS)  | LogicApps                      |

| STORAGE                | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| Blob Storage           | S3                             | Storage                        | Cloud Storage (Standard)
| Low-Cost Archival      | Glacier                        |                                | Cloud Storage (Nearline)
| Mountable Storage      | Elastic File System            | File Storage                   |
| Data Import/Export     | Snowball                       | Import/Export                  |
| On-Prem-to-Cloud       | Storage Gateway                | StorSimple                     |
| Secrets                | Key Management Service (KMS)   | Key Vault                      |

| DATABASE               | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| Database (SQL)         | Relational DB Service (RDS)    | SQL Database                   | Cloud SQL
| Database (NoSQL)       | DynamoDB                       | Document DB / Table Storage    | Cloud Bigtable / Cloud Datastore
| Cache                  | ElastiCache                    | Managed Cache / Redis Cache    |
| Data Warehouse (SQL)   | Redshift                       | SQL Data Warehouse             |
| Data Warehouse (NoSQL) |                                | Data Lake Store                | BigQuery
| Data Migration         | DB Migration Service (DMS)     | SQL DB Migration Wizard        |

| NETWORKING             | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| Load Balancer          | EC2 Load Balancer              | Load Balancer                  | Cloud Load Balancing
| CDN                    | CloudFront                     | Azure CDN                      | Cloud CDN
| Network Mgmt           | Virtual Private Cloud (VPC)    | Virtual Network                | Cloud Virtual Network
| VPN                    | Direct Connect                 | ExpressRoute                   | Cloud Interconnect
| DNS                    | Route 53                       | DNS                            | Cloud DNS

| DEVELOPER TOOLS        | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| Git Hosting            | CodeCommit                     | VSTS                           | Cloud Source Repositories
| C/I|C/D                | CodeDeploy                     | VSTS                           |
| C/I|C/D Workflow       | CodePipeline                   |                                |

| MANAGEMENT TOOLS       | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| E2E Management         | CloudWatch                     |                                | Stackdriver
| Auditing               | CloudTrail                     |                                |
| Monitoring             | -                              | VS AppInsights                 | Monitoring
| Logging                | -                              | Log Analytics                  | Logging
| Error Reporting        | -                              |                                | Error Reporting
| Perf Tracing           | -                              |                                | Trace
| Debugging              | -                              |                                | Debugger
| Mgmt Templates         | CloudFormation                 |                                | Deployment Manager
| Governance             | Config                         |                                |
| Ops                    | OpsWorks (Chef)                | Resource Manager / Automation  |
| Security Templates     | Service Catalog                |                                |
| Service Optimization   | Trusted Advisor                |                                |

| SECURITY & IDENTITY    | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| Identity & Access      | Identity & Access Mgmt         |                                | Cloud IAM
| LDAP / AD              | Directory Service (AD)         | Active Directory               | Cloud Resource Manager
| Security Analysis      | Inspector                      | Security Center                | Cloud Security Scanner
| DoS/Malicious Guards   | WAF                            |                                |
| SSL/TLS Cert Mgmt      | Certificate Manager            |                                |

| BIG DATA / ANALYTICS   | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| MapReduce              | Elastic MapReduce (Hadoop)     | HDInsight                      | Cloud Dataproc
| Data Processing        | Data Pipeline                  | Data Factory                   |
| Search                 | Elasticsearch Service          | Search                         |
| Streams Processing     | Kinesis                        | Stream Analytics               | Cloud Dataflow
| Data Exploration       | -                              | PowerBI / Data Lake Analytics  | Cloud Datalab
| Pub/Sub/Push Notify    | Simple Notif. Service (SNS)    | Notif Hub Topics / Event Hubs  | Cloud Pub/Sub
| Big Science            | -                              |                                | Cloud Genomics

| MACHINE LEARNING       | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| ML Platform            | Machine Learning               | Machine Learning               | Cloud ML Platform
| ML/AI Services         |                                | Cognitive Services             | Vision, Speech, NL, Translate

| INTERNET OF THINGS     | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| IoT                    | IoT                            | IoT Hub                        | IoT

| GAME DEVELOPMENT       | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| Games                  | GameLift

| MOBILE SERVICES        | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| Mobile E2E             | Mobile Hub                     |                                |
| Mobile Auth            | Cognito                        | Multifactor Auth               |
| Mobile Testing         | Device Farm                    | DevTest Labs                   | Cloud Test Lab
| Mobile Analytics       | Mobile Analytics               | HockeyApp                      |

| APP SERVICES           | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| API Gateway            | API Gateway                    | API Management                 | Google Cloud Endpoints
| Remote Desktop         | AppStream                      |                                | RemoteApp
| Search                 | CloudSearch                    | Search                         |
| Media Transcoding      | Elastic Transcoder             | Media Services                 |
| Email                  | Simple Email Service (SES)     |                                |

| ENTERPRISE APPS        | AWS                            | Azure                          | Google
| ---------------------- | ------------------------------ | ------------------------------ | -------------------------------
| Remote Desktop         | WorkSpaces                     |                                |
| Document Sharing       | WorkDocs                       |                                |
| Office (Email/Calendar)| WorkMail                       |                                |

