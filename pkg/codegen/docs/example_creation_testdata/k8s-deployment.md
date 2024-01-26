### TypeScript

```typescript
import * as pulumi from "@pulumi/pulumi";
import * as kubernetes from "@pulumi/kubernetes";

const deployment = new kubernetes.apps.v1.Deployment("deployment", {
  apiVersion: "string",
  kind: "string",
  metadata: {
    annotations: {
      "string": "string"
    },
    clusterName: "string",
    creationTimestamp: "string",
    deletionGracePeriodSeconds: 0,
    deletionTimestamp: "string",
    finalizers: ["string"],
    generateName: "string",
    generation: 0,
    labels: {
      "string": "string"
    },
    managedFields: [{
      apiVersion: "string",
      fieldsType: "string",
      fieldsV1: ,
      manager: "string",
      operation: "string",
      subresource: "string",
      time: "string",
    }],
    name: "string",
    namespace: "string",
    ownerReferences: [{
      apiVersion: "string",
      blockOwnerDeletion: true|false,
      controller: true|false,
      kind: "string",
      name: "string",
      uid: "string",
    }],
    resourceVersion: "string",
    selfLink: "string",
    uid: "string",
  },
  spec: {
    minReadySeconds: 0,
    paused: true|false,
    progressDeadlineSeconds: 0,
    replicas: 0,
    revisionHistoryLimit: 0,
    selector: {
      matchExpressions: [{
        key: "string",
        operator: "string",
        values: ["string"],
      }],
      matchLabels: {
        "string": "string"
      },
    },
    strategy: {
      rollingUpdate: {
        maxSurge: 0,
        maxUnavailable: 0,
      },
      type: "string",
    },
    template: {
      metadata: {
        annotations: {
          "string": "string"
        },
        clusterName: "string",
        creationTimestamp: "string",
        deletionGracePeriodSeconds: 0,
        deletionTimestamp: "string",
        finalizers: ["string"],
        generateName: "string",
        generation: 0,
        labels: {
          "string": "string"
        },
        managedFields: [{
          apiVersion: "string",
          fieldsType: "string",
          fieldsV1: ,
          manager: "string",
          operation: "string",
          subresource: "string",
          time: "string",
        }],
        name: "string",
        namespace: "string",
        ownerReferences: [{
          apiVersion: "string",
          blockOwnerDeletion: true|false,
          controller: true|false,
          kind: "string",
          name: "string",
          uid: "string",
        }],
        resourceVersion: "string",
        selfLink: "string",
        uid: "string",
      },
      spec: {
        activeDeadlineSeconds: 0,
        affinity: {
          nodeAffinity: {
            preferredDuringSchedulingIgnoredDuringExecution: [{
              preference: {
                matchExpressions: [{
                  key: "string",
                  operator: "string",
                  values: ["string"],
                }],
                matchFields: [{
                  key: "string",
                  operator: "string",
                  values: ["string"],
                }],
              },
              weight: 0,
            }],
            requiredDuringSchedulingIgnoredDuringExecution: {
              nodeSelectorTerms: [{
                matchExpressions: [{
                  key: "string",
                  operator: "string",
                  values: ["string"],
                }],
                matchFields: [{
                  key: "string",
                  operator: "string",
                  values: ["string"],
                }],
              }],
            },
          },
          podAffinity: {
            preferredDuringSchedulingIgnoredDuringExecution: [{
              podAffinityTerm: {
                labelSelector: {
                  matchExpressions: [{
                    key: "string",
                    operator: "string",
                    values: ["string"],
                  }],
                  matchLabels: {
                    "string": "string"
                  },
                },
                namespaceSelector: {
                  matchExpressions: [{
                    key: "string",
                    operator: "string",
                    values: ["string"],
                  }],
                  matchLabels: {
                    "string": "string"
                  },
                },
                namespaces: ["string"],
                topologyKey: "string",
              },
              weight: 0,
            }],
            requiredDuringSchedulingIgnoredDuringExecution: [{
              labelSelector: {
                matchExpressions: [{
                  key: "string",
                  operator: "string",
                  values: ["string"],
                }],
                matchLabels: {
                  "string": "string"
                },
              },
              namespaceSelector: {
                matchExpressions: [{
                  key: "string",
                  operator: "string",
                  values: ["string"],
                }],
                matchLabels: {
                  "string": "string"
                },
              },
              namespaces: ["string"],
              topologyKey: "string",
            }],
          },
          podAntiAffinity: {
            preferredDuringSchedulingIgnoredDuringExecution: [{
              podAffinityTerm: {
                labelSelector: {
                  matchExpressions: [{
                    key: "string",
                    operator: "string",
                    values: ["string"],
                  }],
                  matchLabels: {
                    "string": "string"
                  },
                },
                namespaceSelector: {
                  matchExpressions: [{
                    key: "string",
                    operator: "string",
                    values: ["string"],
                  }],
                  matchLabels: {
                    "string": "string"
                  },
                },
                namespaces: ["string"],
                topologyKey: "string",
              },
              weight: 0,
            }],
            requiredDuringSchedulingIgnoredDuringExecution: [{
              labelSelector: {
                matchExpressions: [{
                  key: "string",
                  operator: "string",
                  values: ["string"],
                }],
                matchLabels: {
                  "string": "string"
                },
              },
              namespaceSelector: {
                matchExpressions: [{
                  key: "string",
                  operator: "string",
                  values: ["string"],
                }],
                matchLabels: {
                  "string": "string"
                },
              },
              namespaces: ["string"],
              topologyKey: "string",
            }],
          },
        },
        automountServiceAccountToken: true|false,
        containers: [{
          args: ["string"],
          command: ["string"],
          env: [{
            name: "string",
            value: "string",
            valueFrom: {
              configMapKeyRef: {
                key: "string",
                name: "string",
                optional: true|false,
              },
              fieldRef: {
                apiVersion: "string",
                fieldPath: "string",
              },
              resourceFieldRef: {
                containerName: "string",
                divisor: "string",
                resource: "string",
              },
              secretKeyRef: {
                key: "string",
                name: "string",
                optional: true|false,
              },
            },
          }],
          envFrom: [{
            configMapRef: {
              name: "string",
              optional: true|false,
            },
            prefix: "string",
            secretRef: {
              name: "string",
              optional: true|false,
            },
          }],
          image: "string",
          imagePullPolicy: "string",
          lifecycle: {
            postStart: {
              exec: {
                command: ["string"],
              },
              httpGet: {
                host: "string",
                httpHeaders: [{
                  name: "string",
                  value: "string",
                }],
                path: "string",
                port: 0,
                scheme: "string",
              },
              tcpSocket: {
                host: "string",
                port: 0,
              },
            },
            preStop: {
              exec: {
                command: ["string"],
              },
              httpGet: {
                host: "string",
                httpHeaders: [{
                  name: "string",
                  value: "string",
                }],
                path: "string",
                port: 0,
                scheme: "string",
              },
              tcpSocket: {
                host: "string",
                port: 0,
              },
            },
          },
          livenessProbe: {
            exec: {
              command: ["string"],
            },
            failureThreshold: 0,
            httpGet: {
              host: "string",
              httpHeaders: [{
                name: "string",
                value: "string",
              }],
              path: "string",
              port: 0,
              scheme: "string",
            },
            initialDelaySeconds: 0,
            periodSeconds: 0,
            successThreshold: 0,
            tcpSocket: {
              host: "string",
              port: 0,
            },
            terminationGracePeriodSeconds: 0,
            timeoutSeconds: 0,
          },
          name: "string",
          ports: [{
            containerPort: 0,
            hostIP: "string",
            hostPort: 0,
            name: "string",
            protocol: "string",
          }],
          readinessProbe: {
            exec: {
              command: ["string"],
            },
            failureThreshold: 0,
            httpGet: {
              host: "string",
              httpHeaders: [{
                name: "string",
                value: "string",
              }],
              path: "string",
              port: 0,
              scheme: "string",
            },
            initialDelaySeconds: 0,
            periodSeconds: 0,
            successThreshold: 0,
            tcpSocket: {
              host: "string",
              port: 0,
            },
            terminationGracePeriodSeconds: 0,
            timeoutSeconds: 0,
          },
          resources: {
            limits: {
              "string": "string"
            },
            requests: {
              "string": "string"
            },
          },
          securityContext: {
            allowPrivilegeEscalation: true|false,
            capabilities: {
              add: ["string"],
              drop: ["string"],
            },
            privileged: true|false,
            procMount: "string",
            readOnlyRootFilesystem: true|false,
            runAsGroup: 0,
            runAsNonRoot: true|false,
            runAsUser: 0,
            seLinuxOptions: {
              level: "string",
              role: "string",
              type: "string",
              user: "string",
            },
            seccompProfile: {
              localhostProfile: "string",
              type: "string",
            },
            windowsOptions: {
              gmsaCredentialSpec: "string",
              gmsaCredentialSpecName: "string",
              hostProcess: true|false,
              runAsUserName: "string",
            },
          },
          startupProbe: {
            exec: {
              command: ["string"],
            },
            failureThreshold: 0,
            httpGet: {
              host: "string",
              httpHeaders: [{
                name: "string",
                value: "string",
              }],
              path: "string",
              port: 0,
              scheme: "string",
            },
            initialDelaySeconds: 0,
            periodSeconds: 0,
            successThreshold: 0,
            tcpSocket: {
              host: "string",
              port: 0,
            },
            terminationGracePeriodSeconds: 0,
            timeoutSeconds: 0,
          },
          stdin: true|false,
          stdinOnce: true|false,
          terminationMessagePath: "string",
          terminationMessagePolicy: "string",
          tty: true|false,
          volumeDevices: [{
            devicePath: "string",
            name: "string",
          }],
          volumeMounts: [{
            mountPath: "string",
            mountPropagation: "string",
            name: "string",
            readOnly: true|false,
            subPath: "string",
            subPathExpr: "string",
          }],
          workingDir: "string",
        }],
        dnsConfig: {
          nameservers: ["string"],
          options: [{
            name: "string",
            value: "string",
          }],
          searches: ["string"],
        },
        dnsPolicy: "string",
        enableServiceLinks: true|false,
        ephemeralContainers: [{
          args: ["string"],
          command: ["string"],
          env: [{
            name: "string",
            value: "string",
            valueFrom: {
              configMapKeyRef: {
                key: "string",
                name: "string",
                optional: true|false,
              },
              fieldRef: {
                apiVersion: "string",
                fieldPath: "string",
              },
              resourceFieldRef: {
                containerName: "string",
                divisor: "string",
                resource: "string",
              },
              secretKeyRef: {
                key: "string",
                name: "string",
                optional: true|false,
              },
            },
          }],
          envFrom: [{
            configMapRef: {
              name: "string",
              optional: true|false,
            },
            prefix: "string",
            secretRef: {
              name: "string",
              optional: true|false,
            },
          }],
          image: "string",
          imagePullPolicy: "string",
          lifecycle: {
            postStart: {
              exec: {
                command: ["string"],
              },
              httpGet: {
                host: "string",
                httpHeaders: [{
                  name: "string",
                  value: "string",
                }],
                path: "string",
                port: 0,
                scheme: "string",
              },
              tcpSocket: {
                host: "string",
                port: 0,
              },
            },
            preStop: {
              exec: {
                command: ["string"],
              },
              httpGet: {
                host: "string",
                httpHeaders: [{
                  name: "string",
                  value: "string",
                }],
                path: "string",
                port: 0,
                scheme: "string",
              },
              tcpSocket: {
                host: "string",
                port: 0,
              },
            },
          },
          livenessProbe: {
            exec: {
              command: ["string"],
            },
            failureThreshold: 0,
            httpGet: {
              host: "string",
              httpHeaders: [{
                name: "string",
                value: "string",
              }],
              path: "string",
              port: 0,
              scheme: "string",
            },
            initialDelaySeconds: 0,
            periodSeconds: 0,
            successThreshold: 0,
            tcpSocket: {
              host: "string",
              port: 0,
            },
            terminationGracePeriodSeconds: 0,
            timeoutSeconds: 0,
          },
          name: "string",
          ports: [{
            containerPort: 0,
            hostIP: "string",
            hostPort: 0,
            name: "string",
            protocol: "string",
          }],
          readinessProbe: {
            exec: {
              command: ["string"],
            },
            failureThreshold: 0,
            httpGet: {
              host: "string",
              httpHeaders: [{
                name: "string",
                value: "string",
              }],
              path: "string",
              port: 0,
              scheme: "string",
            },
            initialDelaySeconds: 0,
            periodSeconds: 0,
            successThreshold: 0,
            tcpSocket: {
              host: "string",
              port: 0,
            },
            terminationGracePeriodSeconds: 0,
            timeoutSeconds: 0,
          },
          resources: {
            limits: {
              "string": "string"
            },
            requests: {
              "string": "string"
            },
          },
          securityContext: {
            allowPrivilegeEscalation: true|false,
            capabilities: {
              add: ["string"],
              drop: ["string"],
            },
            privileged: true|false,
            procMount: "string",
            readOnlyRootFilesystem: true|false,
            runAsGroup: 0,
            runAsNonRoot: true|false,
            runAsUser: 0,
            seLinuxOptions: {
              level: "string",
              role: "string",
              type: "string",
              user: "string",
            },
            seccompProfile: {
              localhostProfile: "string",
              type: "string",
            },
            windowsOptions: {
              gmsaCredentialSpec: "string",
              gmsaCredentialSpecName: "string",
              hostProcess: true|false,
              runAsUserName: "string",
            },
          },
          startupProbe: {
            exec: {
              command: ["string"],
            },
            failureThreshold: 0,
            httpGet: {
              host: "string",
              httpHeaders: [{
                name: "string",
                value: "string",
              }],
              path: "string",
              port: 0,
              scheme: "string",
            },
            initialDelaySeconds: 0,
            periodSeconds: 0,
            successThreshold: 0,
            tcpSocket: {
              host: "string",
              port: 0,
            },
            terminationGracePeriodSeconds: 0,
            timeoutSeconds: 0,
          },
          stdin: true|false,
          stdinOnce: true|false,
          targetContainerName: "string",
          terminationMessagePath: "string",
          terminationMessagePolicy: "string",
          tty: true|false,
          volumeDevices: [{
            devicePath: "string",
            name: "string",
          }],
          volumeMounts: [{
            mountPath: "string",
            mountPropagation: "string",
            name: "string",
            readOnly: true|false,
            subPath: "string",
            subPathExpr: "string",
          }],
          workingDir: "string",
        }],
        hostAliases: [{
          hostnames: ["string"],
          ip: "string",
        }],
        hostIPC: true|false,
        hostNetwork: true|false,
        hostPID: true|false,
        hostname: "string",
        imagePullSecrets: [{
          name: "string",
        }],
        initContainers: [{
          args: ["string"],
          command: ["string"],
          env: [{
            name: "string",
            value: "string",
            valueFrom: {
              configMapKeyRef: {
                key: "string",
                name: "string",
                optional: true|false,
              },
              fieldRef: {
                apiVersion: "string",
                fieldPath: "string",
              },
              resourceFieldRef: {
                containerName: "string",
                divisor: "string",
                resource: "string",
              },
              secretKeyRef: {
                key: "string",
                name: "string",
                optional: true|false,
              },
            },
          }],
          envFrom: [{
            configMapRef: {
              name: "string",
              optional: true|false,
            },
            prefix: "string",
            secretRef: {
              name: "string",
              optional: true|false,
            },
          }],
          image: "string",
          imagePullPolicy: "string",
          lifecycle: {
            postStart: {
              exec: {
                command: ["string"],
              },
              httpGet: {
                host: "string",
                httpHeaders: [{
                  name: "string",
                  value: "string",
                }],
                path: "string",
                port: 0,
                scheme: "string",
              },
              tcpSocket: {
                host: "string",
                port: 0,
              },
            },
            preStop: {
              exec: {
                command: ["string"],
              },
              httpGet: {
                host: "string",
                httpHeaders: [{
                  name: "string",
                  value: "string",
                }],
                path: "string",
                port: 0,
                scheme: "string",
              },
              tcpSocket: {
                host: "string",
                port: 0,
              },
            },
          },
          livenessProbe: {
            exec: {
              command: ["string"],
            },
            failureThreshold: 0,
            httpGet: {
              host: "string",
              httpHeaders: [{
                name: "string",
                value: "string",
              }],
              path: "string",
              port: 0,
              scheme: "string",
            },
            initialDelaySeconds: 0,
            periodSeconds: 0,
            successThreshold: 0,
            tcpSocket: {
              host: "string",
              port: 0,
            },
            terminationGracePeriodSeconds: 0,
            timeoutSeconds: 0,
          },
          name: "string",
          ports: [{
            containerPort: 0,
            hostIP: "string",
            hostPort: 0,
            name: "string",
            protocol: "string",
          }],
          readinessProbe: {
            exec: {
              command: ["string"],
            },
            failureThreshold: 0,
            httpGet: {
              host: "string",
              httpHeaders: [{
                name: "string",
                value: "string",
              }],
              path: "string",
              port: 0,
              scheme: "string",
            },
            initialDelaySeconds: 0,
            periodSeconds: 0,
            successThreshold: 0,
            tcpSocket: {
              host: "string",
              port: 0,
            },
            terminationGracePeriodSeconds: 0,
            timeoutSeconds: 0,
          },
          resources: {
            limits: {
              "string": "string"
            },
            requests: {
              "string": "string"
            },
          },
          securityContext: {
            allowPrivilegeEscalation: true|false,
            capabilities: {
              add: ["string"],
              drop: ["string"],
            },
            privileged: true|false,
            procMount: "string",
            readOnlyRootFilesystem: true|false,
            runAsGroup: 0,
            runAsNonRoot: true|false,
            runAsUser: 0,
            seLinuxOptions: {
              level: "string",
              role: "string",
              type: "string",
              user: "string",
            },
            seccompProfile: {
              localhostProfile: "string",
              type: "string",
            },
            windowsOptions: {
              gmsaCredentialSpec: "string",
              gmsaCredentialSpecName: "string",
              hostProcess: true|false,
              runAsUserName: "string",
            },
          },
          startupProbe: {
            exec: {
              command: ["string"],
            },
            failureThreshold: 0,
            httpGet: {
              host: "string",
              httpHeaders: [{
                name: "string",
                value: "string",
              }],
              path: "string",
              port: 0,
              scheme: "string",
            },
            initialDelaySeconds: 0,
            periodSeconds: 0,
            successThreshold: 0,
            tcpSocket: {
              host: "string",
              port: 0,
            },
            terminationGracePeriodSeconds: 0,
            timeoutSeconds: 0,
          },
          stdin: true|false,
          stdinOnce: true|false,
          terminationMessagePath: "string",
          terminationMessagePolicy: "string",
          tty: true|false,
          volumeDevices: [{
            devicePath: "string",
            name: "string",
          }],
          volumeMounts: [{
            mountPath: "string",
            mountPropagation: "string",
            name: "string",
            readOnly: true|false,
            subPath: "string",
            subPathExpr: "string",
          }],
          workingDir: "string",
        }],
        nodeName: "string",
        nodeSelector: {
          "string": "string"
        },
        overhead: {
          "string": "string"
        },
        preemptionPolicy: "string",
        priority: 0,
        priorityClassName: "string",
        readinessGates: [{
          conditionType: "string",
        }],
        restartPolicy: "string",
        runtimeClassName: "string",
        schedulerName: "string",
        securityContext: {
          fsGroup: 0,
          fsGroupChangePolicy: "string",
          runAsGroup: 0,
          runAsNonRoot: true|false,
          runAsUser: 0,
          seLinuxOptions: {
            level: "string",
            role: "string",
            type: "string",
            user: "string",
          },
          seccompProfile: {
            localhostProfile: "string",
            type: "string",
          },
          supplementalGroups: [0],
          sysctls: [{
            name: "string",
            value: "string",
          }],
          windowsOptions: {
            gmsaCredentialSpec: "string",
            gmsaCredentialSpecName: "string",
            hostProcess: true|false,
            runAsUserName: "string",
          },
        },
        serviceAccount: "string",
        serviceAccountName: "string",
        setHostnameAsFQDN: true|false,
        shareProcessNamespace: true|false,
        subdomain: "string",
        terminationGracePeriodSeconds: 0,
        tolerations: [{
          effect: "string",
          key: "string",
          operator: "string",
          tolerationSeconds: 0,
          value: "string",
        }],
        topologySpreadConstraints: [{
          labelSelector: {
            matchExpressions: [{
              key: "string",
              operator: "string",
              values: ["string"],
            }],
            matchLabels: {
              "string": "string"
            },
          },
          maxSkew: 0,
          topologyKey: "string",
          whenUnsatisfiable: "string",
        }],
        volumes: [{
          awsElasticBlockStore: {
            fsType: "string",
            partition: 0,
            readOnly: true|false,
            volumeID: "string",
          },
          azureDisk: {
            cachingMode: "string",
            diskName: "string",
            diskURI: "string",
            fsType: "string",
            kind: "string",
            readOnly: true|false,
          },
          azureFile: {
            readOnly: true|false,
            secretName: "string",
            shareName: "string",
          },
          cephfs: {
            monitors: ["string"],
            path: "string",
            readOnly: true|false,
            secretFile: "string",
            secretRef: {
              name: "string",
            },
            user: "string",
          },
          cinder: {
            fsType: "string",
            readOnly: true|false,
            secretRef: {
              name: "string",
            },
            volumeID: "string",
          },
          configMap: {
            defaultMode: 0,
            items: [{
              key: "string",
              mode: 0,
              path: "string",
            }],
            name: "string",
            optional: true|false,
          },
          csi: {
            driver: "string",
            fsType: "string",
            nodePublishSecretRef: {
              name: "string",
            },
            readOnly: true|false,
            volumeAttributes: {
              "string": "string"
            },
          },
          downwardAPI: {
            defaultMode: 0,
            items: [{
              fieldRef: {
                apiVersion: "string",
                fieldPath: "string",
              },
              mode: 0,
              path: "string",
              resourceFieldRef: {
                containerName: "string",
                divisor: "string",
                resource: "string",
              },
            }],
          },
          emptyDir: {
            medium: "string",
            sizeLimit: "string",
          },
          ephemeral: {
            readOnly: true|false,
            volumeClaimTemplate: {
              metadata: {
                annotations: {
                  "string": "string"
                },
                clusterName: "string",
                creationTimestamp: "string",
                deletionGracePeriodSeconds: 0,
                deletionTimestamp: "string",
                finalizers: ["string"],
                generateName: "string",
                generation: 0,
                labels: {
                  "string": "string"
                },
                managedFields: [{
                  apiVersion: "string",
                  fieldsType: "string",
                  fieldsV1: ,
                  manager: "string",
                  operation: "string",
                  subresource: "string",
                  time: "string",
                }],
                name: "string",
                namespace: "string",
                ownerReferences: [{
                  apiVersion: "string",
                  blockOwnerDeletion: true|false,
                  controller: true|false,
                  kind: "string",
                  name: "string",
                  uid: "string",
                }],
                resourceVersion: "string",
                selfLink: "string",
                uid: "string",
              },
              spec: {
                accessModes: ["string"],
                dataSource: {
                  apiGroup: "string",
                  kind: "string",
                  name: "string",
                },
                dataSourceRef: {
                  apiGroup: "string",
                  kind: "string",
                  name: "string",
                },
                resources: {
                  limits: {
                    "string": "string"
                  },
                  requests: {
                    "string": "string"
                  },
                },
                selector: {
                  matchExpressions: [{
                    key: "string",
                    operator: "string",
                    values: ["string"],
                  }],
                  matchLabels: {
                    "string": "string"
                  },
                },
                storageClassName: "string",
                volumeMode: "string",
                volumeName: "string",
              },
            },
          },
          fc: {
            fsType: "string",
            lun: 0,
            readOnly: true|false,
            targetWWNs: ["string"],
            wwids: ["string"],
          },
          flexVolume: {
            driver: "string",
            fsType: "string",
            options: {
              "string": "string"
            },
            readOnly: true|false,
            secretRef: {
              name: "string",
            },
          },
          flocker: {
            datasetName: "string",
            datasetUUID: "string",
          },
          gcePersistentDisk: {
            fsType: "string",
            partition: 0,
            pdName: "string",
            readOnly: true|false,
          },
          gitRepo: {
            directory: "string",
            repository: "string",
            revision: "string",
          },
          glusterfs: {
            endpoints: "string",
            path: "string",
            readOnly: true|false,
          },
          hostPath: {
            path: "string",
            type: "string",
          },
          iscsi: {
            chapAuthDiscovery: true|false,
            chapAuthSession: true|false,
            fsType: "string",
            initiatorName: "string",
            iqn: "string",
            iscsiInterface: "string",
            lun: 0,
            portals: ["string"],
            readOnly: true|false,
            secretRef: {
              name: "string",
            },
            targetPortal: "string",
          },
          name: "string",
          nfs: {
            path: "string",
            readOnly: true|false,
            server: "string",
          },
          persistentVolumeClaim: {
            claimName: "string",
            readOnly: true|false,
          },
          photonPersistentDisk: {
            fsType: "string",
            pdID: "string",
          },
          portworxVolume: {
            fsType: "string",
            readOnly: true|false,
            volumeID: "string",
          },
          projected: {
            defaultMode: 0,
            sources: [{
              configMap: {
                items: [{
                  key: "string",
                  mode: 0,
                  path: "string",
                }],
                name: "string",
                optional: true|false,
              },
              downwardAPI: {
                items: [{
                  fieldRef: {
                    apiVersion: "string",
                    fieldPath: "string",
                  },
                  mode: 0,
                  path: "string",
                  resourceFieldRef: {
                    containerName: "string",
                    divisor: "string",
                    resource: "string",
                  },
                }],
              },
              secret: {
                items: [{
                  key: "string",
                  mode: 0,
                  path: "string",
                }],
                name: "string",
                optional: true|false,
              },
              serviceAccountToken: {
                audience: "string",
                expirationSeconds: 0,
                path: "string",
              },
            }],
          },
          quobyte: {
            group: "string",
            readOnly: true|false,
            registry: "string",
            tenant: "string",
            user: "string",
            volume: "string",
          },
          rbd: {
            fsType: "string",
            image: "string",
            keyring: "string",
            monitors: ["string"],
            pool: "string",
            readOnly: true|false,
            secretRef: {
              name: "string",
            },
            user: "string",
          },
          scaleIO: {
            fsType: "string",
            gateway: "string",
            protectionDomain: "string",
            readOnly: true|false,
            secretRef: {
              name: "string",
            },
            sslEnabled: true|false,
            storageMode: "string",
            storagePool: "string",
            system: "string",
            volumeName: "string",
          },
          secret: {
            defaultMode: 0,
            items: [{
              key: "string",
              mode: 0,
              path: "string",
            }],
            optional: true|false,
            secretName: "string",
          },
          storageos: {
            fsType: "string",
            readOnly: true|false,
            secretRef: {
              name: "string",
            },
            volumeName: "string",
            volumeNamespace: "string",
          },
          vsphereVolume: {
            fsType: "string",
            storagePolicyID: "string",
            storagePolicyName: "string",
            volumePath: "string",
          },
        }],
      },
    },
  },
});
```

### Python

```python
import pulumi
import pulumi_kubernetes as kubernetes

deployment = kubernetes.apps.v1.Deployment("deployment",
  api_version="string",
  kind="string",
  metadata=kubernetes.meta.v1.ObjectMetaArgs(
    annotations={
      'string': "string"
    },
    cluster_name="string",
    creation_timestamp="string",
    deletion_grace_period_seconds=0,
    deletion_timestamp="string",
    finalizers=[
      "string",
    ],
    generate_name="string",
    generation=0,
    labels={
      'string': "string"
    },
    managed_fields=[
      kubernetes.meta.v1.ManagedFieldsEntryArgs(
        api_version="string",
        fields_type="string",
        fields_v1=,
        manager="string",
        operation="string",
        subresource="string",
        time="string",
      ),
    ],
    name="string",
    namespace="string",
    owner_references=[
      kubernetes.meta.v1.OwnerReferenceArgs(
        api_version="string",
        block_owner_deletion=True|False,
        controller=True|False,
        kind="string",
        name="string",
        uid="string",
      ),
    ],
    resource_version="string",
    self_link="string",
    uid="string",
  ),
  spec=kubernetes.apps.v1.DeploymentSpecArgs(
    min_ready_seconds=0,
    paused=True|False,
    progress_deadline_seconds=0,
    replicas=0,
    revision_history_limit=0,
    selector=kubernetes.meta.v1.LabelSelectorArgs(
      match_expressions=[
        kubernetes.meta.v1.LabelSelectorRequirementArgs(
          key="string",
          operator="string",
          values=[
            "string",
          ],
        ),
      ],
      match_labels={
        'string': "string"
      },
    ),
    strategy=kubernetes.apps.v1.DeploymentStrategyArgs(
      rolling_update=kubernetes.apps.v1.RollingUpdateDeploymentArgs(
        max_surge=0,
        max_unavailable=0,
      ),
      type="string",
    ),
    template=kubernetes.core.v1.PodTemplateSpecArgs(
      metadata=kubernetes.meta.v1.ObjectMetaArgs(
        annotations={
          'string': "string"
        },
        cluster_name="string",
        creation_timestamp="string",
        deletion_grace_period_seconds=0,
        deletion_timestamp="string",
        finalizers=[
          "string",
        ],
        generate_name="string",
        generation=0,
        labels={
          'string': "string"
        },
        managed_fields=[
          kubernetes.meta.v1.ManagedFieldsEntryArgs(
            api_version="string",
            fields_type="string",
            fields_v1=,
            manager="string",
            operation="string",
            subresource="string",
            time="string",
          ),
        ],
        name="string",
        namespace="string",
        owner_references=[
          kubernetes.meta.v1.OwnerReferenceArgs(
            api_version="string",
            block_owner_deletion=True|False,
            controller=True|False,
            kind="string",
            name="string",
            uid="string",
          ),
        ],
        resource_version="string",
        self_link="string",
        uid="string",
      ),
      spec=kubernetes.core.v1.PodSpecArgs(
        active_deadline_seconds=0,
        affinity=kubernetes.core.v1.AffinityArgs(
          node_affinity=kubernetes.core.v1.NodeAffinityArgs(
            preferred_during_scheduling_ignored_during_execution=[
              kubernetes.core.v1.PreferredSchedulingTermArgs(
                preference=kubernetes.core.v1.NodeSelectorTermArgs(
                  match_expressions=[
                    kubernetes.core.v1.NodeSelectorRequirementArgs(
                      key="string",
                      operator="string",
                      values=[
                        "string",
                      ],
                    ),
                  ],
                  match_fields=[
                    kubernetes.core.v1.NodeSelectorRequirementArgs(
                      key="string",
                      operator="string",
                      values=[
                        "string",
                      ],
                    ),
                  ],
                ),
                weight=0,
              ),
            ],
            required_during_scheduling_ignored_during_execution=kubernetes.core.v1.NodeSelectorArgs(
              node_selector_terms=[
                kubernetes.core.v1.NodeSelectorTermArgs(
                  match_expressions=[
                    kubernetes.core.v1.NodeSelectorRequirementArgs(
                      key="string",
                      operator="string",
                      values=[
                        "string",
                      ],
                    ),
                  ],
                  match_fields=[
                    kubernetes.core.v1.NodeSelectorRequirementArgs(
                      key="string",
                      operator="string",
                      values=[
                        "string",
                      ],
                    ),
                  ],
                ),
              ],
            ),
          ),
          pod_affinity=kubernetes.core.v1.PodAffinityArgs(
            preferred_during_scheduling_ignored_during_execution=[
              kubernetes.core.v1.WeightedPodAffinityTermArgs(
                pod_affinity_term=kubernetes.core.v1.PodAffinityTermArgs(
                  label_selector=kubernetes.meta.v1.LabelSelectorArgs(
                    match_expressions=[
                      kubernetes.meta.v1.LabelSelectorRequirementArgs(
                        key="string",
                        operator="string",
                        values=[
                          "string",
                        ],
                      ),
                    ],
                    match_labels={
                      'string': "string"
                    },
                  ),
                  namespace_selector=kubernetes.meta.v1.LabelSelectorArgs(
                    match_expressions=[
                      kubernetes.meta.v1.LabelSelectorRequirementArgs(
                        key="string",
                        operator="string",
                        values=[
                          "string",
                        ],
                      ),
                    ],
                    match_labels={
                      'string': "string"
                    },
                  ),
                  namespaces=[
                    "string",
                  ],
                  topology_key="string",
                ),
                weight=0,
              ),
            ],
            required_during_scheduling_ignored_during_execution=[
              kubernetes.core.v1.PodAffinityTermArgs(
                label_selector=kubernetes.meta.v1.LabelSelectorArgs(
                  match_expressions=[
                    kubernetes.meta.v1.LabelSelectorRequirementArgs(
                      key="string",
                      operator="string",
                      values=[
                        "string",
                      ],
                    ),
                  ],
                  match_labels={
                    'string': "string"
                  },
                ),
                namespace_selector=kubernetes.meta.v1.LabelSelectorArgs(
                  match_expressions=[
                    kubernetes.meta.v1.LabelSelectorRequirementArgs(
                      key="string",
                      operator="string",
                      values=[
                        "string",
                      ],
                    ),
                  ],
                  match_labels={
                    'string': "string"
                  },
                ),
                namespaces=[
                  "string",
                ],
                topology_key="string",
              ),
            ],
          ),
          pod_anti_affinity=kubernetes.core.v1.PodAntiAffinityArgs(
            preferred_during_scheduling_ignored_during_execution=[
              kubernetes.core.v1.WeightedPodAffinityTermArgs(
                pod_affinity_term=kubernetes.core.v1.PodAffinityTermArgs(
                  label_selector=kubernetes.meta.v1.LabelSelectorArgs(
                    match_expressions=[
                      kubernetes.meta.v1.LabelSelectorRequirementArgs(
                        key="string",
                        operator="string",
                        values=[
                          "string",
                        ],
                      ),
                    ],
                    match_labels={
                      'string': "string"
                    },
                  ),
                  namespace_selector=kubernetes.meta.v1.LabelSelectorArgs(
                    match_expressions=[
                      kubernetes.meta.v1.LabelSelectorRequirementArgs(
                        key="string",
                        operator="string",
                        values=[
                          "string",
                        ],
                      ),
                    ],
                    match_labels={
                      'string': "string"
                    },
                  ),
                  namespaces=[
                    "string",
                  ],
                  topology_key="string",
                ),
                weight=0,
              ),
            ],
            required_during_scheduling_ignored_during_execution=[
              kubernetes.core.v1.PodAffinityTermArgs(
                label_selector=kubernetes.meta.v1.LabelSelectorArgs(
                  match_expressions=[
                    kubernetes.meta.v1.LabelSelectorRequirementArgs(
                      key="string",
                      operator="string",
                      values=[
                        "string",
                      ],
                    ),
                  ],
                  match_labels={
                    'string': "string"
                  },
                ),
                namespace_selector=kubernetes.meta.v1.LabelSelectorArgs(
                  match_expressions=[
                    kubernetes.meta.v1.LabelSelectorRequirementArgs(
                      key="string",
                      operator="string",
                      values=[
                        "string",
                      ],
                    ),
                  ],
                  match_labels={
                    'string': "string"
                  },
                ),
                namespaces=[
                  "string",
                ],
                topology_key="string",
              ),
            ],
          ),
        ),
        automount_service_account_token=True|False,
        containers=[
          kubernetes.core.v1.ContainerArgs(
            args=[
              "string",
            ],
            command=[
              "string",
            ],
            env=[
              kubernetes.core.v1.EnvVarArgs(
                name="string",
                value="string",
                value_from=kubernetes.core.v1.EnvVarSourceArgs(
                  config_map_key_ref=kubernetes.core.v1.ConfigMapKeySelectorArgs(
                    key="string",
                    name="string",
                    optional=True|False,
                  ),
                  field_ref=kubernetes.core.v1.ObjectFieldSelectorArgs(
                    api_version="string",
                    field_path="string",
                  ),
                  resource_field_ref=kubernetes.core.v1.ResourceFieldSelectorArgs(
                    container_name="string",
                    divisor="string",
                    resource="string",
                  ),
                  secret_key_ref=kubernetes.core.v1.SecretKeySelectorArgs(
                    key="string",
                    name="string",
                    optional=True|False,
                  ),
                ),
              ),
            ],
            env_from=[
              kubernetes.core.v1.EnvFromSourceArgs(
                config_map_ref=kubernetes.core.v1.ConfigMapEnvSourceArgs(
                  name="string",
                  optional=True|False,
                ),
                prefix="string",
                secret_ref=kubernetes.core.v1.SecretEnvSourceArgs(
                  name="string",
                  optional=True|False,
                ),
              ),
            ],
            image="string",
            image_pull_policy="string",
            lifecycle=kubernetes.core.v1.LifecycleArgs(
              post_start=kubernetes.core.v1.HandlerArgs(
                exec_=kubernetes.core.v1.ExecActionArgs(
                  command=[
                    "string",
                  ],
                ),
                http_get=kubernetes.core.v1.HTTPGetActionArgs(
                  host="string",
                  http_headers=[
                    kubernetes.core.v1.HTTPHeaderArgs(
                      name="string",
                      value="string",
                    ),
                  ],
                  path="string",
                  port=0,
                  scheme="string",
                ),
                tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                  host="string",
                  port=0,
                ),
              ),
              pre_stop=kubernetes.core.v1.HandlerArgs(
                exec_=kubernetes.core.v1.ExecActionArgs(
                  command=[
                    "string",
                  ],
                ),
                http_get=kubernetes.core.v1.HTTPGetActionArgs(
                  host="string",
                  http_headers=[
                    kubernetes.core.v1.HTTPHeaderArgs(
                      name="string",
                      value="string",
                    ),
                  ],
                  path="string",
                  port=0,
                  scheme="string",
                ),
                tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                  host="string",
                  port=0,
                ),
              ),
            ),
            liveness_probe=kubernetes.core.v1.ProbeArgs(
              exec_=kubernetes.core.v1.ExecActionArgs(
                command=[
                  "string",
                ],
              ),
              failure_threshold=0,
              http_get=kubernetes.core.v1.HTTPGetActionArgs(
                host="string",
                http_headers=[
                  kubernetes.core.v1.HTTPHeaderArgs(
                    name="string",
                    value="string",
                  ),
                ],
                path="string",
                port=0,
                scheme="string",
              ),
              initial_delay_seconds=0,
              period_seconds=0,
              success_threshold=0,
              tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                host="string",
                port=0,
              ),
              termination_grace_period_seconds=0,
              timeout_seconds=0,
            ),
            name="string",
            ports=[
              kubernetes.core.v1.ContainerPortArgs(
                container_port=0,
                host_ip="string",
                host_port=0,
                name="string",
                protocol="string",
              ),
            ],
            readiness_probe=kubernetes.core.v1.ProbeArgs(
              exec_=kubernetes.core.v1.ExecActionArgs(
                command=[
                  "string",
                ],
              ),
              failure_threshold=0,
              http_get=kubernetes.core.v1.HTTPGetActionArgs(
                host="string",
                http_headers=[
                  kubernetes.core.v1.HTTPHeaderArgs(
                    name="string",
                    value="string",
                  ),
                ],
                path="string",
                port=0,
                scheme="string",
              ),
              initial_delay_seconds=0,
              period_seconds=0,
              success_threshold=0,
              tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                host="string",
                port=0,
              ),
              termination_grace_period_seconds=0,
              timeout_seconds=0,
            ),
            resources=kubernetes.core.v1.ResourceRequirementsArgs(
              limits={
                'string': "string"
              },
              requests={
                'string': "string"
              },
            ),
            security_context=kubernetes.core.v1.SecurityContextArgs(
              allow_privilege_escalation=True|False,
              capabilities=kubernetes.core.v1.CapabilitiesArgs(
                add=[
                  "string",
                ],
                drop=[
                  "string",
                ],
              ),
              privileged=True|False,
              proc_mount="string",
              read_only_root_filesystem=True|False,
              run_as_group=0,
              run_as_non_root=True|False,
              run_as_user=0,
              se_linux_options=kubernetes.core.v1.SELinuxOptionsArgs(
                level="string",
                role="string",
                type="string",
                user="string",
              ),
              seccomp_profile=kubernetes.core.v1.SeccompProfileArgs(
                localhost_profile="string",
                type="string",
              ),
              windows_options=kubernetes.core.v1.WindowsSecurityContextOptionsArgs(
                gmsa_credential_spec="string",
                gmsa_credential_spec_name="string",
                host_process=True|False,
                run_as_user_name="string",
              ),
            ),
            startup_probe=kubernetes.core.v1.ProbeArgs(
              exec_=kubernetes.core.v1.ExecActionArgs(
                command=[
                  "string",
                ],
              ),
              failure_threshold=0,
              http_get=kubernetes.core.v1.HTTPGetActionArgs(
                host="string",
                http_headers=[
                  kubernetes.core.v1.HTTPHeaderArgs(
                    name="string",
                    value="string",
                  ),
                ],
                path="string",
                port=0,
                scheme="string",
              ),
              initial_delay_seconds=0,
              period_seconds=0,
              success_threshold=0,
              tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                host="string",
                port=0,
              ),
              termination_grace_period_seconds=0,
              timeout_seconds=0,
            ),
            stdin=True|False,
            stdin_once=True|False,
            termination_message_path="string",
            termination_message_policy="string",
            tty=True|False,
            volume_devices=[
              kubernetes.core.v1.VolumeDeviceArgs(
                device_path="string",
                name="string",
              ),
            ],
            volume_mounts=[
              kubernetes.core.v1.VolumeMountArgs(
                mount_path="string",
                mount_propagation="string",
                name="string",
                read_only=True|False,
                sub_path="string",
                sub_path_expr="string",
              ),
            ],
            working_dir="string",
          ),
        ],
        dns_config=kubernetes.core.v1.PodDNSConfigArgs(
          nameservers=[
            "string",
          ],
          options=[
            kubernetes.core.v1.PodDNSConfigOptionArgs(
              name="string",
              value="string",
            ),
          ],
          searches=[
            "string",
          ],
        ),
        dns_policy="string",
        enable_service_links=True|False,
        ephemeral_containers=[
          kubernetes.core.v1.EphemeralContainerArgs(
            args=[
              "string",
            ],
            command=[
              "string",
            ],
            env=[
              kubernetes.core.v1.EnvVarArgs(
                name="string",
                value="string",
                value_from=kubernetes.core.v1.EnvVarSourceArgs(
                  config_map_key_ref=kubernetes.core.v1.ConfigMapKeySelectorArgs(
                    key="string",
                    name="string",
                    optional=True|False,
                  ),
                  field_ref=kubernetes.core.v1.ObjectFieldSelectorArgs(
                    api_version="string",
                    field_path="string",
                  ),
                  resource_field_ref=kubernetes.core.v1.ResourceFieldSelectorArgs(
                    container_name="string",
                    divisor="string",
                    resource="string",
                  ),
                  secret_key_ref=kubernetes.core.v1.SecretKeySelectorArgs(
                    key="string",
                    name="string",
                    optional=True|False,
                  ),
                ),
              ),
            ],
            env_from=[
              kubernetes.core.v1.EnvFromSourceArgs(
                config_map_ref=kubernetes.core.v1.ConfigMapEnvSourceArgs(
                  name="string",
                  optional=True|False,
                ),
                prefix="string",
                secret_ref=kubernetes.core.v1.SecretEnvSourceArgs(
                  name="string",
                  optional=True|False,
                ),
              ),
            ],
            image="string",
            image_pull_policy="string",
            lifecycle=kubernetes.core.v1.LifecycleArgs(
              post_start=kubernetes.core.v1.HandlerArgs(
                exec_=kubernetes.core.v1.ExecActionArgs(
                  command=[
                    "string",
                  ],
                ),
                http_get=kubernetes.core.v1.HTTPGetActionArgs(
                  host="string",
                  http_headers=[
                    kubernetes.core.v1.HTTPHeaderArgs(
                      name="string",
                      value="string",
                    ),
                  ],
                  path="string",
                  port=0,
                  scheme="string",
                ),
                tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                  host="string",
                  port=0,
                ),
              ),
              pre_stop=kubernetes.core.v1.HandlerArgs(
                exec_=kubernetes.core.v1.ExecActionArgs(
                  command=[
                    "string",
                  ],
                ),
                http_get=kubernetes.core.v1.HTTPGetActionArgs(
                  host="string",
                  http_headers=[
                    kubernetes.core.v1.HTTPHeaderArgs(
                      name="string",
                      value="string",
                    ),
                  ],
                  path="string",
                  port=0,
                  scheme="string",
                ),
                tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                  host="string",
                  port=0,
                ),
              ),
            ),
            liveness_probe=kubernetes.core.v1.ProbeArgs(
              exec_=kubernetes.core.v1.ExecActionArgs(
                command=[
                  "string",
                ],
              ),
              failure_threshold=0,
              http_get=kubernetes.core.v1.HTTPGetActionArgs(
                host="string",
                http_headers=[
                  kubernetes.core.v1.HTTPHeaderArgs(
                    name="string",
                    value="string",
                  ),
                ],
                path="string",
                port=0,
                scheme="string",
              ),
              initial_delay_seconds=0,
              period_seconds=0,
              success_threshold=0,
              tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                host="string",
                port=0,
              ),
              termination_grace_period_seconds=0,
              timeout_seconds=0,
            ),
            name="string",
            ports=[
              kubernetes.core.v1.ContainerPortArgs(
                container_port=0,
                host_ip="string",
                host_port=0,
                name="string",
                protocol="string",
              ),
            ],
            readiness_probe=kubernetes.core.v1.ProbeArgs(
              exec_=kubernetes.core.v1.ExecActionArgs(
                command=[
                  "string",
                ],
              ),
              failure_threshold=0,
              http_get=kubernetes.core.v1.HTTPGetActionArgs(
                host="string",
                http_headers=[
                  kubernetes.core.v1.HTTPHeaderArgs(
                    name="string",
                    value="string",
                  ),
                ],
                path="string",
                port=0,
                scheme="string",
              ),
              initial_delay_seconds=0,
              period_seconds=0,
              success_threshold=0,
              tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                host="string",
                port=0,
              ),
              termination_grace_period_seconds=0,
              timeout_seconds=0,
            ),
            resources=kubernetes.core.v1.ResourceRequirementsArgs(
              limits={
                'string': "string"
              },
              requests={
                'string': "string"
              },
            ),
            security_context=kubernetes.core.v1.SecurityContextArgs(
              allow_privilege_escalation=True|False,
              capabilities=kubernetes.core.v1.CapabilitiesArgs(
                add=[
                  "string",
                ],
                drop=[
                  "string",
                ],
              ),
              privileged=True|False,
              proc_mount="string",
              read_only_root_filesystem=True|False,
              run_as_group=0,
              run_as_non_root=True|False,
              run_as_user=0,
              se_linux_options=kubernetes.core.v1.SELinuxOptionsArgs(
                level="string",
                role="string",
                type="string",
                user="string",
              ),
              seccomp_profile=kubernetes.core.v1.SeccompProfileArgs(
                localhost_profile="string",
                type="string",
              ),
              windows_options=kubernetes.core.v1.WindowsSecurityContextOptionsArgs(
                gmsa_credential_spec="string",
                gmsa_credential_spec_name="string",
                host_process=True|False,
                run_as_user_name="string",
              ),
            ),
            startup_probe=kubernetes.core.v1.ProbeArgs(
              exec_=kubernetes.core.v1.ExecActionArgs(
                command=[
                  "string",
                ],
              ),
              failure_threshold=0,
              http_get=kubernetes.core.v1.HTTPGetActionArgs(
                host="string",
                http_headers=[
                  kubernetes.core.v1.HTTPHeaderArgs(
                    name="string",
                    value="string",
                  ),
                ],
                path="string",
                port=0,
                scheme="string",
              ),
              initial_delay_seconds=0,
              period_seconds=0,
              success_threshold=0,
              tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                host="string",
                port=0,
              ),
              termination_grace_period_seconds=0,
              timeout_seconds=0,
            ),
            stdin=True|False,
            stdin_once=True|False,
            target_container_name="string",
            termination_message_path="string",
            termination_message_policy="string",
            tty=True|False,
            volume_devices=[
              kubernetes.core.v1.VolumeDeviceArgs(
                device_path="string",
                name="string",
              ),
            ],
            volume_mounts=[
              kubernetes.core.v1.VolumeMountArgs(
                mount_path="string",
                mount_propagation="string",
                name="string",
                read_only=True|False,
                sub_path="string",
                sub_path_expr="string",
              ),
            ],
            working_dir="string",
          ),
        ],
        host_aliases=[
          kubernetes.core.v1.HostAliasArgs(
            hostnames=[
              "string",
            ],
            ip="string",
          ),
        ],
        host_ipc=True|False,
        host_network=True|False,
        host_pid=True|False,
        hostname="string",
        image_pull_secrets=[
          kubernetes.core.v1.LocalObjectReferenceArgs(
            name="string",
          ),
        ],
        init_containers=[
          kubernetes.core.v1.ContainerArgs(
            args=[
              "string",
            ],
            command=[
              "string",
            ],
            env=[
              kubernetes.core.v1.EnvVarArgs(
                name="string",
                value="string",
                value_from=kubernetes.core.v1.EnvVarSourceArgs(
                  config_map_key_ref=kubernetes.core.v1.ConfigMapKeySelectorArgs(
                    key="string",
                    name="string",
                    optional=True|False,
                  ),
                  field_ref=kubernetes.core.v1.ObjectFieldSelectorArgs(
                    api_version="string",
                    field_path="string",
                  ),
                  resource_field_ref=kubernetes.core.v1.ResourceFieldSelectorArgs(
                    container_name="string",
                    divisor="string",
                    resource="string",
                  ),
                  secret_key_ref=kubernetes.core.v1.SecretKeySelectorArgs(
                    key="string",
                    name="string",
                    optional=True|False,
                  ),
                ),
              ),
            ],
            env_from=[
              kubernetes.core.v1.EnvFromSourceArgs(
                config_map_ref=kubernetes.core.v1.ConfigMapEnvSourceArgs(
                  name="string",
                  optional=True|False,
                ),
                prefix="string",
                secret_ref=kubernetes.core.v1.SecretEnvSourceArgs(
                  name="string",
                  optional=True|False,
                ),
              ),
            ],
            image="string",
            image_pull_policy="string",
            lifecycle=kubernetes.core.v1.LifecycleArgs(
              post_start=kubernetes.core.v1.HandlerArgs(
                exec_=kubernetes.core.v1.ExecActionArgs(
                  command=[
                    "string",
                  ],
                ),
                http_get=kubernetes.core.v1.HTTPGetActionArgs(
                  host="string",
                  http_headers=[
                    kubernetes.core.v1.HTTPHeaderArgs(
                      name="string",
                      value="string",
                    ),
                  ],
                  path="string",
                  port=0,
                  scheme="string",
                ),
                tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                  host="string",
                  port=0,
                ),
              ),
              pre_stop=kubernetes.core.v1.HandlerArgs(
                exec_=kubernetes.core.v1.ExecActionArgs(
                  command=[
                    "string",
                  ],
                ),
                http_get=kubernetes.core.v1.HTTPGetActionArgs(
                  host="string",
                  http_headers=[
                    kubernetes.core.v1.HTTPHeaderArgs(
                      name="string",
                      value="string",
                    ),
                  ],
                  path="string",
                  port=0,
                  scheme="string",
                ),
                tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                  host="string",
                  port=0,
                ),
              ),
            ),
            liveness_probe=kubernetes.core.v1.ProbeArgs(
              exec_=kubernetes.core.v1.ExecActionArgs(
                command=[
                  "string",
                ],
              ),
              failure_threshold=0,
              http_get=kubernetes.core.v1.HTTPGetActionArgs(
                host="string",
                http_headers=[
                  kubernetes.core.v1.HTTPHeaderArgs(
                    name="string",
                    value="string",
                  ),
                ],
                path="string",
                port=0,
                scheme="string",
              ),
              initial_delay_seconds=0,
              period_seconds=0,
              success_threshold=0,
              tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                host="string",
                port=0,
              ),
              termination_grace_period_seconds=0,
              timeout_seconds=0,
            ),
            name="string",
            ports=[
              kubernetes.core.v1.ContainerPortArgs(
                container_port=0,
                host_ip="string",
                host_port=0,
                name="string",
                protocol="string",
              ),
            ],
            readiness_probe=kubernetes.core.v1.ProbeArgs(
              exec_=kubernetes.core.v1.ExecActionArgs(
                command=[
                  "string",
                ],
              ),
              failure_threshold=0,
              http_get=kubernetes.core.v1.HTTPGetActionArgs(
                host="string",
                http_headers=[
                  kubernetes.core.v1.HTTPHeaderArgs(
                    name="string",
                    value="string",
                  ),
                ],
                path="string",
                port=0,
                scheme="string",
              ),
              initial_delay_seconds=0,
              period_seconds=0,
              success_threshold=0,
              tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                host="string",
                port=0,
              ),
              termination_grace_period_seconds=0,
              timeout_seconds=0,
            ),
            resources=kubernetes.core.v1.ResourceRequirementsArgs(
              limits={
                'string': "string"
              },
              requests={
                'string': "string"
              },
            ),
            security_context=kubernetes.core.v1.SecurityContextArgs(
              allow_privilege_escalation=True|False,
              capabilities=kubernetes.core.v1.CapabilitiesArgs(
                add=[
                  "string",
                ],
                drop=[
                  "string",
                ],
              ),
              privileged=True|False,
              proc_mount="string",
              read_only_root_filesystem=True|False,
              run_as_group=0,
              run_as_non_root=True|False,
              run_as_user=0,
              se_linux_options=kubernetes.core.v1.SELinuxOptionsArgs(
                level="string",
                role="string",
                type="string",
                user="string",
              ),
              seccomp_profile=kubernetes.core.v1.SeccompProfileArgs(
                localhost_profile="string",
                type="string",
              ),
              windows_options=kubernetes.core.v1.WindowsSecurityContextOptionsArgs(
                gmsa_credential_spec="string",
                gmsa_credential_spec_name="string",
                host_process=True|False,
                run_as_user_name="string",
              ),
            ),
            startup_probe=kubernetes.core.v1.ProbeArgs(
              exec_=kubernetes.core.v1.ExecActionArgs(
                command=[
                  "string",
                ],
              ),
              failure_threshold=0,
              http_get=kubernetes.core.v1.HTTPGetActionArgs(
                host="string",
                http_headers=[
                  kubernetes.core.v1.HTTPHeaderArgs(
                    name="string",
                    value="string",
                  ),
                ],
                path="string",
                port=0,
                scheme="string",
              ),
              initial_delay_seconds=0,
              period_seconds=0,
              success_threshold=0,
              tcp_socket=kubernetes.core.v1.TCPSocketActionArgs(
                host="string",
                port=0,
              ),
              termination_grace_period_seconds=0,
              timeout_seconds=0,
            ),
            stdin=True|False,
            stdin_once=True|False,
            termination_message_path="string",
            termination_message_policy="string",
            tty=True|False,
            volume_devices=[
              kubernetes.core.v1.VolumeDeviceArgs(
                device_path="string",
                name="string",
              ),
            ],
            volume_mounts=[
              kubernetes.core.v1.VolumeMountArgs(
                mount_path="string",
                mount_propagation="string",
                name="string",
                read_only=True|False,
                sub_path="string",
                sub_path_expr="string",
              ),
            ],
            working_dir="string",
          ),
        ],
        node_name="string",
        node_selector={
          'string': "string"
        },
        overhead={
          'string': "string"
        },
        preemption_policy="string",
        priority=0,
        priority_class_name="string",
        readiness_gates=[
          kubernetes.core.v1.PodReadinessGateArgs(
            condition_type="string",
          ),
        ],
        restart_policy="string",
        runtime_class_name="string",
        scheduler_name="string",
        security_context=kubernetes.core.v1.PodSecurityContextArgs(
          fs_group=0,
          fs_group_change_policy="string",
          run_as_group=0,
          run_as_non_root=True|False,
          run_as_user=0,
          se_linux_options=kubernetes.core.v1.SELinuxOptionsArgs(
            level="string",
            role="string",
            type="string",
            user="string",
          ),
          seccomp_profile=kubernetes.core.v1.SeccompProfileArgs(
            localhost_profile="string",
            type="string",
          ),
          supplemental_groups=[
            0,
          ],
          sysctls=[
            kubernetes.core.v1.SysctlArgs(
              name="string",
              value="string",
            ),
          ],
          windows_options=kubernetes.core.v1.WindowsSecurityContextOptionsArgs(
            gmsa_credential_spec="string",
            gmsa_credential_spec_name="string",
            host_process=True|False,
            run_as_user_name="string",
          ),
        ),
        service_account="string",
        service_account_name="string",
        set_hostname_as_fqdn=True|False,
        share_process_namespace=True|False,
        subdomain="string",
        termination_grace_period_seconds=0,
        tolerations=[
          kubernetes.core.v1.TolerationArgs(
            effect="string",
            key="string",
            operator="string",
            toleration_seconds=0,
            value="string",
          ),
        ],
        topology_spread_constraints=[
          kubernetes.core.v1.TopologySpreadConstraintArgs(
            label_selector=kubernetes.meta.v1.LabelSelectorArgs(
              match_expressions=[
                kubernetes.meta.v1.LabelSelectorRequirementArgs(
                  key="string",
                  operator="string",
                  values=[
                    "string",
                  ],
                ),
              ],
              match_labels={
                'string': "string"
              },
            ),
            max_skew=0,
            topology_key="string",
            when_unsatisfiable="string",
          ),
        ],
        volumes=[
          kubernetes.core.v1.VolumeArgs(
            aws_elastic_block_store=kubernetes.core.v1.AWSElasticBlockStoreVolumeSourceArgs(
              fs_type="string",
              partition=0,
              read_only=True|False,
              volume_id="string",
            ),
            azure_disk=kubernetes.core.v1.AzureDiskVolumeSourceArgs(
              caching_mode="string",
              disk_name="string",
              disk_uri="string",
              fs_type="string",
              kind="string",
              read_only=True|False,
            ),
            azure_file=kubernetes.core.v1.AzureFileVolumeSourceArgs(
              read_only=True|False,
              secret_name="string",
              share_name="string",
            ),
            cephfs=kubernetes.core.v1.CephFSVolumeSourceArgs(
              monitors=[
                "string",
              ],
              path="string",
              read_only=True|False,
              secret_file="string",
              secret_ref=kubernetes.core.v1.LocalObjectReferenceArgs(
                name="string",
              ),
              user="string",
            ),
            cinder=kubernetes.core.v1.CinderVolumeSourceArgs(
              fs_type="string",
              read_only=True|False,
              secret_ref=kubernetes.core.v1.LocalObjectReferenceArgs(
                name="string",
              ),
              volume_id="string",
            ),
            config_map=kubernetes.core.v1.ConfigMapVolumeSourceArgs(
              default_mode=0,
              items=[
                kubernetes.core.v1.KeyToPathArgs(
                  key="string",
                  mode=0,
                  path="string",
                ),
              ],
              name="string",
              optional=True|False,
            ),
            csi=kubernetes.core.v1.CSIVolumeSourceArgs(
              driver="string",
              fs_type="string",
              node_publish_secret_ref=kubernetes.core.v1.LocalObjectReferenceArgs(
                name="string",
              ),
              read_only=True|False,
              volume_attributes={
                'string': "string"
              },
            ),
            downward_api=kubernetes.core.v1.DownwardAPIVolumeSourceArgs(
              default_mode=0,
              items=[
                kubernetes.core.v1.DownwardAPIVolumeFileArgs(
                  field_ref=kubernetes.core.v1.ObjectFieldSelectorArgs(
                    api_version="string",
                    field_path="string",
                  ),
                  mode=0,
                  path="string",
                  resource_field_ref=kubernetes.core.v1.ResourceFieldSelectorArgs(
                    container_name="string",
                    divisor="string",
                    resource="string",
                  ),
                ),
              ],
            ),
            empty_dir=kubernetes.core.v1.EmptyDirVolumeSourceArgs(
              medium="string",
              size_limit="string",
            ),
            ephemeral=kubernetes.core.v1.EphemeralVolumeSourceArgs(
              read_only=True|False,
              volume_claim_template=kubernetes.core.v1.PersistentVolumeClaimTemplateArgs(
                metadata=kubernetes.meta.v1.ObjectMetaArgs(
                  annotations={
                    'string': "string"
                  },
                  cluster_name="string",
                  creation_timestamp="string",
                  deletion_grace_period_seconds=0,
                  deletion_timestamp="string",
                  finalizers=[
                    "string",
                  ],
                  generate_name="string",
                  generation=0,
                  labels={
                    'string': "string"
                  },
                  managed_fields=[
                    kubernetes.meta.v1.ManagedFieldsEntryArgs(
                      api_version="string",
                      fields_type="string",
                      fields_v1=,
                      manager="string",
                      operation="string",
                      subresource="string",
                      time="string",
                    ),
                  ],
                  name="string",
                  namespace="string",
                  owner_references=[
                    kubernetes.meta.v1.OwnerReferenceArgs(
                      api_version="string",
                      block_owner_deletion=True|False,
                      controller=True|False,
                      kind="string",
                      name="string",
                      uid="string",
                    ),
                  ],
                  resource_version="string",
                  self_link="string",
                  uid="string",
                ),
                spec=kubernetes.core.v1.PersistentVolumeClaimSpecArgs(
                  access_modes=[
                    "string",
                  ],
                  data_source=kubernetes.core.v1.TypedLocalObjectReferenceArgs(
                    api_group="string",
                    kind="string",
                    name="string",
                  ),
                  data_source_ref=kubernetes.core.v1.TypedLocalObjectReferenceArgs(
                    api_group="string",
                    kind="string",
                    name="string",
                  ),
                  resources=kubernetes.core.v1.ResourceRequirementsArgs(
                    limits={
                      'string': "string"
                    },
                    requests={
                      'string': "string"
                    },
                  ),
                  selector=kubernetes.meta.v1.LabelSelectorArgs(
                    match_expressions=[
                      kubernetes.meta.v1.LabelSelectorRequirementArgs(
                        key="string",
                        operator="string",
                        values=[
                          "string",
                        ],
                      ),
                    ],
                    match_labels={
                      'string': "string"
                    },
                  ),
                  storage_class_name="string",
                  volume_mode="string",
                  volume_name="string",
                ),
              ),
            ),
            fc=kubernetes.core.v1.FCVolumeSourceArgs(
              fs_type="string",
              lun=0,
              read_only=True|False,
              target_wwns=[
                "string",
              ],
              wwids=[
                "string",
              ],
            ),
            flex_volume=kubernetes.core.v1.FlexVolumeSourceArgs(
              driver="string",
              fs_type="string",
              options={
                'string': "string"
              },
              read_only=True|False,
              secret_ref=kubernetes.core.v1.LocalObjectReferenceArgs(
                name="string",
              ),
            ),
            flocker=kubernetes.core.v1.FlockerVolumeSourceArgs(
              dataset_name="string",
              dataset_uuid="string",
            ),
            gce_persistent_disk=kubernetes.core.v1.GCEPersistentDiskVolumeSourceArgs(
              fs_type="string",
              partition=0,
              pd_name="string",
              read_only=True|False,
            ),
            git_repo=kubernetes.core.v1.GitRepoVolumeSourceArgs(
              directory="string",
              repository="string",
              revision="string",
            ),
            glusterfs=kubernetes.core.v1.GlusterfsVolumeSourceArgs(
              endpoints="string",
              path="string",
              read_only=True|False,
            ),
            host_path=kubernetes.core.v1.HostPathVolumeSourceArgs(
              path="string",
              type="string",
            ),
            iscsi=kubernetes.core.v1.ISCSIVolumeSourceArgs(
              chap_auth_discovery=True|False,
              chap_auth_session=True|False,
              fs_type="string",
              initiator_name="string",
              iqn="string",
              iscsi_interface="string",
              lun=0,
              portals=[
                "string",
              ],
              read_only=True|False,
              secret_ref=kubernetes.core.v1.LocalObjectReferenceArgs(
                name="string",
              ),
              target_portal="string",
            ),
            name="string",
            nfs=kubernetes.core.v1.NFSVolumeSourceArgs(
              path="string",
              read_only=True|False,
              server="string",
            ),
            persistent_volume_claim=kubernetes.core.v1.PersistentVolumeClaimVolumeSourceArgs(
              claim_name="string",
              read_only=True|False,
            ),
            photon_persistent_disk=kubernetes.core.v1.PhotonPersistentDiskVolumeSourceArgs(
              fs_type="string",
              pd_id="string",
            ),
            portworx_volume=kubernetes.core.v1.PortworxVolumeSourceArgs(
              fs_type="string",
              read_only=True|False,
              volume_id="string",
            ),
            projected=kubernetes.core.v1.ProjectedVolumeSourceArgs(
              default_mode=0,
              sources=[
                kubernetes.core.v1.VolumeProjectionArgs(
                  config_map=kubernetes.core.v1.ConfigMapProjectionArgs(
                    items=[
                      kubernetes.core.v1.KeyToPathArgs(
                        key="string",
                        mode=0,
                        path="string",
                      ),
                    ],
                    name="string",
                    optional=True|False,
                  ),
                  downward_api=kubernetes.core.v1.DownwardAPIProjectionArgs(
                    items=[
                      kubernetes.core.v1.DownwardAPIVolumeFileArgs(
                        field_ref=kubernetes.core.v1.ObjectFieldSelectorArgs(
                          api_version="string",
                          field_path="string",
                        ),
                        mode=0,
                        path="string",
                        resource_field_ref=kubernetes.core.v1.ResourceFieldSelectorArgs(
                          container_name="string",
                          divisor="string",
                          resource="string",
                        ),
                      ),
                    ],
                  ),
                  secret=kubernetes.core.v1.SecretProjectionArgs(
                    items=[
                      kubernetes.core.v1.KeyToPathArgs(
                        key="string",
                        mode=0,
                        path="string",
                      ),
                    ],
                    name="string",
                    optional=True|False,
                  ),
                  service_account_token=kubernetes.core.v1.ServiceAccountTokenProjectionArgs(
                    audience="string",
                    expiration_seconds=0,
                    path="string",
                  ),
                ),
              ],
            ),
            quobyte=kubernetes.core.v1.QuobyteVolumeSourceArgs(
              group="string",
              read_only=True|False,
              registry="string",
              tenant="string",
              user="string",
              volume="string",
            ),
            rbd=kubernetes.core.v1.RBDVolumeSourceArgs(
              fs_type="string",
              image="string",
              keyring="string",
              monitors=[
                "string",
              ],
              pool="string",
              read_only=True|False,
              secret_ref=kubernetes.core.v1.LocalObjectReferenceArgs(
                name="string",
              ),
              user="string",
            ),
            scale_io=kubernetes.core.v1.ScaleIOVolumeSourceArgs(
              fs_type="string",
              gateway="string",
              protection_domain="string",
              read_only=True|False,
              secret_ref=kubernetes.core.v1.LocalObjectReferenceArgs(
                name="string",
              ),
              ssl_enabled=True|False,
              storage_mode="string",
              storage_pool="string",
              system="string",
              volume_name="string",
            ),
            secret=kubernetes.core.v1.SecretVolumeSourceArgs(
              default_mode=0,
              items=[
                kubernetes.core.v1.KeyToPathArgs(
                  key="string",
                  mode=0,
                  path="string",
                ),
              ],
              optional=True|False,
              secret_name="string",
            ),
            storageos=kubernetes.core.v1.StorageOSVolumeSourceArgs(
              fs_type="string",
              read_only=True|False,
              secret_ref=kubernetes.core.v1.LocalObjectReferenceArgs(
                name="string",
              ),
              volume_name="string",
              volume_namespace="string",
            ),
            vsphere_volume=kubernetes.core.v1.VsphereVirtualDiskVolumeSourceArgs(
              fs_type="string",
              storage_policy_id="string",
              storage_policy_name="string",
              volume_path="string",
            ),
          ),
        ],
      ),
    ),
  )
)
```

### C#

```csharp
using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

var deployment = new Kubernetes.Apps.V1.Deployment("deployment", new () 
{
  ApiVersion = "string",
  Kind = "string",
  Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
  {
    Annotations = {
      ["string"] = "string"
    },
    ClusterName = "string",
    CreationTimestamp = "string",
    DeletionGracePeriodSeconds = 0,
    DeletionTimestamp = "string",
    Finalizers = new []
    {
      "string"
    },
    GenerateName = "string",
    Generation = 0,
    Labels = {
      ["string"] = "string"
    },
    ManagedFields = new []
    {
      new Kubernetes.Types.Inputs.Meta.V1.ManagedFieldsEntryArgs
      {
        ApiVersion = "string",
        FieldsType = "string",
        FieldsV1 = ,
        Manager = "string",
        Operation = "string",
        Subresource = "string",
        Time = "string",
      }
    },
    Name = "string",
    Namespace = "string",
    OwnerReferences = new []
    {
      new Kubernetes.Types.Inputs.Meta.V1.OwnerReferenceArgs
      {
        ApiVersion = "string",
        BlockOwnerDeletion = true|false,
        Controller = true|false,
        Kind = "string",
        Name = "string",
        Uid = "string",
      }
    },
    ResourceVersion = "string",
    SelfLink = "string",
    Uid = "string",
  },
  Spec = new Kubernetes.Types.Inputs.Apps.V1.DeploymentSpecArgs
  {
    MinReadySeconds = 0,
    Paused = true|false,
    ProgressDeadlineSeconds = 0,
    Replicas = 0,
    RevisionHistoryLimit = 0,
    Selector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
    {
      MatchExpressions = new []
      {
        new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorRequirementArgs
        {
          Key = "string",
          Operator = "string",
          Values = new []
          {
            "string"
          },
        }
      },
      MatchLabels = {
        ["string"] = "string"
      },
    },
    Strategy = new Kubernetes.Types.Inputs.Apps.V1.DeploymentStrategyArgs
    {
      RollingUpdate = new Kubernetes.Types.Inputs.Apps.V1.RollingUpdateDeploymentArgs
      {
        MaxSurge = 0,
        MaxUnavailable = 0,
      },
      Type = "string",
    },
    Template = new Kubernetes.Types.Inputs.Core.V1.PodTemplateSpecArgs
    {
      Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
      {
        Annotations = {
          ["string"] = "string"
        },
        ClusterName = "string",
        CreationTimestamp = "string",
        DeletionGracePeriodSeconds = 0,
        DeletionTimestamp = "string",
        Finalizers = new []
        {
          "string"
        },
        GenerateName = "string",
        Generation = 0,
        Labels = {
          ["string"] = "string"
        },
        ManagedFields = new []
        {
          new Kubernetes.Types.Inputs.Meta.V1.ManagedFieldsEntryArgs
          {
            ApiVersion = "string",
            FieldsType = "string",
            FieldsV1 = ,
            Manager = "string",
            Operation = "string",
            Subresource = "string",
            Time = "string",
          }
        },
        Name = "string",
        Namespace = "string",
        OwnerReferences = new []
        {
          new Kubernetes.Types.Inputs.Meta.V1.OwnerReferenceArgs
          {
            ApiVersion = "string",
            BlockOwnerDeletion = true|false,
            Controller = true|false,
            Kind = "string",
            Name = "string",
            Uid = "string",
          }
        },
        ResourceVersion = "string",
        SelfLink = "string",
        Uid = "string",
      },
      Spec = new Kubernetes.Types.Inputs.Core.V1.PodSpecArgs
      {
        ActiveDeadlineSeconds = 0,
        Affinity = new Kubernetes.Types.Inputs.Core.V1.AffinityArgs
        {
          NodeAffinity = new Kubernetes.Types.Inputs.Core.V1.NodeAffinityArgs
          {
            PreferredDuringSchedulingIgnoredDuringExecution = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.PreferredSchedulingTermArgs
              {
                Preference = new Kubernetes.Types.Inputs.Core.V1.NodeSelectorTermArgs
                {
                  MatchExpressions = new []
                  {
                    new Kubernetes.Types.Inputs.Core.V1.NodeSelectorRequirementArgs
                    {
                      Key = "string",
                      Operator = "string",
                      Values = new []
                      {
                        "string"
                      },
                    }
                  },
                  MatchFields = new []
                  {
                    new Kubernetes.Types.Inputs.Core.V1.NodeSelectorRequirementArgs
                    {
                      Key = "string",
                      Operator = "string",
                      Values = new []
                      {
                        "string"
                      },
                    }
                  },
                },
                Weight = 0,
              }
            },
            RequiredDuringSchedulingIgnoredDuringExecution = new Kubernetes.Types.Inputs.Core.V1.NodeSelectorArgs
            {
              NodeSelectorTerms = new []
              {
                new Kubernetes.Types.Inputs.Core.V1.NodeSelectorTermArgs
                {
                  MatchExpressions = new []
                  {
                    new Kubernetes.Types.Inputs.Core.V1.NodeSelectorRequirementArgs
                    {
                      Key = "string",
                      Operator = "string",
                      Values = new []
                      {
                        "string"
                      },
                    }
                  },
                  MatchFields = new []
                  {
                    new Kubernetes.Types.Inputs.Core.V1.NodeSelectorRequirementArgs
                    {
                      Key = "string",
                      Operator = "string",
                      Values = new []
                      {
                        "string"
                      },
                    }
                  },
                }
              },
            },
          },
          PodAffinity = new Kubernetes.Types.Inputs.Core.V1.PodAffinityArgs
          {
            PreferredDuringSchedulingIgnoredDuringExecution = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.WeightedPodAffinityTermArgs
              {
                PodAffinityTerm = new Kubernetes.Types.Inputs.Core.V1.PodAffinityTermArgs
                {
                  LabelSelector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
                  {
                    MatchExpressions = new []
                    {
                      new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorRequirementArgs
                      {
                        Key = "string",
                        Operator = "string",
                        Values = new []
                        {
                          "string"
                        },
                      }
                    },
                    MatchLabels = {
                      ["string"] = "string"
                    },
                  },
                  NamespaceSelector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
                  {
                    MatchExpressions = new []
                    {
                      new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorRequirementArgs
                      {
                        Key = "string",
                        Operator = "string",
                        Values = new []
                        {
                          "string"
                        },
                      }
                    },
                    MatchLabels = {
                      ["string"] = "string"
                    },
                  },
                  Namespaces = new []
                  {
                    "string"
                  },
                  TopologyKey = "string",
                },
                Weight = 0,
              }
            },
            RequiredDuringSchedulingIgnoredDuringExecution = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.PodAffinityTermArgs
              {
                LabelSelector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
                {
                  MatchExpressions = new []
                  {
                    new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorRequirementArgs
                    {
                      Key = "string",
                      Operator = "string",
                      Values = new []
                      {
                        "string"
                      },
                    }
                  },
                  MatchLabels = {
                    ["string"] = "string"
                  },
                },
                NamespaceSelector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
                {
                  MatchExpressions = new []
                  {
                    new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorRequirementArgs
                    {
                      Key = "string",
                      Operator = "string",
                      Values = new []
                      {
                        "string"
                      },
                    }
                  },
                  MatchLabels = {
                    ["string"] = "string"
                  },
                },
                Namespaces = new []
                {
                  "string"
                },
                TopologyKey = "string",
              }
            },
          },
          PodAntiAffinity = new Kubernetes.Types.Inputs.Core.V1.PodAntiAffinityArgs
          {
            PreferredDuringSchedulingIgnoredDuringExecution = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.WeightedPodAffinityTermArgs
              {
                PodAffinityTerm = new Kubernetes.Types.Inputs.Core.V1.PodAffinityTermArgs
                {
                  LabelSelector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
                  {
                    MatchExpressions = new []
                    {
                      new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorRequirementArgs
                      {
                        Key = "string",
                        Operator = "string",
                        Values = new []
                        {
                          "string"
                        },
                      }
                    },
                    MatchLabels = {
                      ["string"] = "string"
                    },
                  },
                  NamespaceSelector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
                  {
                    MatchExpressions = new []
                    {
                      new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorRequirementArgs
                      {
                        Key = "string",
                        Operator = "string",
                        Values = new []
                        {
                          "string"
                        },
                      }
                    },
                    MatchLabels = {
                      ["string"] = "string"
                    },
                  },
                  Namespaces = new []
                  {
                    "string"
                  },
                  TopologyKey = "string",
                },
                Weight = 0,
              }
            },
            RequiredDuringSchedulingIgnoredDuringExecution = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.PodAffinityTermArgs
              {
                LabelSelector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
                {
                  MatchExpressions = new []
                  {
                    new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorRequirementArgs
                    {
                      Key = "string",
                      Operator = "string",
                      Values = new []
                      {
                        "string"
                      },
                    }
                  },
                  MatchLabels = {
                    ["string"] = "string"
                  },
                },
                NamespaceSelector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
                {
                  MatchExpressions = new []
                  {
                    new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorRequirementArgs
                    {
                      Key = "string",
                      Operator = "string",
                      Values = new []
                      {
                        "string"
                      },
                    }
                  },
                  MatchLabels = {
                    ["string"] = "string"
                  },
                },
                Namespaces = new []
                {
                  "string"
                },
                TopologyKey = "string",
              }
            },
          },
        },
        AutomountServiceAccountToken = true|false,
        Containers = new []
        {
          new Kubernetes.Types.Inputs.Core.V1.ContainerArgs
          {
            Args = new []
            {
              "string"
            },
            Command = new []
            {
              "string"
            },
            Env = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.EnvVarArgs
              {
                Name = "string",
                Value = "string",
                ValueFrom = new Kubernetes.Types.Inputs.Core.V1.EnvVarSourceArgs
                {
                  ConfigMapKeyRef = new Kubernetes.Types.Inputs.Core.V1.ConfigMapKeySelectorArgs
                  {
                    Key = "string",
                    Name = "string",
                    Optional = true|false,
                  },
                  FieldRef = new Kubernetes.Types.Inputs.Core.V1.ObjectFieldSelectorArgs
                  {
                    ApiVersion = "string",
                    FieldPath = "string",
                  },
                  ResourceFieldRef = new Kubernetes.Types.Inputs.Core.V1.ResourceFieldSelectorArgs
                  {
                    ContainerName = "string",
                    Divisor = "string",
                    Resource = "string",
                  },
                  SecretKeyRef = new Kubernetes.Types.Inputs.Core.V1.SecretKeySelectorArgs
                  {
                    Key = "string",
                    Name = "string",
                    Optional = true|false,
                  },
                },
              }
            },
            EnvFrom = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.EnvFromSourceArgs
              {
                ConfigMapRef = new Kubernetes.Types.Inputs.Core.V1.ConfigMapEnvSourceArgs
                {
                  Name = "string",
                  Optional = true|false,
                },
                Prefix = "string",
                SecretRef = new Kubernetes.Types.Inputs.Core.V1.SecretEnvSourceArgs
                {
                  Name = "string",
                  Optional = true|false,
                },
              }
            },
            Image = "string",
            ImagePullPolicy = "string",
            Lifecycle = new Kubernetes.Types.Inputs.Core.V1.LifecycleArgs
            {
              PostStart = new Kubernetes.Types.Inputs.Core.V1.HandlerArgs
              {
                Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
                {
                  Command = new []
                  {
                    "string"
                  },
                },
                HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
                {
                  Host = "string",
                  HttpHeaders = new []
                  {
                    new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                    {
                      Name = "string",
                      Value = "string",
                    }
                  },
                  Path = "string",
                  Port = 0,
                  Scheme = "string",
                },
                TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
                {
                  Host = "string",
                  Port = 0,
                },
              },
              PreStop = new Kubernetes.Types.Inputs.Core.V1.HandlerArgs
              {
                Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
                {
                  Command = new []
                  {
                    "string"
                  },
                },
                HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
                {
                  Host = "string",
                  HttpHeaders = new []
                  {
                    new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                    {
                      Name = "string",
                      Value = "string",
                    }
                  },
                  Path = "string",
                  Port = 0,
                  Scheme = "string",
                },
                TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
                {
                  Host = "string",
                  Port = 0,
                },
              },
            },
            LivenessProbe = new Kubernetes.Types.Inputs.Core.V1.ProbeArgs
            {
              Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
              {
                Command = new []
                {
                  "string"
                },
              },
              FailureThreshold = 0,
              HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
              {
                Host = "string",
                HttpHeaders = new []
                {
                  new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                  {
                    Name = "string",
                    Value = "string",
                  }
                },
                Path = "string",
                Port = 0,
                Scheme = "string",
              },
              InitialDelaySeconds = 0,
              PeriodSeconds = 0,
              SuccessThreshold = 0,
              TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
              {
                Host = "string",
                Port = 0,
              },
              TerminationGracePeriodSeconds = 0,
              TimeoutSeconds = 0,
            },
            Name = "string",
            Ports = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.ContainerPortArgs
              {
                ContainerPort = 0,
                HostIP = "string",
                HostPort = 0,
                Name = "string",
                Protocol = "string",
              }
            },
            ReadinessProbe = new Kubernetes.Types.Inputs.Core.V1.ProbeArgs
            {
              Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
              {
                Command = new []
                {
                  "string"
                },
              },
              FailureThreshold = 0,
              HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
              {
                Host = "string",
                HttpHeaders = new []
                {
                  new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                  {
                    Name = "string",
                    Value = "string",
                  }
                },
                Path = "string",
                Port = 0,
                Scheme = "string",
              },
              InitialDelaySeconds = 0,
              PeriodSeconds = 0,
              SuccessThreshold = 0,
              TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
              {
                Host = "string",
                Port = 0,
              },
              TerminationGracePeriodSeconds = 0,
              TimeoutSeconds = 0,
            },
            Resources = new Kubernetes.Types.Inputs.Core.V1.ResourceRequirementsArgs
            {
              Limits = {
                ["string"] = "string"
              },
              Requests = {
                ["string"] = "string"
              },
            },
            SecurityContext = new Kubernetes.Types.Inputs.Core.V1.SecurityContextArgs
            {
              AllowPrivilegeEscalation = true|false,
              Capabilities = new Kubernetes.Types.Inputs.Core.V1.CapabilitiesArgs
              {
                Add = new []
                {
                  "string"
                },
                Drop = new []
                {
                  "string"
                },
              },
              Privileged = true|false,
              ProcMount = "string",
              ReadOnlyRootFilesystem = true|false,
              RunAsGroup = 0,
              RunAsNonRoot = true|false,
              RunAsUser = 0,
              SeLinuxOptions = new Kubernetes.Types.Inputs.Core.V1.SELinuxOptionsArgs
              {
                Level = "string",
                Role = "string",
                Type = "string",
                User = "string",
              },
              SeccompProfile = new Kubernetes.Types.Inputs.Core.V1.SeccompProfileArgs
              {
                LocalhostProfile = "string",
                Type = "string",
              },
              WindowsOptions = new Kubernetes.Types.Inputs.Core.V1.WindowsSecurityContextOptionsArgs
              {
                GmsaCredentialSpec = "string",
                GmsaCredentialSpecName = "string",
                HostProcess = true|false,
                RunAsUserName = "string",
              },
            },
            StartupProbe = new Kubernetes.Types.Inputs.Core.V1.ProbeArgs
            {
              Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
              {
                Command = new []
                {
                  "string"
                },
              },
              FailureThreshold = 0,
              HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
              {
                Host = "string",
                HttpHeaders = new []
                {
                  new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                  {
                    Name = "string",
                    Value = "string",
                  }
                },
                Path = "string",
                Port = 0,
                Scheme = "string",
              },
              InitialDelaySeconds = 0,
              PeriodSeconds = 0,
              SuccessThreshold = 0,
              TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
              {
                Host = "string",
                Port = 0,
              },
              TerminationGracePeriodSeconds = 0,
              TimeoutSeconds = 0,
            },
            Stdin = true|false,
            StdinOnce = true|false,
            TerminationMessagePath = "string",
            TerminationMessagePolicy = "string",
            Tty = true|false,
            VolumeDevices = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.VolumeDeviceArgs
              {
                DevicePath = "string",
                Name = "string",
              }
            },
            VolumeMounts = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.VolumeMountArgs
              {
                MountPath = "string",
                MountPropagation = "string",
                Name = "string",
                ReadOnly = true|false,
                SubPath = "string",
                SubPathExpr = "string",
              }
            },
            WorkingDir = "string",
          }
        },
        DnsConfig = new Kubernetes.Types.Inputs.Core.V1.PodDNSConfigArgs
        {
          Nameservers = new []
          {
            "string"
          },
          Options = new []
          {
            new Kubernetes.Types.Inputs.Core.V1.PodDNSConfigOptionArgs
            {
              Name = "string",
              Value = "string",
            }
          },
          Searches = new []
          {
            "string"
          },
        },
        DnsPolicy = "string",
        EnableServiceLinks = true|false,
        EphemeralContainers = new []
        {
          new Kubernetes.Types.Inputs.Core.V1.EphemeralContainerArgs
          {
            Args = new []
            {
              "string"
            },
            Command = new []
            {
              "string"
            },
            Env = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.EnvVarArgs
              {
                Name = "string",
                Value = "string",
                ValueFrom = new Kubernetes.Types.Inputs.Core.V1.EnvVarSourceArgs
                {
                  ConfigMapKeyRef = new Kubernetes.Types.Inputs.Core.V1.ConfigMapKeySelectorArgs
                  {
                    Key = "string",
                    Name = "string",
                    Optional = true|false,
                  },
                  FieldRef = new Kubernetes.Types.Inputs.Core.V1.ObjectFieldSelectorArgs
                  {
                    ApiVersion = "string",
                    FieldPath = "string",
                  },
                  ResourceFieldRef = new Kubernetes.Types.Inputs.Core.V1.ResourceFieldSelectorArgs
                  {
                    ContainerName = "string",
                    Divisor = "string",
                    Resource = "string",
                  },
                  SecretKeyRef = new Kubernetes.Types.Inputs.Core.V1.SecretKeySelectorArgs
                  {
                    Key = "string",
                    Name = "string",
                    Optional = true|false,
                  },
                },
              }
            },
            EnvFrom = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.EnvFromSourceArgs
              {
                ConfigMapRef = new Kubernetes.Types.Inputs.Core.V1.ConfigMapEnvSourceArgs
                {
                  Name = "string",
                  Optional = true|false,
                },
                Prefix = "string",
                SecretRef = new Kubernetes.Types.Inputs.Core.V1.SecretEnvSourceArgs
                {
                  Name = "string",
                  Optional = true|false,
                },
              }
            },
            Image = "string",
            ImagePullPolicy = "string",
            Lifecycle = new Kubernetes.Types.Inputs.Core.V1.LifecycleArgs
            {
              PostStart = new Kubernetes.Types.Inputs.Core.V1.HandlerArgs
              {
                Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
                {
                  Command = new []
                  {
                    "string"
                  },
                },
                HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
                {
                  Host = "string",
                  HttpHeaders = new []
                  {
                    new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                    {
                      Name = "string",
                      Value = "string",
                    }
                  },
                  Path = "string",
                  Port = 0,
                  Scheme = "string",
                },
                TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
                {
                  Host = "string",
                  Port = 0,
                },
              },
              PreStop = new Kubernetes.Types.Inputs.Core.V1.HandlerArgs
              {
                Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
                {
                  Command = new []
                  {
                    "string"
                  },
                },
                HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
                {
                  Host = "string",
                  HttpHeaders = new []
                  {
                    new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                    {
                      Name = "string",
                      Value = "string",
                    }
                  },
                  Path = "string",
                  Port = 0,
                  Scheme = "string",
                },
                TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
                {
                  Host = "string",
                  Port = 0,
                },
              },
            },
            LivenessProbe = new Kubernetes.Types.Inputs.Core.V1.ProbeArgs
            {
              Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
              {
                Command = new []
                {
                  "string"
                },
              },
              FailureThreshold = 0,
              HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
              {
                Host = "string",
                HttpHeaders = new []
                {
                  new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                  {
                    Name = "string",
                    Value = "string",
                  }
                },
                Path = "string",
                Port = 0,
                Scheme = "string",
              },
              InitialDelaySeconds = 0,
              PeriodSeconds = 0,
              SuccessThreshold = 0,
              TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
              {
                Host = "string",
                Port = 0,
              },
              TerminationGracePeriodSeconds = 0,
              TimeoutSeconds = 0,
            },
            Name = "string",
            Ports = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.ContainerPortArgs
              {
                ContainerPort = 0,
                HostIP = "string",
                HostPort = 0,
                Name = "string",
                Protocol = "string",
              }
            },
            ReadinessProbe = new Kubernetes.Types.Inputs.Core.V1.ProbeArgs
            {
              Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
              {
                Command = new []
                {
                  "string"
                },
              },
              FailureThreshold = 0,
              HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
              {
                Host = "string",
                HttpHeaders = new []
                {
                  new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                  {
                    Name = "string",
                    Value = "string",
                  }
                },
                Path = "string",
                Port = 0,
                Scheme = "string",
              },
              InitialDelaySeconds = 0,
              PeriodSeconds = 0,
              SuccessThreshold = 0,
              TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
              {
                Host = "string",
                Port = 0,
              },
              TerminationGracePeriodSeconds = 0,
              TimeoutSeconds = 0,
            },
            Resources = new Kubernetes.Types.Inputs.Core.V1.ResourceRequirementsArgs
            {
              Limits = {
                ["string"] = "string"
              },
              Requests = {
                ["string"] = "string"
              },
            },
            SecurityContext = new Kubernetes.Types.Inputs.Core.V1.SecurityContextArgs
            {
              AllowPrivilegeEscalation = true|false,
              Capabilities = new Kubernetes.Types.Inputs.Core.V1.CapabilitiesArgs
              {
                Add = new []
                {
                  "string"
                },
                Drop = new []
                {
                  "string"
                },
              },
              Privileged = true|false,
              ProcMount = "string",
              ReadOnlyRootFilesystem = true|false,
              RunAsGroup = 0,
              RunAsNonRoot = true|false,
              RunAsUser = 0,
              SeLinuxOptions = new Kubernetes.Types.Inputs.Core.V1.SELinuxOptionsArgs
              {
                Level = "string",
                Role = "string",
                Type = "string",
                User = "string",
              },
              SeccompProfile = new Kubernetes.Types.Inputs.Core.V1.SeccompProfileArgs
              {
                LocalhostProfile = "string",
                Type = "string",
              },
              WindowsOptions = new Kubernetes.Types.Inputs.Core.V1.WindowsSecurityContextOptionsArgs
              {
                GmsaCredentialSpec = "string",
                GmsaCredentialSpecName = "string",
                HostProcess = true|false,
                RunAsUserName = "string",
              },
            },
            StartupProbe = new Kubernetes.Types.Inputs.Core.V1.ProbeArgs
            {
              Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
              {
                Command = new []
                {
                  "string"
                },
              },
              FailureThreshold = 0,
              HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
              {
                Host = "string",
                HttpHeaders = new []
                {
                  new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                  {
                    Name = "string",
                    Value = "string",
                  }
                },
                Path = "string",
                Port = 0,
                Scheme = "string",
              },
              InitialDelaySeconds = 0,
              PeriodSeconds = 0,
              SuccessThreshold = 0,
              TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
              {
                Host = "string",
                Port = 0,
              },
              TerminationGracePeriodSeconds = 0,
              TimeoutSeconds = 0,
            },
            Stdin = true|false,
            StdinOnce = true|false,
            TargetContainerName = "string",
            TerminationMessagePath = "string",
            TerminationMessagePolicy = "string",
            Tty = true|false,
            VolumeDevices = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.VolumeDeviceArgs
              {
                DevicePath = "string",
                Name = "string",
              }
            },
            VolumeMounts = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.VolumeMountArgs
              {
                MountPath = "string",
                MountPropagation = "string",
                Name = "string",
                ReadOnly = true|false,
                SubPath = "string",
                SubPathExpr = "string",
              }
            },
            WorkingDir = "string",
          }
        },
        HostAliases = new []
        {
          new Kubernetes.Types.Inputs.Core.V1.HostAliasArgs
          {
            Hostnames = new []
            {
              "string"
            },
            Ip = "string",
          }
        },
        HostIPC = true|false,
        HostNetwork = true|false,
        HostPID = true|false,
        Hostname = "string",
        ImagePullSecrets = new []
        {
          new Kubernetes.Types.Inputs.Core.V1.LocalObjectReferenceArgs
          {
            Name = "string",
          }
        },
        InitContainers = new []
        {
          new Kubernetes.Types.Inputs.Core.V1.ContainerArgs
          {
            Args = new []
            {
              "string"
            },
            Command = new []
            {
              "string"
            },
            Env = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.EnvVarArgs
              {
                Name = "string",
                Value = "string",
                ValueFrom = new Kubernetes.Types.Inputs.Core.V1.EnvVarSourceArgs
                {
                  ConfigMapKeyRef = new Kubernetes.Types.Inputs.Core.V1.ConfigMapKeySelectorArgs
                  {
                    Key = "string",
                    Name = "string",
                    Optional = true|false,
                  },
                  FieldRef = new Kubernetes.Types.Inputs.Core.V1.ObjectFieldSelectorArgs
                  {
                    ApiVersion = "string",
                    FieldPath = "string",
                  },
                  ResourceFieldRef = new Kubernetes.Types.Inputs.Core.V1.ResourceFieldSelectorArgs
                  {
                    ContainerName = "string",
                    Divisor = "string",
                    Resource = "string",
                  },
                  SecretKeyRef = new Kubernetes.Types.Inputs.Core.V1.SecretKeySelectorArgs
                  {
                    Key = "string",
                    Name = "string",
                    Optional = true|false,
                  },
                },
              }
            },
            EnvFrom = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.EnvFromSourceArgs
              {
                ConfigMapRef = new Kubernetes.Types.Inputs.Core.V1.ConfigMapEnvSourceArgs
                {
                  Name = "string",
                  Optional = true|false,
                },
                Prefix = "string",
                SecretRef = new Kubernetes.Types.Inputs.Core.V1.SecretEnvSourceArgs
                {
                  Name = "string",
                  Optional = true|false,
                },
              }
            },
            Image = "string",
            ImagePullPolicy = "string",
            Lifecycle = new Kubernetes.Types.Inputs.Core.V1.LifecycleArgs
            {
              PostStart = new Kubernetes.Types.Inputs.Core.V1.HandlerArgs
              {
                Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
                {
                  Command = new []
                  {
                    "string"
                  },
                },
                HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
                {
                  Host = "string",
                  HttpHeaders = new []
                  {
                    new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                    {
                      Name = "string",
                      Value = "string",
                    }
                  },
                  Path = "string",
                  Port = 0,
                  Scheme = "string",
                },
                TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
                {
                  Host = "string",
                  Port = 0,
                },
              },
              PreStop = new Kubernetes.Types.Inputs.Core.V1.HandlerArgs
              {
                Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
                {
                  Command = new []
                  {
                    "string"
                  },
                },
                HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
                {
                  Host = "string",
                  HttpHeaders = new []
                  {
                    new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                    {
                      Name = "string",
                      Value = "string",
                    }
                  },
                  Path = "string",
                  Port = 0,
                  Scheme = "string",
                },
                TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
                {
                  Host = "string",
                  Port = 0,
                },
              },
            },
            LivenessProbe = new Kubernetes.Types.Inputs.Core.V1.ProbeArgs
            {
              Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
              {
                Command = new []
                {
                  "string"
                },
              },
              FailureThreshold = 0,
              HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
              {
                Host = "string",
                HttpHeaders = new []
                {
                  new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                  {
                    Name = "string",
                    Value = "string",
                  }
                },
                Path = "string",
                Port = 0,
                Scheme = "string",
              },
              InitialDelaySeconds = 0,
              PeriodSeconds = 0,
              SuccessThreshold = 0,
              TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
              {
                Host = "string",
                Port = 0,
              },
              TerminationGracePeriodSeconds = 0,
              TimeoutSeconds = 0,
            },
            Name = "string",
            Ports = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.ContainerPortArgs
              {
                ContainerPort = 0,
                HostIP = "string",
                HostPort = 0,
                Name = "string",
                Protocol = "string",
              }
            },
            ReadinessProbe = new Kubernetes.Types.Inputs.Core.V1.ProbeArgs
            {
              Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
              {
                Command = new []
                {
                  "string"
                },
              },
              FailureThreshold = 0,
              HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
              {
                Host = "string",
                HttpHeaders = new []
                {
                  new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                  {
                    Name = "string",
                    Value = "string",
                  }
                },
                Path = "string",
                Port = 0,
                Scheme = "string",
              },
              InitialDelaySeconds = 0,
              PeriodSeconds = 0,
              SuccessThreshold = 0,
              TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
              {
                Host = "string",
                Port = 0,
              },
              TerminationGracePeriodSeconds = 0,
              TimeoutSeconds = 0,
            },
            Resources = new Kubernetes.Types.Inputs.Core.V1.ResourceRequirementsArgs
            {
              Limits = {
                ["string"] = "string"
              },
              Requests = {
                ["string"] = "string"
              },
            },
            SecurityContext = new Kubernetes.Types.Inputs.Core.V1.SecurityContextArgs
            {
              AllowPrivilegeEscalation = true|false,
              Capabilities = new Kubernetes.Types.Inputs.Core.V1.CapabilitiesArgs
              {
                Add = new []
                {
                  "string"
                },
                Drop = new []
                {
                  "string"
                },
              },
              Privileged = true|false,
              ProcMount = "string",
              ReadOnlyRootFilesystem = true|false,
              RunAsGroup = 0,
              RunAsNonRoot = true|false,
              RunAsUser = 0,
              SeLinuxOptions = new Kubernetes.Types.Inputs.Core.V1.SELinuxOptionsArgs
              {
                Level = "string",
                Role = "string",
                Type = "string",
                User = "string",
              },
              SeccompProfile = new Kubernetes.Types.Inputs.Core.V1.SeccompProfileArgs
              {
                LocalhostProfile = "string",
                Type = "string",
              },
              WindowsOptions = new Kubernetes.Types.Inputs.Core.V1.WindowsSecurityContextOptionsArgs
              {
                GmsaCredentialSpec = "string",
                GmsaCredentialSpecName = "string",
                HostProcess = true|false,
                RunAsUserName = "string",
              },
            },
            StartupProbe = new Kubernetes.Types.Inputs.Core.V1.ProbeArgs
            {
              Exec = new Kubernetes.Types.Inputs.Core.V1.ExecActionArgs
              {
                Command = new []
                {
                  "string"
                },
              },
              FailureThreshold = 0,
              HttpGet = new Kubernetes.Types.Inputs.Core.V1.HTTPGetActionArgs
              {
                Host = "string",
                HttpHeaders = new []
                {
                  new Kubernetes.Types.Inputs.Core.V1.HTTPHeaderArgs
                  {
                    Name = "string",
                    Value = "string",
                  }
                },
                Path = "string",
                Port = 0,
                Scheme = "string",
              },
              InitialDelaySeconds = 0,
              PeriodSeconds = 0,
              SuccessThreshold = 0,
              TcpSocket = new Kubernetes.Types.Inputs.Core.V1.TCPSocketActionArgs
              {
                Host = "string",
                Port = 0,
              },
              TerminationGracePeriodSeconds = 0,
              TimeoutSeconds = 0,
            },
            Stdin = true|false,
            StdinOnce = true|false,
            TerminationMessagePath = "string",
            TerminationMessagePolicy = "string",
            Tty = true|false,
            VolumeDevices = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.VolumeDeviceArgs
              {
                DevicePath = "string",
                Name = "string",
              }
            },
            VolumeMounts = new []
            {
              new Kubernetes.Types.Inputs.Core.V1.VolumeMountArgs
              {
                MountPath = "string",
                MountPropagation = "string",
                Name = "string",
                ReadOnly = true|false,
                SubPath = "string",
                SubPathExpr = "string",
              }
            },
            WorkingDir = "string",
          }
        },
        NodeName = "string",
        NodeSelector = {
          ["string"] = "string"
        },
        Overhead = {
          ["string"] = "string"
        },
        PreemptionPolicy = "string",
        Priority = 0,
        PriorityClassName = "string",
        ReadinessGates = new []
        {
          new Kubernetes.Types.Inputs.Core.V1.PodReadinessGateArgs
          {
            ConditionType = "string",
          }
        },
        RestartPolicy = "string",
        RuntimeClassName = "string",
        SchedulerName = "string",
        SecurityContext = new Kubernetes.Types.Inputs.Core.V1.PodSecurityContextArgs
        {
          FsGroup = 0,
          FsGroupChangePolicy = "string",
          RunAsGroup = 0,
          RunAsNonRoot = true|false,
          RunAsUser = 0,
          SeLinuxOptions = new Kubernetes.Types.Inputs.Core.V1.SELinuxOptionsArgs
          {
            Level = "string",
            Role = "string",
            Type = "string",
            User = "string",
          },
          SeccompProfile = new Kubernetes.Types.Inputs.Core.V1.SeccompProfileArgs
          {
            LocalhostProfile = "string",
            Type = "string",
          },
          SupplementalGroups = new []
          {
            0
          },
          Sysctls = new []
          {
            new Kubernetes.Types.Inputs.Core.V1.SysctlArgs
            {
              Name = "string",
              Value = "string",
            }
          },
          WindowsOptions = new Kubernetes.Types.Inputs.Core.V1.WindowsSecurityContextOptionsArgs
          {
            GmsaCredentialSpec = "string",
            GmsaCredentialSpecName = "string",
            HostProcess = true|false,
            RunAsUserName = "string",
          },
        },
        ServiceAccount = "string",
        ServiceAccountName = "string",
        SetHostnameAsFQDN = true|false,
        ShareProcessNamespace = true|false,
        Subdomain = "string",
        TerminationGracePeriodSeconds = 0,
        Tolerations = new []
        {
          new Kubernetes.Types.Inputs.Core.V1.TolerationArgs
          {
            Effect = "string",
            Key = "string",
            Operator = "string",
            TolerationSeconds = 0,
            Value = "string",
          }
        },
        TopologySpreadConstraints = new []
        {
          new Kubernetes.Types.Inputs.Core.V1.TopologySpreadConstraintArgs
          {
            LabelSelector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
            {
              MatchExpressions = new []
              {
                new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorRequirementArgs
                {
                  Key = "string",
                  Operator = "string",
                  Values = new []
                  {
                    "string"
                  },
                }
              },
              MatchLabels = {
                ["string"] = "string"
              },
            },
            MaxSkew = 0,
            TopologyKey = "string",
            WhenUnsatisfiable = "string",
          }
        },
        Volumes = new []
        {
          new Kubernetes.Types.Inputs.Core.V1.VolumeArgs
          {
            AwsElasticBlockStore = new Kubernetes.Types.Inputs.Core.V1.AWSElasticBlockStoreVolumeSourceArgs
            {
              FsType = "string",
              Partition = 0,
              ReadOnly = true|false,
              VolumeID = "string",
            },
            AzureDisk = new Kubernetes.Types.Inputs.Core.V1.AzureDiskVolumeSourceArgs
            {
              CachingMode = "string",
              DiskName = "string",
              DiskURI = "string",
              FsType = "string",
              Kind = "string",
              ReadOnly = true|false,
            },
            AzureFile = new Kubernetes.Types.Inputs.Core.V1.AzureFileVolumeSourceArgs
            {
              ReadOnly = true|false,
              SecretName = "string",
              ShareName = "string",
            },
            Cephfs = new Kubernetes.Types.Inputs.Core.V1.CephFSVolumeSourceArgs
            {
              Monitors = new []
              {
                "string"
              },
              Path = "string",
              ReadOnly = true|false,
              SecretFile = "string",
              SecretRef = new Kubernetes.Types.Inputs.Core.V1.LocalObjectReferenceArgs
              {
                Name = "string",
              },
              User = "string",
            },
            Cinder = new Kubernetes.Types.Inputs.Core.V1.CinderVolumeSourceArgs
            {
              FsType = "string",
              ReadOnly = true|false,
              SecretRef = new Kubernetes.Types.Inputs.Core.V1.LocalObjectReferenceArgs
              {
                Name = "string",
              },
              VolumeID = "string",
            },
            ConfigMap = new Kubernetes.Types.Inputs.Core.V1.ConfigMapVolumeSourceArgs
            {
              DefaultMode = 0,
              Items = new []
              {
                new Kubernetes.Types.Inputs.Core.V1.KeyToPathArgs
                {
                  Key = "string",
                  Mode = 0,
                  Path = "string",
                }
              },
              Name = "string",
              Optional = true|false,
            },
            Csi = new Kubernetes.Types.Inputs.Core.V1.CSIVolumeSourceArgs
            {
              Driver = "string",
              FsType = "string",
              NodePublishSecretRef = new Kubernetes.Types.Inputs.Core.V1.LocalObjectReferenceArgs
              {
                Name = "string",
              },
              ReadOnly = true|false,
              VolumeAttributes = {
                ["string"] = "string"
              },
            },
            DownwardAPI = new Kubernetes.Types.Inputs.Core.V1.DownwardAPIVolumeSourceArgs
            {
              DefaultMode = 0,
              Items = new []
              {
                new Kubernetes.Types.Inputs.Core.V1.DownwardAPIVolumeFileArgs
                {
                  FieldRef = new Kubernetes.Types.Inputs.Core.V1.ObjectFieldSelectorArgs
                  {
                    ApiVersion = "string",
                    FieldPath = "string",
                  },
                  Mode = 0,
                  Path = "string",
                  ResourceFieldRef = new Kubernetes.Types.Inputs.Core.V1.ResourceFieldSelectorArgs
                  {
                    ContainerName = "string",
                    Divisor = "string",
                    Resource = "string",
                  },
                }
              },
            },
            EmptyDir = new Kubernetes.Types.Inputs.Core.V1.EmptyDirVolumeSourceArgs
            {
              Medium = "string",
              SizeLimit = "string",
            },
            Ephemeral = new Kubernetes.Types.Inputs.Core.V1.EphemeralVolumeSourceArgs
            {
              ReadOnly = true|false,
              VolumeClaimTemplate = new Kubernetes.Types.Inputs.Core.V1.PersistentVolumeClaimTemplateArgs
              {
                Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
                {
                  Annotations = {
                    ["string"] = "string"
                  },
                  ClusterName = "string",
                  CreationTimestamp = "string",
                  DeletionGracePeriodSeconds = 0,
                  DeletionTimestamp = "string",
                  Finalizers = new []
                  {
                    "string"
                  },
                  GenerateName = "string",
                  Generation = 0,
                  Labels = {
                    ["string"] = "string"
                  },
                  ManagedFields = new []
                  {
                    new Kubernetes.Types.Inputs.Meta.V1.ManagedFieldsEntryArgs
                    {
                      ApiVersion = "string",
                      FieldsType = "string",
                      FieldsV1 = ,
                      Manager = "string",
                      Operation = "string",
                      Subresource = "string",
                      Time = "string",
                    }
                  },
                  Name = "string",
                  Namespace = "string",
                  OwnerReferences = new []
                  {
                    new Kubernetes.Types.Inputs.Meta.V1.OwnerReferenceArgs
                    {
                      ApiVersion = "string",
                      BlockOwnerDeletion = true|false,
                      Controller = true|false,
                      Kind = "string",
                      Name = "string",
                      Uid = "string",
                    }
                  },
                  ResourceVersion = "string",
                  SelfLink = "string",
                  Uid = "string",
                },
                Spec = new Kubernetes.Types.Inputs.Core.V1.PersistentVolumeClaimSpecArgs
                {
                  AccessModes = new []
                  {
                    "string"
                  },
                  DataSource = new Kubernetes.Types.Inputs.Core.V1.TypedLocalObjectReferenceArgs
                  {
                    ApiGroup = "string",
                    Kind = "string",
                    Name = "string",
                  },
                  DataSourceRef = new Kubernetes.Types.Inputs.Core.V1.TypedLocalObjectReferenceArgs
                  {
                    ApiGroup = "string",
                    Kind = "string",
                    Name = "string",
                  },
                  Resources = new Kubernetes.Types.Inputs.Core.V1.ResourceRequirementsArgs
                  {
                    Limits = {
                      ["string"] = "string"
                    },
                    Requests = {
                      ["string"] = "string"
                    },
                  },
                  Selector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
                  {
                    MatchExpressions = new []
                    {
                      new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorRequirementArgs
                      {
                        Key = "string",
                        Operator = "string",
                        Values = new []
                        {
                          "string"
                        },
                      }
                    },
                    MatchLabels = {
                      ["string"] = "string"
                    },
                  },
                  StorageClassName = "string",
                  VolumeMode = "string",
                  VolumeName = "string",
                },
              },
            },
            Fc = new Kubernetes.Types.Inputs.Core.V1.FCVolumeSourceArgs
            {
              FsType = "string",
              Lun = 0,
              ReadOnly = true|false,
              TargetWWNs = new []
              {
                "string"
              },
              Wwids = new []
              {
                "string"
              },
            },
            FlexVolume = new Kubernetes.Types.Inputs.Core.V1.FlexVolumeSourceArgs
            {
              Driver = "string",
              FsType = "string",
              Options = {
                ["string"] = "string"
              },
              ReadOnly = true|false,
              SecretRef = new Kubernetes.Types.Inputs.Core.V1.LocalObjectReferenceArgs
              {
                Name = "string",
              },
            },
            Flocker = new Kubernetes.Types.Inputs.Core.V1.FlockerVolumeSourceArgs
            {
              DatasetName = "string",
              DatasetUUID = "string",
            },
            GcePersistentDisk = new Kubernetes.Types.Inputs.Core.V1.GCEPersistentDiskVolumeSourceArgs
            {
              FsType = "string",
              Partition = 0,
              PdName = "string",
              ReadOnly = true|false,
            },
            GitRepo = new Kubernetes.Types.Inputs.Core.V1.GitRepoVolumeSourceArgs
            {
              Directory = "string",
              Repository = "string",
              Revision = "string",
            },
            Glusterfs = new Kubernetes.Types.Inputs.Core.V1.GlusterfsVolumeSourceArgs
            {
              Endpoints = "string",
              Path = "string",
              ReadOnly = true|false,
            },
            HostPath = new Kubernetes.Types.Inputs.Core.V1.HostPathVolumeSourceArgs
            {
              Path = "string",
              Type = "string",
            },
            Iscsi = new Kubernetes.Types.Inputs.Core.V1.ISCSIVolumeSourceArgs
            {
              ChapAuthDiscovery = true|false,
              ChapAuthSession = true|false,
              FsType = "string",
              InitiatorName = "string",
              Iqn = "string",
              IscsiInterface = "string",
              Lun = 0,
              Portals = new []
              {
                "string"
              },
              ReadOnly = true|false,
              SecretRef = new Kubernetes.Types.Inputs.Core.V1.LocalObjectReferenceArgs
              {
                Name = "string",
              },
              TargetPortal = "string",
            },
            Name = "string",
            Nfs = new Kubernetes.Types.Inputs.Core.V1.NFSVolumeSourceArgs
            {
              Path = "string",
              ReadOnly = true|false,
              Server = "string",
            },
            PersistentVolumeClaim = new Kubernetes.Types.Inputs.Core.V1.PersistentVolumeClaimVolumeSourceArgs
            {
              ClaimName = "string",
              ReadOnly = true|false,
            },
            PhotonPersistentDisk = new Kubernetes.Types.Inputs.Core.V1.PhotonPersistentDiskVolumeSourceArgs
            {
              FsType = "string",
              PdID = "string",
            },
            PortworxVolume = new Kubernetes.Types.Inputs.Core.V1.PortworxVolumeSourceArgs
            {
              FsType = "string",
              ReadOnly = true|false,
              VolumeID = "string",
            },
            Projected = new Kubernetes.Types.Inputs.Core.V1.ProjectedVolumeSourceArgs
            {
              DefaultMode = 0,
              Sources = new []
              {
                new Kubernetes.Types.Inputs.Core.V1.VolumeProjectionArgs
                {
                  ConfigMap = new Kubernetes.Types.Inputs.Core.V1.ConfigMapProjectionArgs
                  {
                    Items = new []
                    {
                      new Kubernetes.Types.Inputs.Core.V1.KeyToPathArgs
                      {
                        Key = "string",
                        Mode = 0,
                        Path = "string",
                      }
                    },
                    Name = "string",
                    Optional = true|false,
                  },
                  DownwardAPI = new Kubernetes.Types.Inputs.Core.V1.DownwardAPIProjectionArgs
                  {
                    Items = new []
                    {
                      new Kubernetes.Types.Inputs.Core.V1.DownwardAPIVolumeFileArgs
                      {
                        FieldRef = new Kubernetes.Types.Inputs.Core.V1.ObjectFieldSelectorArgs
                        {
                          ApiVersion = "string",
                          FieldPath = "string",
                        },
                        Mode = 0,
                        Path = "string",
                        ResourceFieldRef = new Kubernetes.Types.Inputs.Core.V1.ResourceFieldSelectorArgs
                        {
                          ContainerName = "string",
                          Divisor = "string",
                          Resource = "string",
                        },
                      }
                    },
                  },
                  Secret = new Kubernetes.Types.Inputs.Core.V1.SecretProjectionArgs
                  {
                    Items = new []
                    {
                      new Kubernetes.Types.Inputs.Core.V1.KeyToPathArgs
                      {
                        Key = "string",
                        Mode = 0,
                        Path = "string",
                      }
                    },
                    Name = "string",
                    Optional = true|false,
                  },
                  ServiceAccountToken = new Kubernetes.Types.Inputs.Core.V1.ServiceAccountTokenProjectionArgs
                  {
                    Audience = "string",
                    ExpirationSeconds = 0,
                    Path = "string",
                  },
                }
              },
            },
            Quobyte = new Kubernetes.Types.Inputs.Core.V1.QuobyteVolumeSourceArgs
            {
              Group = "string",
              ReadOnly = true|false,
              Registry = "string",
              Tenant = "string",
              User = "string",
              Volume = "string",
            },
            Rbd = new Kubernetes.Types.Inputs.Core.V1.RBDVolumeSourceArgs
            {
              FsType = "string",
              Image = "string",
              Keyring = "string",
              Monitors = new []
              {
                "string"
              },
              Pool = "string",
              ReadOnly = true|false,
              SecretRef = new Kubernetes.Types.Inputs.Core.V1.LocalObjectReferenceArgs
              {
                Name = "string",
              },
              User = "string",
            },
            ScaleIO = new Kubernetes.Types.Inputs.Core.V1.ScaleIOVolumeSourceArgs
            {
              FsType = "string",
              Gateway = "string",
              ProtectionDomain = "string",
              ReadOnly = true|false,
              SecretRef = new Kubernetes.Types.Inputs.Core.V1.LocalObjectReferenceArgs
              {
                Name = "string",
              },
              SslEnabled = true|false,
              StorageMode = "string",
              StoragePool = "string",
              System = "string",
              VolumeName = "string",
            },
            Secret = new Kubernetes.Types.Inputs.Core.V1.SecretVolumeSourceArgs
            {
              DefaultMode = 0,
              Items = new []
              {
                new Kubernetes.Types.Inputs.Core.V1.KeyToPathArgs
                {
                  Key = "string",
                  Mode = 0,
                  Path = "string",
                }
              },
              Optional = true|false,
              SecretName = "string",
            },
            Storageos = new Kubernetes.Types.Inputs.Core.V1.StorageOSVolumeSourceArgs
            {
              FsType = "string",
              ReadOnly = true|false,
              SecretRef = new Kubernetes.Types.Inputs.Core.V1.LocalObjectReferenceArgs
              {
                Name = "string",
              },
              VolumeName = "string",
              VolumeNamespace = "string",
            },
            VsphereVolume = new Kubernetes.Types.Inputs.Core.V1.VsphereVirtualDiskVolumeSourceArgs
            {
              FsType = "string",
              StoragePolicyID = "string",
              StoragePolicyName = "string",
              VolumePath = "string",
            },
          }
        },
      },
    },
  },
});
```

### Go

```go
import (
  "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
  "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apps/v1"
)

deployment, err := apps.v1.NewDeployment("deployment", &apps.v1.DeploymentArgs{
  ApiVersion: pulumi.String("string"),
  Kind: pulumi.String("string"),
  Metadata: &meta.v1.ObjectMetaArgs{
    Annotations: pulumi.StringMap{
      "string": pulumi.String("string")
    },
    ClusterName: pulumi.String("string"),
    CreationTimestamp: pulumi.String("string"),
    DeletionGracePeriodSeconds: pulumi.Int(0),
    DeletionTimestamp: pulumi.String("string"),
    Finalizers: pulumi.StringArray{
      pulumi.String("string")
    },
    GenerateName: pulumi.String("string"),
    Generation: pulumi.Int(0),
    Labels: pulumi.StringMap{
      "string": pulumi.String("string")
    },
    ManagedFields: meta.v1.ManagedFieldsEntryArray{
      &meta.v1.ManagedFieldsEntryArgs{
        ApiVersion: pulumi.String("string"),
        FieldsType: pulumi.String("string"),
        FieldsV1: ,
        Manager: pulumi.String("string"),
        Operation: pulumi.String("string"),
        Subresource: pulumi.String("string"),
        Time: pulumi.String("string"),
      }
    },
    Name: pulumi.String("string"),
    Namespace: pulumi.String("string"),
    OwnerReferences: meta.v1.OwnerReferenceArray{
      &meta.v1.OwnerReferenceArgs{
        ApiVersion: pulumi.String("string"),
        BlockOwnerDeletion: pulumi.Bool(true|false),
        Controller: pulumi.Bool(true|false),
        Kind: pulumi.String("string"),
        Name: pulumi.String("string"),
        Uid: pulumi.String("string"),
      }
    },
    ResourceVersion: pulumi.String("string"),
    SelfLink: pulumi.String("string"),
    Uid: pulumi.String("string"),
  },
  Spec: &apps.v1.DeploymentSpecArgs{
    MinReadySeconds: pulumi.Int(0),
    Paused: pulumi.Bool(true|false),
    ProgressDeadlineSeconds: pulumi.Int(0),
    Replicas: pulumi.Int(0),
    RevisionHistoryLimit: pulumi.Int(0),
    Selector: &meta.v1.LabelSelectorArgs{
      MatchExpressions: meta.v1.LabelSelectorRequirementArray{
        &meta.v1.LabelSelectorRequirementArgs{
          Key: pulumi.String("string"),
          Operator: pulumi.String("string"),
          Values: pulumi.StringArray{
            pulumi.String("string")
          },
        }
      },
      MatchLabels: pulumi.StringMap{
        "string": pulumi.String("string")
      },
    },
    Strategy: &apps.v1.DeploymentStrategyArgs{
      RollingUpdate: &apps.v1.RollingUpdateDeploymentArgs{
        MaxSurge: pulumi.Int(0),
        MaxUnavailable: pulumi.Int(0),
      },
      Type: pulumi.String("string"),
    },
    Template: &core.v1.PodTemplateSpecArgs{
      Metadata: &meta.v1.ObjectMetaArgs{
        Annotations: pulumi.StringMap{
          "string": pulumi.String("string")
        },
        ClusterName: pulumi.String("string"),
        CreationTimestamp: pulumi.String("string"),
        DeletionGracePeriodSeconds: pulumi.Int(0),
        DeletionTimestamp: pulumi.String("string"),
        Finalizers: pulumi.StringArray{
          pulumi.String("string")
        },
        GenerateName: pulumi.String("string"),
        Generation: pulumi.Int(0),
        Labels: pulumi.StringMap{
          "string": pulumi.String("string")
        },
        ManagedFields: meta.v1.ManagedFieldsEntryArray{
          &meta.v1.ManagedFieldsEntryArgs{
            ApiVersion: pulumi.String("string"),
            FieldsType: pulumi.String("string"),
            FieldsV1: ,
            Manager: pulumi.String("string"),
            Operation: pulumi.String("string"),
            Subresource: pulumi.String("string"),
            Time: pulumi.String("string"),
          }
        },
        Name: pulumi.String("string"),
        Namespace: pulumi.String("string"),
        OwnerReferences: meta.v1.OwnerReferenceArray{
          &meta.v1.OwnerReferenceArgs{
            ApiVersion: pulumi.String("string"),
            BlockOwnerDeletion: pulumi.Bool(true|false),
            Controller: pulumi.Bool(true|false),
            Kind: pulumi.String("string"),
            Name: pulumi.String("string"),
            Uid: pulumi.String("string"),
          }
        },
        ResourceVersion: pulumi.String("string"),
        SelfLink: pulumi.String("string"),
        Uid: pulumi.String("string"),
      },
      Spec: &core.v1.PodSpecArgs{
        ActiveDeadlineSeconds: pulumi.Int(0),
        Affinity: &core.v1.AffinityArgs{
          NodeAffinity: &core.v1.NodeAffinityArgs{
            PreferredDuringSchedulingIgnoredDuringExecution: core.v1.PreferredSchedulingTermArray{
              &core.v1.PreferredSchedulingTermArgs{
                Preference: &core.v1.NodeSelectorTermArgs{
                  MatchExpressions: core.v1.NodeSelectorRequirementArray{
                    &core.v1.NodeSelectorRequirementArgs{
                      Key: pulumi.String("string"),
                      Operator: pulumi.String("string"),
                      Values: pulumi.StringArray{
                        pulumi.String("string")
                      },
                    }
                  },
                  MatchFields: core.v1.NodeSelectorRequirementArray{
                    &core.v1.NodeSelectorRequirementArgs{
                      Key: pulumi.String("string"),
                      Operator: pulumi.String("string"),
                      Values: pulumi.StringArray{
                        pulumi.String("string")
                      },
                    }
                  },
                },
                Weight: pulumi.Int(0),
              }
            },
            RequiredDuringSchedulingIgnoredDuringExecution: &core.v1.NodeSelectorArgs{
              NodeSelectorTerms: core.v1.NodeSelectorTermArray{
                &core.v1.NodeSelectorTermArgs{
                  MatchExpressions: core.v1.NodeSelectorRequirementArray{
                    &core.v1.NodeSelectorRequirementArgs{
                      Key: pulumi.String("string"),
                      Operator: pulumi.String("string"),
                      Values: pulumi.StringArray{
                        pulumi.String("string")
                      },
                    }
                  },
                  MatchFields: core.v1.NodeSelectorRequirementArray{
                    &core.v1.NodeSelectorRequirementArgs{
                      Key: pulumi.String("string"),
                      Operator: pulumi.String("string"),
                      Values: pulumi.StringArray{
                        pulumi.String("string")
                      },
                    }
                  },
                }
              },
            },
          },
          PodAffinity: &core.v1.PodAffinityArgs{
            PreferredDuringSchedulingIgnoredDuringExecution: core.v1.WeightedPodAffinityTermArray{
              &core.v1.WeightedPodAffinityTermArgs{
                PodAffinityTerm: &core.v1.PodAffinityTermArgs{
                  LabelSelector: &meta.v1.LabelSelectorArgs{
                    MatchExpressions: meta.v1.LabelSelectorRequirementArray{
                      &meta.v1.LabelSelectorRequirementArgs{
                        Key: pulumi.String("string"),
                        Operator: pulumi.String("string"),
                        Values: pulumi.StringArray{
                          pulumi.String("string")
                        },
                      }
                    },
                    MatchLabels: pulumi.StringMap{
                      "string": pulumi.String("string")
                    },
                  },
                  NamespaceSelector: &meta.v1.LabelSelectorArgs{
                    MatchExpressions: meta.v1.LabelSelectorRequirementArray{
                      &meta.v1.LabelSelectorRequirementArgs{
                        Key: pulumi.String("string"),
                        Operator: pulumi.String("string"),
                        Values: pulumi.StringArray{
                          pulumi.String("string")
                        },
                      }
                    },
                    MatchLabels: pulumi.StringMap{
                      "string": pulumi.String("string")
                    },
                  },
                  Namespaces: pulumi.StringArray{
                    pulumi.String("string")
                  },
                  TopologyKey: pulumi.String("string"),
                },
                Weight: pulumi.Int(0),
              }
            },
            RequiredDuringSchedulingIgnoredDuringExecution: core.v1.PodAffinityTermArray{
              &core.v1.PodAffinityTermArgs{
                LabelSelector: &meta.v1.LabelSelectorArgs{
                  MatchExpressions: meta.v1.LabelSelectorRequirementArray{
                    &meta.v1.LabelSelectorRequirementArgs{
                      Key: pulumi.String("string"),
                      Operator: pulumi.String("string"),
                      Values: pulumi.StringArray{
                        pulumi.String("string")
                      },
                    }
                  },
                  MatchLabels: pulumi.StringMap{
                    "string": pulumi.String("string")
                  },
                },
                NamespaceSelector: &meta.v1.LabelSelectorArgs{
                  MatchExpressions: meta.v1.LabelSelectorRequirementArray{
                    &meta.v1.LabelSelectorRequirementArgs{
                      Key: pulumi.String("string"),
                      Operator: pulumi.String("string"),
                      Values: pulumi.StringArray{
                        pulumi.String("string")
                      },
                    }
                  },
                  MatchLabels: pulumi.StringMap{
                    "string": pulumi.String("string")
                  },
                },
                Namespaces: pulumi.StringArray{
                  pulumi.String("string")
                },
                TopologyKey: pulumi.String("string"),
              }
            },
          },
          PodAntiAffinity: &core.v1.PodAntiAffinityArgs{
            PreferredDuringSchedulingIgnoredDuringExecution: core.v1.WeightedPodAffinityTermArray{
              &core.v1.WeightedPodAffinityTermArgs{
                PodAffinityTerm: &core.v1.PodAffinityTermArgs{
                  LabelSelector: &meta.v1.LabelSelectorArgs{
                    MatchExpressions: meta.v1.LabelSelectorRequirementArray{
                      &meta.v1.LabelSelectorRequirementArgs{
                        Key: pulumi.String("string"),
                        Operator: pulumi.String("string"),
                        Values: pulumi.StringArray{
                          pulumi.String("string")
                        },
                      }
                    },
                    MatchLabels: pulumi.StringMap{
                      "string": pulumi.String("string")
                    },
                  },
                  NamespaceSelector: &meta.v1.LabelSelectorArgs{
                    MatchExpressions: meta.v1.LabelSelectorRequirementArray{
                      &meta.v1.LabelSelectorRequirementArgs{
                        Key: pulumi.String("string"),
                        Operator: pulumi.String("string"),
                        Values: pulumi.StringArray{
                          pulumi.String("string")
                        },
                      }
                    },
                    MatchLabels: pulumi.StringMap{
                      "string": pulumi.String("string")
                    },
                  },
                  Namespaces: pulumi.StringArray{
                    pulumi.String("string")
                  },
                  TopologyKey: pulumi.String("string"),
                },
                Weight: pulumi.Int(0),
              }
            },
            RequiredDuringSchedulingIgnoredDuringExecution: core.v1.PodAffinityTermArray{
              &core.v1.PodAffinityTermArgs{
                LabelSelector: &meta.v1.LabelSelectorArgs{
                  MatchExpressions: meta.v1.LabelSelectorRequirementArray{
                    &meta.v1.LabelSelectorRequirementArgs{
                      Key: pulumi.String("string"),
                      Operator: pulumi.String("string"),
                      Values: pulumi.StringArray{
                        pulumi.String("string")
                      },
                    }
                  },
                  MatchLabels: pulumi.StringMap{
                    "string": pulumi.String("string")
                  },
                },
                NamespaceSelector: &meta.v1.LabelSelectorArgs{
                  MatchExpressions: meta.v1.LabelSelectorRequirementArray{
                    &meta.v1.LabelSelectorRequirementArgs{
                      Key: pulumi.String("string"),
                      Operator: pulumi.String("string"),
                      Values: pulumi.StringArray{
                        pulumi.String("string")
                      },
                    }
                  },
                  MatchLabels: pulumi.StringMap{
                    "string": pulumi.String("string")
                  },
                },
                Namespaces: pulumi.StringArray{
                  pulumi.String("string")
                },
                TopologyKey: pulumi.String("string"),
              }
            },
          },
        },
        AutomountServiceAccountToken: pulumi.Bool(true|false),
        Containers: core.v1.ContainerArray{
          &core.v1.ContainerArgs{
            Args: pulumi.StringArray{
              pulumi.String("string")
            },
            Command: pulumi.StringArray{
              pulumi.String("string")
            },
            Env: core.v1.EnvVarArray{
              &core.v1.EnvVarArgs{
                Name: pulumi.String("string"),
                Value: pulumi.String("string"),
                ValueFrom: &core.v1.EnvVarSourceArgs{
                  ConfigMapKeyRef: &core.v1.ConfigMapKeySelectorArgs{
                    Key: pulumi.String("string"),
                    Name: pulumi.String("string"),
                    Optional: pulumi.Bool(true|false),
                  },
                  FieldRef: &core.v1.ObjectFieldSelectorArgs{
                    ApiVersion: pulumi.String("string"),
                    FieldPath: pulumi.String("string"),
                  },
                  ResourceFieldRef: &core.v1.ResourceFieldSelectorArgs{
                    ContainerName: pulumi.String("string"),
                    Divisor: pulumi.String("string"),
                    Resource: pulumi.String("string"),
                  },
                  SecretKeyRef: &core.v1.SecretKeySelectorArgs{
                    Key: pulumi.String("string"),
                    Name: pulumi.String("string"),
                    Optional: pulumi.Bool(true|false),
                  },
                },
              }
            },
            EnvFrom: core.v1.EnvFromSourceArray{
              &core.v1.EnvFromSourceArgs{
                ConfigMapRef: &core.v1.ConfigMapEnvSourceArgs{
                  Name: pulumi.String("string"),
                  Optional: pulumi.Bool(true|false),
                },
                Prefix: pulumi.String("string"),
                SecretRef: &core.v1.SecretEnvSourceArgs{
                  Name: pulumi.String("string"),
                  Optional: pulumi.Bool(true|false),
                },
              }
            },
            Image: pulumi.String("string"),
            ImagePullPolicy: pulumi.String("string"),
            Lifecycle: &core.v1.LifecycleArgs{
              PostStart: &core.v1.HandlerArgs{
                Exec: &core.v1.ExecActionArgs{
                  Command: pulumi.StringArray{
                    pulumi.String("string")
                  },
                },
                HttpGet: &core.v1.HTTPGetActionArgs{
                  Host: pulumi.String("string"),
                  HttpHeaders: core.v1.HTTPHeaderArray{
                    &core.v1.HTTPHeaderArgs{
                      Name: pulumi.String("string"),
                      Value: pulumi.String("string"),
                    }
                  },
                  Path: pulumi.String("string"),
                  Port: pulumi.Int(0),
                  Scheme: pulumi.String("string"),
                },
                TcpSocket: &core.v1.TCPSocketActionArgs{
                  Host: pulumi.String("string"),
                  Port: pulumi.Int(0),
                },
              },
              PreStop: &core.v1.HandlerArgs{
                Exec: &core.v1.ExecActionArgs{
                  Command: pulumi.StringArray{
                    pulumi.String("string")
                  },
                },
                HttpGet: &core.v1.HTTPGetActionArgs{
                  Host: pulumi.String("string"),
                  HttpHeaders: core.v1.HTTPHeaderArray{
                    &core.v1.HTTPHeaderArgs{
                      Name: pulumi.String("string"),
                      Value: pulumi.String("string"),
                    }
                  },
                  Path: pulumi.String("string"),
                  Port: pulumi.Int(0),
                  Scheme: pulumi.String("string"),
                },
                TcpSocket: &core.v1.TCPSocketActionArgs{
                  Host: pulumi.String("string"),
                  Port: pulumi.Int(0),
                },
              },
            },
            LivenessProbe: &core.v1.ProbeArgs{
              Exec: &core.v1.ExecActionArgs{
                Command: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              FailureThreshold: pulumi.Int(0),
              HttpGet: &core.v1.HTTPGetActionArgs{
                Host: pulumi.String("string"),
                HttpHeaders: core.v1.HTTPHeaderArray{
                  &core.v1.HTTPHeaderArgs{
                    Name: pulumi.String("string"),
                    Value: pulumi.String("string"),
                  }
                },
                Path: pulumi.String("string"),
                Port: pulumi.Int(0),
                Scheme: pulumi.String("string"),
              },
              InitialDelaySeconds: pulumi.Int(0),
              PeriodSeconds: pulumi.Int(0),
              SuccessThreshold: pulumi.Int(0),
              TcpSocket: &core.v1.TCPSocketActionArgs{
                Host: pulumi.String("string"),
                Port: pulumi.Int(0),
              },
              TerminationGracePeriodSeconds: pulumi.Int(0),
              TimeoutSeconds: pulumi.Int(0),
            },
            Name: pulumi.String("string"),
            Ports: core.v1.ContainerPortArray{
              &core.v1.ContainerPortArgs{
                ContainerPort: pulumi.Int(0),
                HostIP: pulumi.String("string"),
                HostPort: pulumi.Int(0),
                Name: pulumi.String("string"),
                Protocol: pulumi.String("string"),
              }
            },
            ReadinessProbe: &core.v1.ProbeArgs{
              Exec: &core.v1.ExecActionArgs{
                Command: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              FailureThreshold: pulumi.Int(0),
              HttpGet: &core.v1.HTTPGetActionArgs{
                Host: pulumi.String("string"),
                HttpHeaders: core.v1.HTTPHeaderArray{
                  &core.v1.HTTPHeaderArgs{
                    Name: pulumi.String("string"),
                    Value: pulumi.String("string"),
                  }
                },
                Path: pulumi.String("string"),
                Port: pulumi.Int(0),
                Scheme: pulumi.String("string"),
              },
              InitialDelaySeconds: pulumi.Int(0),
              PeriodSeconds: pulumi.Int(0),
              SuccessThreshold: pulumi.Int(0),
              TcpSocket: &core.v1.TCPSocketActionArgs{
                Host: pulumi.String("string"),
                Port: pulumi.Int(0),
              },
              TerminationGracePeriodSeconds: pulumi.Int(0),
              TimeoutSeconds: pulumi.Int(0),
            },
            Resources: &core.v1.ResourceRequirementsArgs{
              Limits: pulumi.StringMap{
                "string": pulumi.String("string")
              },
              Requests: pulumi.StringMap{
                "string": pulumi.String("string")
              },
            },
            SecurityContext: &core.v1.SecurityContextArgs{
              AllowPrivilegeEscalation: pulumi.Bool(true|false),
              Capabilities: &core.v1.CapabilitiesArgs{
                Add: pulumi.StringArray{
                  pulumi.String("string")
                },
                Drop: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              Privileged: pulumi.Bool(true|false),
              ProcMount: pulumi.String("string"),
              ReadOnlyRootFilesystem: pulumi.Bool(true|false),
              RunAsGroup: pulumi.Int(0),
              RunAsNonRoot: pulumi.Bool(true|false),
              RunAsUser: pulumi.Int(0),
              SeLinuxOptions: &core.v1.SELinuxOptionsArgs{
                Level: pulumi.String("string"),
                Role: pulumi.String("string"),
                Type: pulumi.String("string"),
                User: pulumi.String("string"),
              },
              SeccompProfile: &core.v1.SeccompProfileArgs{
                LocalhostProfile: pulumi.String("string"),
                Type: pulumi.String("string"),
              },
              WindowsOptions: &core.v1.WindowsSecurityContextOptionsArgs{
                GmsaCredentialSpec: pulumi.String("string"),
                GmsaCredentialSpecName: pulumi.String("string"),
                HostProcess: pulumi.Bool(true|false),
                RunAsUserName: pulumi.String("string"),
              },
            },
            StartupProbe: &core.v1.ProbeArgs{
              Exec: &core.v1.ExecActionArgs{
                Command: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              FailureThreshold: pulumi.Int(0),
              HttpGet: &core.v1.HTTPGetActionArgs{
                Host: pulumi.String("string"),
                HttpHeaders: core.v1.HTTPHeaderArray{
                  &core.v1.HTTPHeaderArgs{
                    Name: pulumi.String("string"),
                    Value: pulumi.String("string"),
                  }
                },
                Path: pulumi.String("string"),
                Port: pulumi.Int(0),
                Scheme: pulumi.String("string"),
              },
              InitialDelaySeconds: pulumi.Int(0),
              PeriodSeconds: pulumi.Int(0),
              SuccessThreshold: pulumi.Int(0),
              TcpSocket: &core.v1.TCPSocketActionArgs{
                Host: pulumi.String("string"),
                Port: pulumi.Int(0),
              },
              TerminationGracePeriodSeconds: pulumi.Int(0),
              TimeoutSeconds: pulumi.Int(0),
            },
            Stdin: pulumi.Bool(true|false),
            StdinOnce: pulumi.Bool(true|false),
            TerminationMessagePath: pulumi.String("string"),
            TerminationMessagePolicy: pulumi.String("string"),
            Tty: pulumi.Bool(true|false),
            VolumeDevices: core.v1.VolumeDeviceArray{
              &core.v1.VolumeDeviceArgs{
                DevicePath: pulumi.String("string"),
                Name: pulumi.String("string"),
              }
            },
            VolumeMounts: core.v1.VolumeMountArray{
              &core.v1.VolumeMountArgs{
                MountPath: pulumi.String("string"),
                MountPropagation: pulumi.String("string"),
                Name: pulumi.String("string"),
                ReadOnly: pulumi.Bool(true|false),
                SubPath: pulumi.String("string"),
                SubPathExpr: pulumi.String("string"),
              }
            },
            WorkingDir: pulumi.String("string"),
          }
        },
        DnsConfig: &core.v1.PodDNSConfigArgs{
          Nameservers: pulumi.StringArray{
            pulumi.String("string")
          },
          Options: core.v1.PodDNSConfigOptionArray{
            &core.v1.PodDNSConfigOptionArgs{
              Name: pulumi.String("string"),
              Value: pulumi.String("string"),
            }
          },
          Searches: pulumi.StringArray{
            pulumi.String("string")
          },
        },
        DnsPolicy: pulumi.String("string"),
        EnableServiceLinks: pulumi.Bool(true|false),
        EphemeralContainers: core.v1.EphemeralContainerArray{
          &core.v1.EphemeralContainerArgs{
            Args: pulumi.StringArray{
              pulumi.String("string")
            },
            Command: pulumi.StringArray{
              pulumi.String("string")
            },
            Env: core.v1.EnvVarArray{
              &core.v1.EnvVarArgs{
                Name: pulumi.String("string"),
                Value: pulumi.String("string"),
                ValueFrom: &core.v1.EnvVarSourceArgs{
                  ConfigMapKeyRef: &core.v1.ConfigMapKeySelectorArgs{
                    Key: pulumi.String("string"),
                    Name: pulumi.String("string"),
                    Optional: pulumi.Bool(true|false),
                  },
                  FieldRef: &core.v1.ObjectFieldSelectorArgs{
                    ApiVersion: pulumi.String("string"),
                    FieldPath: pulumi.String("string"),
                  },
                  ResourceFieldRef: &core.v1.ResourceFieldSelectorArgs{
                    ContainerName: pulumi.String("string"),
                    Divisor: pulumi.String("string"),
                    Resource: pulumi.String("string"),
                  },
                  SecretKeyRef: &core.v1.SecretKeySelectorArgs{
                    Key: pulumi.String("string"),
                    Name: pulumi.String("string"),
                    Optional: pulumi.Bool(true|false),
                  },
                },
              }
            },
            EnvFrom: core.v1.EnvFromSourceArray{
              &core.v1.EnvFromSourceArgs{
                ConfigMapRef: &core.v1.ConfigMapEnvSourceArgs{
                  Name: pulumi.String("string"),
                  Optional: pulumi.Bool(true|false),
                },
                Prefix: pulumi.String("string"),
                SecretRef: &core.v1.SecretEnvSourceArgs{
                  Name: pulumi.String("string"),
                  Optional: pulumi.Bool(true|false),
                },
              }
            },
            Image: pulumi.String("string"),
            ImagePullPolicy: pulumi.String("string"),
            Lifecycle: &core.v1.LifecycleArgs{
              PostStart: &core.v1.HandlerArgs{
                Exec: &core.v1.ExecActionArgs{
                  Command: pulumi.StringArray{
                    pulumi.String("string")
                  },
                },
                HttpGet: &core.v1.HTTPGetActionArgs{
                  Host: pulumi.String("string"),
                  HttpHeaders: core.v1.HTTPHeaderArray{
                    &core.v1.HTTPHeaderArgs{
                      Name: pulumi.String("string"),
                      Value: pulumi.String("string"),
                    }
                  },
                  Path: pulumi.String("string"),
                  Port: pulumi.Int(0),
                  Scheme: pulumi.String("string"),
                },
                TcpSocket: &core.v1.TCPSocketActionArgs{
                  Host: pulumi.String("string"),
                  Port: pulumi.Int(0),
                },
              },
              PreStop: &core.v1.HandlerArgs{
                Exec: &core.v1.ExecActionArgs{
                  Command: pulumi.StringArray{
                    pulumi.String("string")
                  },
                },
                HttpGet: &core.v1.HTTPGetActionArgs{
                  Host: pulumi.String("string"),
                  HttpHeaders: core.v1.HTTPHeaderArray{
                    &core.v1.HTTPHeaderArgs{
                      Name: pulumi.String("string"),
                      Value: pulumi.String("string"),
                    }
                  },
                  Path: pulumi.String("string"),
                  Port: pulumi.Int(0),
                  Scheme: pulumi.String("string"),
                },
                TcpSocket: &core.v1.TCPSocketActionArgs{
                  Host: pulumi.String("string"),
                  Port: pulumi.Int(0),
                },
              },
            },
            LivenessProbe: &core.v1.ProbeArgs{
              Exec: &core.v1.ExecActionArgs{
                Command: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              FailureThreshold: pulumi.Int(0),
              HttpGet: &core.v1.HTTPGetActionArgs{
                Host: pulumi.String("string"),
                HttpHeaders: core.v1.HTTPHeaderArray{
                  &core.v1.HTTPHeaderArgs{
                    Name: pulumi.String("string"),
                    Value: pulumi.String("string"),
                  }
                },
                Path: pulumi.String("string"),
                Port: pulumi.Int(0),
                Scheme: pulumi.String("string"),
              },
              InitialDelaySeconds: pulumi.Int(0),
              PeriodSeconds: pulumi.Int(0),
              SuccessThreshold: pulumi.Int(0),
              TcpSocket: &core.v1.TCPSocketActionArgs{
                Host: pulumi.String("string"),
                Port: pulumi.Int(0),
              },
              TerminationGracePeriodSeconds: pulumi.Int(0),
              TimeoutSeconds: pulumi.Int(0),
            },
            Name: pulumi.String("string"),
            Ports: core.v1.ContainerPortArray{
              &core.v1.ContainerPortArgs{
                ContainerPort: pulumi.Int(0),
                HostIP: pulumi.String("string"),
                HostPort: pulumi.Int(0),
                Name: pulumi.String("string"),
                Protocol: pulumi.String("string"),
              }
            },
            ReadinessProbe: &core.v1.ProbeArgs{
              Exec: &core.v1.ExecActionArgs{
                Command: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              FailureThreshold: pulumi.Int(0),
              HttpGet: &core.v1.HTTPGetActionArgs{
                Host: pulumi.String("string"),
                HttpHeaders: core.v1.HTTPHeaderArray{
                  &core.v1.HTTPHeaderArgs{
                    Name: pulumi.String("string"),
                    Value: pulumi.String("string"),
                  }
                },
                Path: pulumi.String("string"),
                Port: pulumi.Int(0),
                Scheme: pulumi.String("string"),
              },
              InitialDelaySeconds: pulumi.Int(0),
              PeriodSeconds: pulumi.Int(0),
              SuccessThreshold: pulumi.Int(0),
              TcpSocket: &core.v1.TCPSocketActionArgs{
                Host: pulumi.String("string"),
                Port: pulumi.Int(0),
              },
              TerminationGracePeriodSeconds: pulumi.Int(0),
              TimeoutSeconds: pulumi.Int(0),
            },
            Resources: &core.v1.ResourceRequirementsArgs{
              Limits: pulumi.StringMap{
                "string": pulumi.String("string")
              },
              Requests: pulumi.StringMap{
                "string": pulumi.String("string")
              },
            },
            SecurityContext: &core.v1.SecurityContextArgs{
              AllowPrivilegeEscalation: pulumi.Bool(true|false),
              Capabilities: &core.v1.CapabilitiesArgs{
                Add: pulumi.StringArray{
                  pulumi.String("string")
                },
                Drop: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              Privileged: pulumi.Bool(true|false),
              ProcMount: pulumi.String("string"),
              ReadOnlyRootFilesystem: pulumi.Bool(true|false),
              RunAsGroup: pulumi.Int(0),
              RunAsNonRoot: pulumi.Bool(true|false),
              RunAsUser: pulumi.Int(0),
              SeLinuxOptions: &core.v1.SELinuxOptionsArgs{
                Level: pulumi.String("string"),
                Role: pulumi.String("string"),
                Type: pulumi.String("string"),
                User: pulumi.String("string"),
              },
              SeccompProfile: &core.v1.SeccompProfileArgs{
                LocalhostProfile: pulumi.String("string"),
                Type: pulumi.String("string"),
              },
              WindowsOptions: &core.v1.WindowsSecurityContextOptionsArgs{
                GmsaCredentialSpec: pulumi.String("string"),
                GmsaCredentialSpecName: pulumi.String("string"),
                HostProcess: pulumi.Bool(true|false),
                RunAsUserName: pulumi.String("string"),
              },
            },
            StartupProbe: &core.v1.ProbeArgs{
              Exec: &core.v1.ExecActionArgs{
                Command: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              FailureThreshold: pulumi.Int(0),
              HttpGet: &core.v1.HTTPGetActionArgs{
                Host: pulumi.String("string"),
                HttpHeaders: core.v1.HTTPHeaderArray{
                  &core.v1.HTTPHeaderArgs{
                    Name: pulumi.String("string"),
                    Value: pulumi.String("string"),
                  }
                },
                Path: pulumi.String("string"),
                Port: pulumi.Int(0),
                Scheme: pulumi.String("string"),
              },
              InitialDelaySeconds: pulumi.Int(0),
              PeriodSeconds: pulumi.Int(0),
              SuccessThreshold: pulumi.Int(0),
              TcpSocket: &core.v1.TCPSocketActionArgs{
                Host: pulumi.String("string"),
                Port: pulumi.Int(0),
              },
              TerminationGracePeriodSeconds: pulumi.Int(0),
              TimeoutSeconds: pulumi.Int(0),
            },
            Stdin: pulumi.Bool(true|false),
            StdinOnce: pulumi.Bool(true|false),
            TargetContainerName: pulumi.String("string"),
            TerminationMessagePath: pulumi.String("string"),
            TerminationMessagePolicy: pulumi.String("string"),
            Tty: pulumi.Bool(true|false),
            VolumeDevices: core.v1.VolumeDeviceArray{
              &core.v1.VolumeDeviceArgs{
                DevicePath: pulumi.String("string"),
                Name: pulumi.String("string"),
              }
            },
            VolumeMounts: core.v1.VolumeMountArray{
              &core.v1.VolumeMountArgs{
                MountPath: pulumi.String("string"),
                MountPropagation: pulumi.String("string"),
                Name: pulumi.String("string"),
                ReadOnly: pulumi.Bool(true|false),
                SubPath: pulumi.String("string"),
                SubPathExpr: pulumi.String("string"),
              }
            },
            WorkingDir: pulumi.String("string"),
          }
        },
        HostAliases: core.v1.HostAliasArray{
          &core.v1.HostAliasArgs{
            Hostnames: pulumi.StringArray{
              pulumi.String("string")
            },
            Ip: pulumi.String("string"),
          }
        },
        HostIPC: pulumi.Bool(true|false),
        HostNetwork: pulumi.Bool(true|false),
        HostPID: pulumi.Bool(true|false),
        Hostname: pulumi.String("string"),
        ImagePullSecrets: core.v1.LocalObjectReferenceArray{
          &core.v1.LocalObjectReferenceArgs{
            Name: pulumi.String("string"),
          }
        },
        InitContainers: core.v1.ContainerArray{
          &core.v1.ContainerArgs{
            Args: pulumi.StringArray{
              pulumi.String("string")
            },
            Command: pulumi.StringArray{
              pulumi.String("string")
            },
            Env: core.v1.EnvVarArray{
              &core.v1.EnvVarArgs{
                Name: pulumi.String("string"),
                Value: pulumi.String("string"),
                ValueFrom: &core.v1.EnvVarSourceArgs{
                  ConfigMapKeyRef: &core.v1.ConfigMapKeySelectorArgs{
                    Key: pulumi.String("string"),
                    Name: pulumi.String("string"),
                    Optional: pulumi.Bool(true|false),
                  },
                  FieldRef: &core.v1.ObjectFieldSelectorArgs{
                    ApiVersion: pulumi.String("string"),
                    FieldPath: pulumi.String("string"),
                  },
                  ResourceFieldRef: &core.v1.ResourceFieldSelectorArgs{
                    ContainerName: pulumi.String("string"),
                    Divisor: pulumi.String("string"),
                    Resource: pulumi.String("string"),
                  },
                  SecretKeyRef: &core.v1.SecretKeySelectorArgs{
                    Key: pulumi.String("string"),
                    Name: pulumi.String("string"),
                    Optional: pulumi.Bool(true|false),
                  },
                },
              }
            },
            EnvFrom: core.v1.EnvFromSourceArray{
              &core.v1.EnvFromSourceArgs{
                ConfigMapRef: &core.v1.ConfigMapEnvSourceArgs{
                  Name: pulumi.String("string"),
                  Optional: pulumi.Bool(true|false),
                },
                Prefix: pulumi.String("string"),
                SecretRef: &core.v1.SecretEnvSourceArgs{
                  Name: pulumi.String("string"),
                  Optional: pulumi.Bool(true|false),
                },
              }
            },
            Image: pulumi.String("string"),
            ImagePullPolicy: pulumi.String("string"),
            Lifecycle: &core.v1.LifecycleArgs{
              PostStart: &core.v1.HandlerArgs{
                Exec: &core.v1.ExecActionArgs{
                  Command: pulumi.StringArray{
                    pulumi.String("string")
                  },
                },
                HttpGet: &core.v1.HTTPGetActionArgs{
                  Host: pulumi.String("string"),
                  HttpHeaders: core.v1.HTTPHeaderArray{
                    &core.v1.HTTPHeaderArgs{
                      Name: pulumi.String("string"),
                      Value: pulumi.String("string"),
                    }
                  },
                  Path: pulumi.String("string"),
                  Port: pulumi.Int(0),
                  Scheme: pulumi.String("string"),
                },
                TcpSocket: &core.v1.TCPSocketActionArgs{
                  Host: pulumi.String("string"),
                  Port: pulumi.Int(0),
                },
              },
              PreStop: &core.v1.HandlerArgs{
                Exec: &core.v1.ExecActionArgs{
                  Command: pulumi.StringArray{
                    pulumi.String("string")
                  },
                },
                HttpGet: &core.v1.HTTPGetActionArgs{
                  Host: pulumi.String("string"),
                  HttpHeaders: core.v1.HTTPHeaderArray{
                    &core.v1.HTTPHeaderArgs{
                      Name: pulumi.String("string"),
                      Value: pulumi.String("string"),
                    }
                  },
                  Path: pulumi.String("string"),
                  Port: pulumi.Int(0),
                  Scheme: pulumi.String("string"),
                },
                TcpSocket: &core.v1.TCPSocketActionArgs{
                  Host: pulumi.String("string"),
                  Port: pulumi.Int(0),
                },
              },
            },
            LivenessProbe: &core.v1.ProbeArgs{
              Exec: &core.v1.ExecActionArgs{
                Command: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              FailureThreshold: pulumi.Int(0),
              HttpGet: &core.v1.HTTPGetActionArgs{
                Host: pulumi.String("string"),
                HttpHeaders: core.v1.HTTPHeaderArray{
                  &core.v1.HTTPHeaderArgs{
                    Name: pulumi.String("string"),
                    Value: pulumi.String("string"),
                  }
                },
                Path: pulumi.String("string"),
                Port: pulumi.Int(0),
                Scheme: pulumi.String("string"),
              },
              InitialDelaySeconds: pulumi.Int(0),
              PeriodSeconds: pulumi.Int(0),
              SuccessThreshold: pulumi.Int(0),
              TcpSocket: &core.v1.TCPSocketActionArgs{
                Host: pulumi.String("string"),
                Port: pulumi.Int(0),
              },
              TerminationGracePeriodSeconds: pulumi.Int(0),
              TimeoutSeconds: pulumi.Int(0),
            },
            Name: pulumi.String("string"),
            Ports: core.v1.ContainerPortArray{
              &core.v1.ContainerPortArgs{
                ContainerPort: pulumi.Int(0),
                HostIP: pulumi.String("string"),
                HostPort: pulumi.Int(0),
                Name: pulumi.String("string"),
                Protocol: pulumi.String("string"),
              }
            },
            ReadinessProbe: &core.v1.ProbeArgs{
              Exec: &core.v1.ExecActionArgs{
                Command: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              FailureThreshold: pulumi.Int(0),
              HttpGet: &core.v1.HTTPGetActionArgs{
                Host: pulumi.String("string"),
                HttpHeaders: core.v1.HTTPHeaderArray{
                  &core.v1.HTTPHeaderArgs{
                    Name: pulumi.String("string"),
                    Value: pulumi.String("string"),
                  }
                },
                Path: pulumi.String("string"),
                Port: pulumi.Int(0),
                Scheme: pulumi.String("string"),
              },
              InitialDelaySeconds: pulumi.Int(0),
              PeriodSeconds: pulumi.Int(0),
              SuccessThreshold: pulumi.Int(0),
              TcpSocket: &core.v1.TCPSocketActionArgs{
                Host: pulumi.String("string"),
                Port: pulumi.Int(0),
              },
              TerminationGracePeriodSeconds: pulumi.Int(0),
              TimeoutSeconds: pulumi.Int(0),
            },
            Resources: &core.v1.ResourceRequirementsArgs{
              Limits: pulumi.StringMap{
                "string": pulumi.String("string")
              },
              Requests: pulumi.StringMap{
                "string": pulumi.String("string")
              },
            },
            SecurityContext: &core.v1.SecurityContextArgs{
              AllowPrivilegeEscalation: pulumi.Bool(true|false),
              Capabilities: &core.v1.CapabilitiesArgs{
                Add: pulumi.StringArray{
                  pulumi.String("string")
                },
                Drop: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              Privileged: pulumi.Bool(true|false),
              ProcMount: pulumi.String("string"),
              ReadOnlyRootFilesystem: pulumi.Bool(true|false),
              RunAsGroup: pulumi.Int(0),
              RunAsNonRoot: pulumi.Bool(true|false),
              RunAsUser: pulumi.Int(0),
              SeLinuxOptions: &core.v1.SELinuxOptionsArgs{
                Level: pulumi.String("string"),
                Role: pulumi.String("string"),
                Type: pulumi.String("string"),
                User: pulumi.String("string"),
              },
              SeccompProfile: &core.v1.SeccompProfileArgs{
                LocalhostProfile: pulumi.String("string"),
                Type: pulumi.String("string"),
              },
              WindowsOptions: &core.v1.WindowsSecurityContextOptionsArgs{
                GmsaCredentialSpec: pulumi.String("string"),
                GmsaCredentialSpecName: pulumi.String("string"),
                HostProcess: pulumi.Bool(true|false),
                RunAsUserName: pulumi.String("string"),
              },
            },
            StartupProbe: &core.v1.ProbeArgs{
              Exec: &core.v1.ExecActionArgs{
                Command: pulumi.StringArray{
                  pulumi.String("string")
                },
              },
              FailureThreshold: pulumi.Int(0),
              HttpGet: &core.v1.HTTPGetActionArgs{
                Host: pulumi.String("string"),
                HttpHeaders: core.v1.HTTPHeaderArray{
                  &core.v1.HTTPHeaderArgs{
                    Name: pulumi.String("string"),
                    Value: pulumi.String("string"),
                  }
                },
                Path: pulumi.String("string"),
                Port: pulumi.Int(0),
                Scheme: pulumi.String("string"),
              },
              InitialDelaySeconds: pulumi.Int(0),
              PeriodSeconds: pulumi.Int(0),
              SuccessThreshold: pulumi.Int(0),
              TcpSocket: &core.v1.TCPSocketActionArgs{
                Host: pulumi.String("string"),
                Port: pulumi.Int(0),
              },
              TerminationGracePeriodSeconds: pulumi.Int(0),
              TimeoutSeconds: pulumi.Int(0),
            },
            Stdin: pulumi.Bool(true|false),
            StdinOnce: pulumi.Bool(true|false),
            TerminationMessagePath: pulumi.String("string"),
            TerminationMessagePolicy: pulumi.String("string"),
            Tty: pulumi.Bool(true|false),
            VolumeDevices: core.v1.VolumeDeviceArray{
              &core.v1.VolumeDeviceArgs{
                DevicePath: pulumi.String("string"),
                Name: pulumi.String("string"),
              }
            },
            VolumeMounts: core.v1.VolumeMountArray{
              &core.v1.VolumeMountArgs{
                MountPath: pulumi.String("string"),
                MountPropagation: pulumi.String("string"),
                Name: pulumi.String("string"),
                ReadOnly: pulumi.Bool(true|false),
                SubPath: pulumi.String("string"),
                SubPathExpr: pulumi.String("string"),
              }
            },
            WorkingDir: pulumi.String("string"),
          }
        },
        NodeName: pulumi.String("string"),
        NodeSelector: pulumi.StringMap{
          "string": pulumi.String("string")
        },
        Overhead: pulumi.StringMap{
          "string": pulumi.String("string")
        },
        PreemptionPolicy: pulumi.String("string"),
        Priority: pulumi.Int(0),
        PriorityClassName: pulumi.String("string"),
        ReadinessGates: core.v1.PodReadinessGateArray{
          &core.v1.PodReadinessGateArgs{
            ConditionType: pulumi.String("string"),
          }
        },
        RestartPolicy: pulumi.String("string"),
        RuntimeClassName: pulumi.String("string"),
        SchedulerName: pulumi.String("string"),
        SecurityContext: &core.v1.PodSecurityContextArgs{
          FsGroup: pulumi.Int(0),
          FsGroupChangePolicy: pulumi.String("string"),
          RunAsGroup: pulumi.Int(0),
          RunAsNonRoot: pulumi.Bool(true|false),
          RunAsUser: pulumi.Int(0),
          SeLinuxOptions: &core.v1.SELinuxOptionsArgs{
            Level: pulumi.String("string"),
            Role: pulumi.String("string"),
            Type: pulumi.String("string"),
            User: pulumi.String("string"),
          },
          SeccompProfile: &core.v1.SeccompProfileArgs{
            LocalhostProfile: pulumi.String("string"),
            Type: pulumi.String("string"),
          },
          SupplementalGroups: pulumi.IntArray{
            pulumi.Int(0)
          },
          Sysctls: core.v1.SysctlArray{
            &core.v1.SysctlArgs{
              Name: pulumi.String("string"),
              Value: pulumi.String("string"),
            }
          },
          WindowsOptions: &core.v1.WindowsSecurityContextOptionsArgs{
            GmsaCredentialSpec: pulumi.String("string"),
            GmsaCredentialSpecName: pulumi.String("string"),
            HostProcess: pulumi.Bool(true|false),
            RunAsUserName: pulumi.String("string"),
          },
        },
        ServiceAccount: pulumi.String("string"),
        ServiceAccountName: pulumi.String("string"),
        SetHostnameAsFQDN: pulumi.Bool(true|false),
        ShareProcessNamespace: pulumi.Bool(true|false),
        Subdomain: pulumi.String("string"),
        TerminationGracePeriodSeconds: pulumi.Int(0),
        Tolerations: core.v1.TolerationArray{
          &core.v1.TolerationArgs{
            Effect: pulumi.String("string"),
            Key: pulumi.String("string"),
            Operator: pulumi.String("string"),
            TolerationSeconds: pulumi.Int(0),
            Value: pulumi.String("string"),
          }
        },
        TopologySpreadConstraints: core.v1.TopologySpreadConstraintArray{
          &core.v1.TopologySpreadConstraintArgs{
            LabelSelector: &meta.v1.LabelSelectorArgs{
              MatchExpressions: meta.v1.LabelSelectorRequirementArray{
                &meta.v1.LabelSelectorRequirementArgs{
                  Key: pulumi.String("string"),
                  Operator: pulumi.String("string"),
                  Values: pulumi.StringArray{
                    pulumi.String("string")
                  },
                }
              },
              MatchLabels: pulumi.StringMap{
                "string": pulumi.String("string")
              },
            },
            MaxSkew: pulumi.Int(0),
            TopologyKey: pulumi.String("string"),
            WhenUnsatisfiable: pulumi.String("string"),
          }
        },
        Volumes: core.v1.VolumeArray{
          &core.v1.VolumeArgs{
            AwsElasticBlockStore: &core.v1.AWSElasticBlockStoreVolumeSourceArgs{
              FsType: pulumi.String("string"),
              Partition: pulumi.Int(0),
              ReadOnly: pulumi.Bool(true|false),
              VolumeID: pulumi.String("string"),
            },
            AzureDisk: &core.v1.AzureDiskVolumeSourceArgs{
              CachingMode: pulumi.String("string"),
              DiskName: pulumi.String("string"),
              DiskURI: pulumi.String("string"),
              FsType: pulumi.String("string"),
              Kind: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
            },
            AzureFile: &core.v1.AzureFileVolumeSourceArgs{
              ReadOnly: pulumi.Bool(true|false),
              SecretName: pulumi.String("string"),
              ShareName: pulumi.String("string"),
            },
            Cephfs: &core.v1.CephFSVolumeSourceArgs{
              Monitors: pulumi.StringArray{
                pulumi.String("string")
              },
              Path: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
              SecretFile: pulumi.String("string"),
              SecretRef: &core.v1.LocalObjectReferenceArgs{
                Name: pulumi.String("string"),
              },
              User: pulumi.String("string"),
            },
            Cinder: &core.v1.CinderVolumeSourceArgs{
              FsType: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
              SecretRef: &core.v1.LocalObjectReferenceArgs{
                Name: pulumi.String("string"),
              },
              VolumeID: pulumi.String("string"),
            },
            ConfigMap: &core.v1.ConfigMapVolumeSourceArgs{
              DefaultMode: pulumi.Int(0),
              Items: core.v1.KeyToPathArray{
                &core.v1.KeyToPathArgs{
                  Key: pulumi.String("string"),
                  Mode: pulumi.Int(0),
                  Path: pulumi.String("string"),
                }
              },
              Name: pulumi.String("string"),
              Optional: pulumi.Bool(true|false),
            },
            Csi: &core.v1.CSIVolumeSourceArgs{
              Driver: pulumi.String("string"),
              FsType: pulumi.String("string"),
              NodePublishSecretRef: &core.v1.LocalObjectReferenceArgs{
                Name: pulumi.String("string"),
              },
              ReadOnly: pulumi.Bool(true|false),
              VolumeAttributes: pulumi.StringMap{
                "string": pulumi.String("string")
              },
            },
            DownwardAPI: &core.v1.DownwardAPIVolumeSourceArgs{
              DefaultMode: pulumi.Int(0),
              Items: core.v1.DownwardAPIVolumeFileArray{
                &core.v1.DownwardAPIVolumeFileArgs{
                  FieldRef: &core.v1.ObjectFieldSelectorArgs{
                    ApiVersion: pulumi.String("string"),
                    FieldPath: pulumi.String("string"),
                  },
                  Mode: pulumi.Int(0),
                  Path: pulumi.String("string"),
                  ResourceFieldRef: &core.v1.ResourceFieldSelectorArgs{
                    ContainerName: pulumi.String("string"),
                    Divisor: pulumi.String("string"),
                    Resource: pulumi.String("string"),
                  },
                }
              },
            },
            EmptyDir: &core.v1.EmptyDirVolumeSourceArgs{
              Medium: pulumi.String("string"),
              SizeLimit: pulumi.String("string"),
            },
            Ephemeral: &core.v1.EphemeralVolumeSourceArgs{
              ReadOnly: pulumi.Bool(true|false),
              VolumeClaimTemplate: &core.v1.PersistentVolumeClaimTemplateArgs{
                Metadata: &meta.v1.ObjectMetaArgs{
                  Annotations: pulumi.StringMap{
                    "string": pulumi.String("string")
                  },
                  ClusterName: pulumi.String("string"),
                  CreationTimestamp: pulumi.String("string"),
                  DeletionGracePeriodSeconds: pulumi.Int(0),
                  DeletionTimestamp: pulumi.String("string"),
                  Finalizers: pulumi.StringArray{
                    pulumi.String("string")
                  },
                  GenerateName: pulumi.String("string"),
                  Generation: pulumi.Int(0),
                  Labels: pulumi.StringMap{
                    "string": pulumi.String("string")
                  },
                  ManagedFields: meta.v1.ManagedFieldsEntryArray{
                    &meta.v1.ManagedFieldsEntryArgs{
                      ApiVersion: pulumi.String("string"),
                      FieldsType: pulumi.String("string"),
                      FieldsV1: ,
                      Manager: pulumi.String("string"),
                      Operation: pulumi.String("string"),
                      Subresource: pulumi.String("string"),
                      Time: pulumi.String("string"),
                    }
                  },
                  Name: pulumi.String("string"),
                  Namespace: pulumi.String("string"),
                  OwnerReferences: meta.v1.OwnerReferenceArray{
                    &meta.v1.OwnerReferenceArgs{
                      ApiVersion: pulumi.String("string"),
                      BlockOwnerDeletion: pulumi.Bool(true|false),
                      Controller: pulumi.Bool(true|false),
                      Kind: pulumi.String("string"),
                      Name: pulumi.String("string"),
                      Uid: pulumi.String("string"),
                    }
                  },
                  ResourceVersion: pulumi.String("string"),
                  SelfLink: pulumi.String("string"),
                  Uid: pulumi.String("string"),
                },
                Spec: &core.v1.PersistentVolumeClaimSpecArgs{
                  AccessModes: pulumi.StringArray{
                    pulumi.String("string")
                  },
                  DataSource: &core.v1.TypedLocalObjectReferenceArgs{
                    ApiGroup: pulumi.String("string"),
                    Kind: pulumi.String("string"),
                    Name: pulumi.String("string"),
                  },
                  DataSourceRef: &core.v1.TypedLocalObjectReferenceArgs{
                    ApiGroup: pulumi.String("string"),
                    Kind: pulumi.String("string"),
                    Name: pulumi.String("string"),
                  },
                  Resources: &core.v1.ResourceRequirementsArgs{
                    Limits: pulumi.StringMap{
                      "string": pulumi.String("string")
                    },
                    Requests: pulumi.StringMap{
                      "string": pulumi.String("string")
                    },
                  },
                  Selector: &meta.v1.LabelSelectorArgs{
                    MatchExpressions: meta.v1.LabelSelectorRequirementArray{
                      &meta.v1.LabelSelectorRequirementArgs{
                        Key: pulumi.String("string"),
                        Operator: pulumi.String("string"),
                        Values: pulumi.StringArray{
                          pulumi.String("string")
                        },
                      }
                    },
                    MatchLabels: pulumi.StringMap{
                      "string": pulumi.String("string")
                    },
                  },
                  StorageClassName: pulumi.String("string"),
                  VolumeMode: pulumi.String("string"),
                  VolumeName: pulumi.String("string"),
                },
              },
            },
            Fc: &core.v1.FCVolumeSourceArgs{
              FsType: pulumi.String("string"),
              Lun: pulumi.Int(0),
              ReadOnly: pulumi.Bool(true|false),
              TargetWWNs: pulumi.StringArray{
                pulumi.String("string")
              },
              Wwids: pulumi.StringArray{
                pulumi.String("string")
              },
            },
            FlexVolume: &core.v1.FlexVolumeSourceArgs{
              Driver: pulumi.String("string"),
              FsType: pulumi.String("string"),
              Options: pulumi.StringMap{
                "string": pulumi.String("string")
              },
              ReadOnly: pulumi.Bool(true|false),
              SecretRef: &core.v1.LocalObjectReferenceArgs{
                Name: pulumi.String("string"),
              },
            },
            Flocker: &core.v1.FlockerVolumeSourceArgs{
              DatasetName: pulumi.String("string"),
              DatasetUUID: pulumi.String("string"),
            },
            GcePersistentDisk: &core.v1.GCEPersistentDiskVolumeSourceArgs{
              FsType: pulumi.String("string"),
              Partition: pulumi.Int(0),
              PdName: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
            },
            GitRepo: &core.v1.GitRepoVolumeSourceArgs{
              Directory: pulumi.String("string"),
              Repository: pulumi.String("string"),
              Revision: pulumi.String("string"),
            },
            Glusterfs: &core.v1.GlusterfsVolumeSourceArgs{
              Endpoints: pulumi.String("string"),
              Path: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
            },
            HostPath: &core.v1.HostPathVolumeSourceArgs{
              Path: pulumi.String("string"),
              Type: pulumi.String("string"),
            },
            Iscsi: &core.v1.ISCSIVolumeSourceArgs{
              ChapAuthDiscovery: pulumi.Bool(true|false),
              ChapAuthSession: pulumi.Bool(true|false),
              FsType: pulumi.String("string"),
              InitiatorName: pulumi.String("string"),
              Iqn: pulumi.String("string"),
              IscsiInterface: pulumi.String("string"),
              Lun: pulumi.Int(0),
              Portals: pulumi.StringArray{
                pulumi.String("string")
              },
              ReadOnly: pulumi.Bool(true|false),
              SecretRef: &core.v1.LocalObjectReferenceArgs{
                Name: pulumi.String("string"),
              },
              TargetPortal: pulumi.String("string"),
            },
            Name: pulumi.String("string"),
            Nfs: &core.v1.NFSVolumeSourceArgs{
              Path: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
              Server: pulumi.String("string"),
            },
            PersistentVolumeClaim: &core.v1.PersistentVolumeClaimVolumeSourceArgs{
              ClaimName: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
            },
            PhotonPersistentDisk: &core.v1.PhotonPersistentDiskVolumeSourceArgs{
              FsType: pulumi.String("string"),
              PdID: pulumi.String("string"),
            },
            PortworxVolume: &core.v1.PortworxVolumeSourceArgs{
              FsType: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
              VolumeID: pulumi.String("string"),
            },
            Projected: &core.v1.ProjectedVolumeSourceArgs{
              DefaultMode: pulumi.Int(0),
              Sources: core.v1.VolumeProjectionArray{
                &core.v1.VolumeProjectionArgs{
                  ConfigMap: &core.v1.ConfigMapProjectionArgs{
                    Items: core.v1.KeyToPathArray{
                      &core.v1.KeyToPathArgs{
                        Key: pulumi.String("string"),
                        Mode: pulumi.Int(0),
                        Path: pulumi.String("string"),
                      }
                    },
                    Name: pulumi.String("string"),
                    Optional: pulumi.Bool(true|false),
                  },
                  DownwardAPI: &core.v1.DownwardAPIProjectionArgs{
                    Items: core.v1.DownwardAPIVolumeFileArray{
                      &core.v1.DownwardAPIVolumeFileArgs{
                        FieldRef: &core.v1.ObjectFieldSelectorArgs{
                          ApiVersion: pulumi.String("string"),
                          FieldPath: pulumi.String("string"),
                        },
                        Mode: pulumi.Int(0),
                        Path: pulumi.String("string"),
                        ResourceFieldRef: &core.v1.ResourceFieldSelectorArgs{
                          ContainerName: pulumi.String("string"),
                          Divisor: pulumi.String("string"),
                          Resource: pulumi.String("string"),
                        },
                      }
                    },
                  },
                  Secret: &core.v1.SecretProjectionArgs{
                    Items: core.v1.KeyToPathArray{
                      &core.v1.KeyToPathArgs{
                        Key: pulumi.String("string"),
                        Mode: pulumi.Int(0),
                        Path: pulumi.String("string"),
                      }
                    },
                    Name: pulumi.String("string"),
                    Optional: pulumi.Bool(true|false),
                  },
                  ServiceAccountToken: &core.v1.ServiceAccountTokenProjectionArgs{
                    Audience: pulumi.String("string"),
                    ExpirationSeconds: pulumi.Int(0),
                    Path: pulumi.String("string"),
                  },
                }
              },
            },
            Quobyte: &core.v1.QuobyteVolumeSourceArgs{
              Group: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
              Registry: pulumi.String("string"),
              Tenant: pulumi.String("string"),
              User: pulumi.String("string"),
              Volume: pulumi.String("string"),
            },
            Rbd: &core.v1.RBDVolumeSourceArgs{
              FsType: pulumi.String("string"),
              Image: pulumi.String("string"),
              Keyring: pulumi.String("string"),
              Monitors: pulumi.StringArray{
                pulumi.String("string")
              },
              Pool: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
              SecretRef: &core.v1.LocalObjectReferenceArgs{
                Name: pulumi.String("string"),
              },
              User: pulumi.String("string"),
            },
            ScaleIO: &core.v1.ScaleIOVolumeSourceArgs{
              FsType: pulumi.String("string"),
              Gateway: pulumi.String("string"),
              ProtectionDomain: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
              SecretRef: &core.v1.LocalObjectReferenceArgs{
                Name: pulumi.String("string"),
              },
              SslEnabled: pulumi.Bool(true|false),
              StorageMode: pulumi.String("string"),
              StoragePool: pulumi.String("string"),
              System: pulumi.String("string"),
              VolumeName: pulumi.String("string"),
            },
            Secret: &core.v1.SecretVolumeSourceArgs{
              DefaultMode: pulumi.Int(0),
              Items: core.v1.KeyToPathArray{
                &core.v1.KeyToPathArgs{
                  Key: pulumi.String("string"),
                  Mode: pulumi.Int(0),
                  Path: pulumi.String("string"),
                }
              },
              Optional: pulumi.Bool(true|false),
              SecretName: pulumi.String("string"),
            },
            Storageos: &core.v1.StorageOSVolumeSourceArgs{
              FsType: pulumi.String("string"),
              ReadOnly: pulumi.Bool(true|false),
              SecretRef: &core.v1.LocalObjectReferenceArgs{
                Name: pulumi.String("string"),
              },
              VolumeName: pulumi.String("string"),
              VolumeNamespace: pulumi.String("string"),
            },
            VsphereVolume: &core.v1.VsphereVirtualDiskVolumeSourceArgs{
              FsType: pulumi.String("string"),
              StoragePolicyID: pulumi.String("string"),
              StoragePolicyName: pulumi.String("string"),
              VolumePath: pulumi.String("string"),
            },
          }
        },
      },
    },
  },
})
```

### Java

```java
import com.pulumi.Pulumi;
import java.util.List;
import java.util.Map;

var deployment = new Deployment("deployment", DeploymentArgs.builder()
  .apiVersion("string")
  .kind("string")
  .metadata(ObjectMetaArgs.builder()
    .annotations(Map.ofEntries(
      Map.entry("string", "string")
    ))
    .clusterName("string")
    .creationTimestamp("string")
    .deletionGracePeriodSeconds(0)
    .deletionTimestamp("string")
    .finalizers(List.of("string"))
    .generateName("string")
    .generation(0)
    .labels(Map.ofEntries(
      Map.entry("string", "string")
    ))
    .managedFields(List.of(
      ManagedFieldsEntryArgs.builder()
        .apiVersion("string")
        .fieldsType("string")
        .fieldsV1()
        .manager("string")
        .operation("string")
        .subresource("string")
        .time("string")
        .build()
    ))
    .name("string")
    .namespace("string")
    .ownerReferences(List.of(
      OwnerReferenceArgs.builder()
        .apiVersion("string")
        .blockOwnerDeletion(true|false)
        .controller(true|false)
        .kind("string")
        .name("string")
        .uid("string")
        .build()
    ))
    .resourceVersion("string")
    .selfLink("string")
    .uid("string")
    .build())
  .spec(DeploymentSpecArgs.builder()
    .minReadySeconds(0)
    .paused(true|false)
    .progressDeadlineSeconds(0)
    .replicas(0)
    .revisionHistoryLimit(0)
    .selector(LabelSelectorArgs.builder()
      .matchExpressions(List.of(
        LabelSelectorRequirementArgs.builder()
          .key("string")
          .operator("string")
          .values(List.of("string"))
          .build()
      ))
      .matchLabels(Map.ofEntries(
        Map.entry("string", "string")
      ))
      .build())
    .strategy(DeploymentStrategyArgs.builder()
      .rollingUpdate(RollingUpdateDeploymentArgs.builder()
        .maxSurge(0)
        .maxUnavailable(0)
        .build())
      .type("string")
      .build())
    .template(PodTemplateSpecArgs.builder()
      .metadata(ObjectMetaArgs.builder()
        .annotations(Map.ofEntries(
          Map.entry("string", "string")
        ))
        .clusterName("string")
        .creationTimestamp("string")
        .deletionGracePeriodSeconds(0)
        .deletionTimestamp("string")
        .finalizers(List.of("string"))
        .generateName("string")
        .generation(0)
        .labels(Map.ofEntries(
          Map.entry("string", "string")
        ))
        .managedFields(List.of(
          ManagedFieldsEntryArgs.builder()
            .apiVersion("string")
            .fieldsType("string")
            .fieldsV1()
            .manager("string")
            .operation("string")
            .subresource("string")
            .time("string")
            .build()
        ))
        .name("string")
        .namespace("string")
        .ownerReferences(List.of(
          OwnerReferenceArgs.builder()
            .apiVersion("string")
            .blockOwnerDeletion(true|false)
            .controller(true|false)
            .kind("string")
            .name("string")
            .uid("string")
            .build()
        ))
        .resourceVersion("string")
        .selfLink("string")
        .uid("string")
        .build())
      .spec(PodSpecArgs.builder()
        .activeDeadlineSeconds(0)
        .affinity(AffinityArgs.builder()
          .nodeAffinity(NodeAffinityArgs.builder()
            .preferredDuringSchedulingIgnoredDuringExecution(List.of(
              PreferredSchedulingTermArgs.builder()
                .preference(NodeSelectorTermArgs.builder()
                  .matchExpressions(List.of(
                    NodeSelectorRequirementArgs.builder()
                      .key("string")
                      .operator("string")
                      .values(List.of("string"))
                      .build()
                  ))
                  .matchFields(List.of(
                    NodeSelectorRequirementArgs.builder()
                      .key("string")
                      .operator("string")
                      .values(List.of("string"))
                      .build()
                  ))
                  .build())
                .weight(0)
                .build()
            ))
            .requiredDuringSchedulingIgnoredDuringExecution(NodeSelectorArgs.builder()
              .nodeSelectorTerms(List.of(
                NodeSelectorTermArgs.builder()
                  .matchExpressions(List.of(
                    NodeSelectorRequirementArgs.builder()
                      .key("string")
                      .operator("string")
                      .values(List.of("string"))
                      .build()
                  ))
                  .matchFields(List.of(
                    NodeSelectorRequirementArgs.builder()
                      .key("string")
                      .operator("string")
                      .values(List.of("string"))
                      .build()
                  ))
                  .build()
              ))
              .build())
            .build())
          .podAffinity(PodAffinityArgs.builder()
            .preferredDuringSchedulingIgnoredDuringExecution(List.of(
              WeightedPodAffinityTermArgs.builder()
                .podAffinityTerm(PodAffinityTermArgs.builder()
                  .labelSelector(LabelSelectorArgs.builder()
                    .matchExpressions(List.of(
                      LabelSelectorRequirementArgs.builder()
                        .key("string")
                        .operator("string")
                        .values(List.of("string"))
                        .build()
                    ))
                    .matchLabels(Map.ofEntries(
                      Map.entry("string", "string")
                    ))
                    .build())
                  .namespaceSelector(LabelSelectorArgs.builder()
                    .matchExpressions(List.of(
                      LabelSelectorRequirementArgs.builder()
                        .key("string")
                        .operator("string")
                        .values(List.of("string"))
                        .build()
                    ))
                    .matchLabels(Map.ofEntries(
                      Map.entry("string", "string")
                    ))
                    .build())
                  .namespaces(List.of("string"))
                  .topologyKey("string")
                  .build())
                .weight(0)
                .build()
            ))
            .requiredDuringSchedulingIgnoredDuringExecution(List.of(
              PodAffinityTermArgs.builder()
                .labelSelector(LabelSelectorArgs.builder()
                  .matchExpressions(List.of(
                    LabelSelectorRequirementArgs.builder()
                      .key("string")
                      .operator("string")
                      .values(List.of("string"))
                      .build()
                  ))
                  .matchLabels(Map.ofEntries(
                    Map.entry("string", "string")
                  ))
                  .build())
                .namespaceSelector(LabelSelectorArgs.builder()
                  .matchExpressions(List.of(
                    LabelSelectorRequirementArgs.builder()
                      .key("string")
                      .operator("string")
                      .values(List.of("string"))
                      .build()
                  ))
                  .matchLabels(Map.ofEntries(
                    Map.entry("string", "string")
                  ))
                  .build())
                .namespaces(List.of("string"))
                .topologyKey("string")
                .build()
            ))
            .build())
          .podAntiAffinity(PodAntiAffinityArgs.builder()
            .preferredDuringSchedulingIgnoredDuringExecution(List.of(
              WeightedPodAffinityTermArgs.builder()
                .podAffinityTerm(PodAffinityTermArgs.builder()
                  .labelSelector(LabelSelectorArgs.builder()
                    .matchExpressions(List.of(
                      LabelSelectorRequirementArgs.builder()
                        .key("string")
                        .operator("string")
                        .values(List.of("string"))
                        .build()
                    ))
                    .matchLabels(Map.ofEntries(
                      Map.entry("string", "string")
                    ))
                    .build())
                  .namespaceSelector(LabelSelectorArgs.builder()
                    .matchExpressions(List.of(
                      LabelSelectorRequirementArgs.builder()
                        .key("string")
                        .operator("string")
                        .values(List.of("string"))
                        .build()
                    ))
                    .matchLabels(Map.ofEntries(
                      Map.entry("string", "string")
                    ))
                    .build())
                  .namespaces(List.of("string"))
                  .topologyKey("string")
                  .build())
                .weight(0)
                .build()
            ))
            .requiredDuringSchedulingIgnoredDuringExecution(List.of(
              PodAffinityTermArgs.builder()
                .labelSelector(LabelSelectorArgs.builder()
                  .matchExpressions(List.of(
                    LabelSelectorRequirementArgs.builder()
                      .key("string")
                      .operator("string")
                      .values(List.of("string"))
                      .build()
                  ))
                  .matchLabels(Map.ofEntries(
                    Map.entry("string", "string")
                  ))
                  .build())
                .namespaceSelector(LabelSelectorArgs.builder()
                  .matchExpressions(List.of(
                    LabelSelectorRequirementArgs.builder()
                      .key("string")
                      .operator("string")
                      .values(List.of("string"))
                      .build()
                  ))
                  .matchLabels(Map.ofEntries(
                    Map.entry("string", "string")
                  ))
                  .build())
                .namespaces(List.of("string"))
                .topologyKey("string")
                .build()
            ))
            .build())
          .build())
        .automountServiceAccountToken(true|false)
        .containers(List.of(
          ContainerArgs.builder()
            .args(List.of("string"))
            .command(List.of("string"))
            .env(List.of(
              EnvVarArgs.builder()
                .name("string")
                .value("string")
                .valueFrom(EnvVarSourceArgs.builder()
                  .configMapKeyRef(ConfigMapKeySelectorArgs.builder()
                    .key("string")
                    .name("string")
                    .optional(true|false)
                    .build())
                  .fieldRef(ObjectFieldSelectorArgs.builder()
                    .apiVersion("string")
                    .fieldPath("string")
                    .build())
                  .resourceFieldRef(ResourceFieldSelectorArgs.builder()
                    .containerName("string")
                    .divisor("string")
                    .resource("string")
                    .build())
                  .secretKeyRef(SecretKeySelectorArgs.builder()
                    .key("string")
                    .name("string")
                    .optional(true|false)
                    .build())
                  .build())
                .build()
            ))
            .envFrom(List.of(
              EnvFromSourceArgs.builder()
                .configMapRef(ConfigMapEnvSourceArgs.builder()
                  .name("string")
                  .optional(true|false)
                  .build())
                .prefix("string")
                .secretRef(SecretEnvSourceArgs.builder()
                  .name("string")
                  .optional(true|false)
                  .build())
                .build()
            ))
            .image("string")
            .imagePullPolicy("string")
            .lifecycle(LifecycleArgs.builder()
              .postStart(HandlerArgs.builder()
                .exec(ExecActionArgs.builder()
                  .command(List.of("string"))
                  .build())
                .httpGet(HTTPGetActionArgs.builder()
                  .host("string")
                  .httpHeaders(List.of(
                    HTTPHeaderArgs.builder()
                      .name("string")
                      .value("string")
                      .build()
                  ))
                  .path("string")
                  .port(0)
                  .scheme("string")
                  .build())
                .tcpSocket(TCPSocketActionArgs.builder()
                  .host("string")
                  .port(0)
                  .build())
                .build())
              .preStop(HandlerArgs.builder()
                .exec(ExecActionArgs.builder()
                  .command(List.of("string"))
                  .build())
                .httpGet(HTTPGetActionArgs.builder()
                  .host("string")
                  .httpHeaders(List.of(
                    HTTPHeaderArgs.builder()
                      .name("string")
                      .value("string")
                      .build()
                  ))
                  .path("string")
                  .port(0)
                  .scheme("string")
                  .build())
                .tcpSocket(TCPSocketActionArgs.builder()
                  .host("string")
                  .port(0)
                  .build())
                .build())
              .build())
            .livenessProbe(ProbeArgs.builder()
              .exec(ExecActionArgs.builder()
                .command(List.of("string"))
                .build())
              .failureThreshold(0)
              .httpGet(HTTPGetActionArgs.builder()
                .host("string")
                .httpHeaders(List.of(
                  HTTPHeaderArgs.builder()
                    .name("string")
                    .value("string")
                    .build()
                ))
                .path("string")
                .port(0)
                .scheme("string")
                .build())
              .initialDelaySeconds(0)
              .periodSeconds(0)
              .successThreshold(0)
              .tcpSocket(TCPSocketActionArgs.builder()
                .host("string")
                .port(0)
                .build())
              .terminationGracePeriodSeconds(0)
              .timeoutSeconds(0)
              .build())
            .name("string")
            .ports(List.of(
              ContainerPortArgs.builder()
                .containerPort(0)
                .hostIP("string")
                .hostPort(0)
                .name("string")
                .protocol("string")
                .build()
            ))
            .readinessProbe(ProbeArgs.builder()
              .exec(ExecActionArgs.builder()
                .command(List.of("string"))
                .build())
              .failureThreshold(0)
              .httpGet(HTTPGetActionArgs.builder()
                .host("string")
                .httpHeaders(List.of(
                  HTTPHeaderArgs.builder()
                    .name("string")
                    .value("string")
                    .build()
                ))
                .path("string")
                .port(0)
                .scheme("string")
                .build())
              .initialDelaySeconds(0)
              .periodSeconds(0)
              .successThreshold(0)
              .tcpSocket(TCPSocketActionArgs.builder()
                .host("string")
                .port(0)
                .build())
              .terminationGracePeriodSeconds(0)
              .timeoutSeconds(0)
              .build())
            .resources(ResourceRequirementsArgs.builder()
              .limits(Map.ofEntries(
                Map.entry("string", "string")
              ))
              .requests(Map.ofEntries(
                Map.entry("string", "string")
              ))
              .build())
            .securityContext(SecurityContextArgs.builder()
              .allowPrivilegeEscalation(true|false)
              .capabilities(CapabilitiesArgs.builder()
                .add(List.of("string"))
                .drop(List.of("string"))
                .build())
              .privileged(true|false)
              .procMount("string")
              .readOnlyRootFilesystem(true|false)
              .runAsGroup(0)
              .runAsNonRoot(true|false)
              .runAsUser(0)
              .seLinuxOptions(SELinuxOptionsArgs.builder()
                .level("string")
                .role("string")
                .type("string")
                .user("string")
                .build())
              .seccompProfile(SeccompProfileArgs.builder()
                .localhostProfile("string")
                .type("string")
                .build())
              .windowsOptions(WindowsSecurityContextOptionsArgs.builder()
                .gmsaCredentialSpec("string")
                .gmsaCredentialSpecName("string")
                .hostProcess(true|false)
                .runAsUserName("string")
                .build())
              .build())
            .startupProbe(ProbeArgs.builder()
              .exec(ExecActionArgs.builder()
                .command(List.of("string"))
                .build())
              .failureThreshold(0)
              .httpGet(HTTPGetActionArgs.builder()
                .host("string")
                .httpHeaders(List.of(
                  HTTPHeaderArgs.builder()
                    .name("string")
                    .value("string")
                    .build()
                ))
                .path("string")
                .port(0)
                .scheme("string")
                .build())
              .initialDelaySeconds(0)
              .periodSeconds(0)
              .successThreshold(0)
              .tcpSocket(TCPSocketActionArgs.builder()
                .host("string")
                .port(0)
                .build())
              .terminationGracePeriodSeconds(0)
              .timeoutSeconds(0)
              .build())
            .stdin(true|false)
            .stdinOnce(true|false)
            .terminationMessagePath("string")
            .terminationMessagePolicy("string")
            .tty(true|false)
            .volumeDevices(List.of(
              VolumeDeviceArgs.builder()
                .devicePath("string")
                .name("string")
                .build()
            ))
            .volumeMounts(List.of(
              VolumeMountArgs.builder()
                .mountPath("string")
                .mountPropagation("string")
                .name("string")
                .readOnly(true|false)
                .subPath("string")
                .subPathExpr("string")
                .build()
            ))
            .workingDir("string")
            .build()
        ))
        .dnsConfig(PodDNSConfigArgs.builder()
          .nameservers(List.of("string"))
          .options(List.of(
            PodDNSConfigOptionArgs.builder()
              .name("string")
              .value("string")
              .build()
          ))
          .searches(List.of("string"))
          .build())
        .dnsPolicy("string")
        .enableServiceLinks(true|false)
        .ephemeralContainers(List.of(
          EphemeralContainerArgs.builder()
            .args(List.of("string"))
            .command(List.of("string"))
            .env(List.of(
              EnvVarArgs.builder()
                .name("string")
                .value("string")
                .valueFrom(EnvVarSourceArgs.builder()
                  .configMapKeyRef(ConfigMapKeySelectorArgs.builder()
                    .key("string")
                    .name("string")
                    .optional(true|false)
                    .build())
                  .fieldRef(ObjectFieldSelectorArgs.builder()
                    .apiVersion("string")
                    .fieldPath("string")
                    .build())
                  .resourceFieldRef(ResourceFieldSelectorArgs.builder()
                    .containerName("string")
                    .divisor("string")
                    .resource("string")
                    .build())
                  .secretKeyRef(SecretKeySelectorArgs.builder()
                    .key("string")
                    .name("string")
                    .optional(true|false)
                    .build())
                  .build())
                .build()
            ))
            .envFrom(List.of(
              EnvFromSourceArgs.builder()
                .configMapRef(ConfigMapEnvSourceArgs.builder()
                  .name("string")
                  .optional(true|false)
                  .build())
                .prefix("string")
                .secretRef(SecretEnvSourceArgs.builder()
                  .name("string")
                  .optional(true|false)
                  .build())
                .build()
            ))
            .image("string")
            .imagePullPolicy("string")
            .lifecycle(LifecycleArgs.builder()
              .postStart(HandlerArgs.builder()
                .exec(ExecActionArgs.builder()
                  .command(List.of("string"))
                  .build())
                .httpGet(HTTPGetActionArgs.builder()
                  .host("string")
                  .httpHeaders(List.of(
                    HTTPHeaderArgs.builder()
                      .name("string")
                      .value("string")
                      .build()
                  ))
                  .path("string")
                  .port(0)
                  .scheme("string")
                  .build())
                .tcpSocket(TCPSocketActionArgs.builder()
                  .host("string")
                  .port(0)
                  .build())
                .build())
              .preStop(HandlerArgs.builder()
                .exec(ExecActionArgs.builder()
                  .command(List.of("string"))
                  .build())
                .httpGet(HTTPGetActionArgs.builder()
                  .host("string")
                  .httpHeaders(List.of(
                    HTTPHeaderArgs.builder()
                      .name("string")
                      .value("string")
                      .build()
                  ))
                  .path("string")
                  .port(0)
                  .scheme("string")
                  .build())
                .tcpSocket(TCPSocketActionArgs.builder()
                  .host("string")
                  .port(0)
                  .build())
                .build())
              .build())
            .livenessProbe(ProbeArgs.builder()
              .exec(ExecActionArgs.builder()
                .command(List.of("string"))
                .build())
              .failureThreshold(0)
              .httpGet(HTTPGetActionArgs.builder()
                .host("string")
                .httpHeaders(List.of(
                  HTTPHeaderArgs.builder()
                    .name("string")
                    .value("string")
                    .build()
                ))
                .path("string")
                .port(0)
                .scheme("string")
                .build())
              .initialDelaySeconds(0)
              .periodSeconds(0)
              .successThreshold(0)
              .tcpSocket(TCPSocketActionArgs.builder()
                .host("string")
                .port(0)
                .build())
              .terminationGracePeriodSeconds(0)
              .timeoutSeconds(0)
              .build())
            .name("string")
            .ports(List.of(
              ContainerPortArgs.builder()
                .containerPort(0)
                .hostIP("string")
                .hostPort(0)
                .name("string")
                .protocol("string")
                .build()
            ))
            .readinessProbe(ProbeArgs.builder()
              .exec(ExecActionArgs.builder()
                .command(List.of("string"))
                .build())
              .failureThreshold(0)
              .httpGet(HTTPGetActionArgs.builder()
                .host("string")
                .httpHeaders(List.of(
                  HTTPHeaderArgs.builder()
                    .name("string")
                    .value("string")
                    .build()
                ))
                .path("string")
                .port(0)
                .scheme("string")
                .build())
              .initialDelaySeconds(0)
              .periodSeconds(0)
              .successThreshold(0)
              .tcpSocket(TCPSocketActionArgs.builder()
                .host("string")
                .port(0)
                .build())
              .terminationGracePeriodSeconds(0)
              .timeoutSeconds(0)
              .build())
            .resources(ResourceRequirementsArgs.builder()
              .limits(Map.ofEntries(
                Map.entry("string", "string")
              ))
              .requests(Map.ofEntries(
                Map.entry("string", "string")
              ))
              .build())
            .securityContext(SecurityContextArgs.builder()
              .allowPrivilegeEscalation(true|false)
              .capabilities(CapabilitiesArgs.builder()
                .add(List.of("string"))
                .drop(List.of("string"))
                .build())
              .privileged(true|false)
              .procMount("string")
              .readOnlyRootFilesystem(true|false)
              .runAsGroup(0)
              .runAsNonRoot(true|false)
              .runAsUser(0)
              .seLinuxOptions(SELinuxOptionsArgs.builder()
                .level("string")
                .role("string")
                .type("string")
                .user("string")
                .build())
              .seccompProfile(SeccompProfileArgs.builder()
                .localhostProfile("string")
                .type("string")
                .build())
              .windowsOptions(WindowsSecurityContextOptionsArgs.builder()
                .gmsaCredentialSpec("string")
                .gmsaCredentialSpecName("string")
                .hostProcess(true|false)
                .runAsUserName("string")
                .build())
              .build())
            .startupProbe(ProbeArgs.builder()
              .exec(ExecActionArgs.builder()
                .command(List.of("string"))
                .build())
              .failureThreshold(0)
              .httpGet(HTTPGetActionArgs.builder()
                .host("string")
                .httpHeaders(List.of(
                  HTTPHeaderArgs.builder()
                    .name("string")
                    .value("string")
                    .build()
                ))
                .path("string")
                .port(0)
                .scheme("string")
                .build())
              .initialDelaySeconds(0)
              .periodSeconds(0)
              .successThreshold(0)
              .tcpSocket(TCPSocketActionArgs.builder()
                .host("string")
                .port(0)
                .build())
              .terminationGracePeriodSeconds(0)
              .timeoutSeconds(0)
              .build())
            .stdin(true|false)
            .stdinOnce(true|false)
            .targetContainerName("string")
            .terminationMessagePath("string")
            .terminationMessagePolicy("string")
            .tty(true|false)
            .volumeDevices(List.of(
              VolumeDeviceArgs.builder()
                .devicePath("string")
                .name("string")
                .build()
            ))
            .volumeMounts(List.of(
              VolumeMountArgs.builder()
                .mountPath("string")
                .mountPropagation("string")
                .name("string")
                .readOnly(true|false)
                .subPath("string")
                .subPathExpr("string")
                .build()
            ))
            .workingDir("string")
            .build()
        ))
        .hostAliases(List.of(
          HostAliasArgs.builder()
            .hostnames(List.of("string"))
            .ip("string")
            .build()
        ))
        .hostIPC(true|false)
        .hostNetwork(true|false)
        .hostPID(true|false)
        .hostname("string")
        .imagePullSecrets(List.of(
          LocalObjectReferenceArgs.builder()
            .name("string")
            .build()
        ))
        .initContainers(List.of(
          ContainerArgs.builder()
            .args(List.of("string"))
            .command(List.of("string"))
            .env(List.of(
              EnvVarArgs.builder()
                .name("string")
                .value("string")
                .valueFrom(EnvVarSourceArgs.builder()
                  .configMapKeyRef(ConfigMapKeySelectorArgs.builder()
                    .key("string")
                    .name("string")
                    .optional(true|false)
                    .build())
                  .fieldRef(ObjectFieldSelectorArgs.builder()
                    .apiVersion("string")
                    .fieldPath("string")
                    .build())
                  .resourceFieldRef(ResourceFieldSelectorArgs.builder()
                    .containerName("string")
                    .divisor("string")
                    .resource("string")
                    .build())
                  .secretKeyRef(SecretKeySelectorArgs.builder()
                    .key("string")
                    .name("string")
                    .optional(true|false)
                    .build())
                  .build())
                .build()
            ))
            .envFrom(List.of(
              EnvFromSourceArgs.builder()
                .configMapRef(ConfigMapEnvSourceArgs.builder()
                  .name("string")
                  .optional(true|false)
                  .build())
                .prefix("string")
                .secretRef(SecretEnvSourceArgs.builder()
                  .name("string")
                  .optional(true|false)
                  .build())
                .build()
            ))
            .image("string")
            .imagePullPolicy("string")
            .lifecycle(LifecycleArgs.builder()
              .postStart(HandlerArgs.builder()
                .exec(ExecActionArgs.builder()
                  .command(List.of("string"))
                  .build())
                .httpGet(HTTPGetActionArgs.builder()
                  .host("string")
                  .httpHeaders(List.of(
                    HTTPHeaderArgs.builder()
                      .name("string")
                      .value("string")
                      .build()
                  ))
                  .path("string")
                  .port(0)
                  .scheme("string")
                  .build())
                .tcpSocket(TCPSocketActionArgs.builder()
                  .host("string")
                  .port(0)
                  .build())
                .build())
              .preStop(HandlerArgs.builder()
                .exec(ExecActionArgs.builder()
                  .command(List.of("string"))
                  .build())
                .httpGet(HTTPGetActionArgs.builder()
                  .host("string")
                  .httpHeaders(List.of(
                    HTTPHeaderArgs.builder()
                      .name("string")
                      .value("string")
                      .build()
                  ))
                  .path("string")
                  .port(0)
                  .scheme("string")
                  .build())
                .tcpSocket(TCPSocketActionArgs.builder()
                  .host("string")
                  .port(0)
                  .build())
                .build())
              .build())
            .livenessProbe(ProbeArgs.builder()
              .exec(ExecActionArgs.builder()
                .command(List.of("string"))
                .build())
              .failureThreshold(0)
              .httpGet(HTTPGetActionArgs.builder()
                .host("string")
                .httpHeaders(List.of(
                  HTTPHeaderArgs.builder()
                    .name("string")
                    .value("string")
                    .build()
                ))
                .path("string")
                .port(0)
                .scheme("string")
                .build())
              .initialDelaySeconds(0)
              .periodSeconds(0)
              .successThreshold(0)
              .tcpSocket(TCPSocketActionArgs.builder()
                .host("string")
                .port(0)
                .build())
              .terminationGracePeriodSeconds(0)
              .timeoutSeconds(0)
              .build())
            .name("string")
            .ports(List.of(
              ContainerPortArgs.builder()
                .containerPort(0)
                .hostIP("string")
                .hostPort(0)
                .name("string")
                .protocol("string")
                .build()
            ))
            .readinessProbe(ProbeArgs.builder()
              .exec(ExecActionArgs.builder()
                .command(List.of("string"))
                .build())
              .failureThreshold(0)
              .httpGet(HTTPGetActionArgs.builder()
                .host("string")
                .httpHeaders(List.of(
                  HTTPHeaderArgs.builder()
                    .name("string")
                    .value("string")
                    .build()
                ))
                .path("string")
                .port(0)
                .scheme("string")
                .build())
              .initialDelaySeconds(0)
              .periodSeconds(0)
              .successThreshold(0)
              .tcpSocket(TCPSocketActionArgs.builder()
                .host("string")
                .port(0)
                .build())
              .terminationGracePeriodSeconds(0)
              .timeoutSeconds(0)
              .build())
            .resources(ResourceRequirementsArgs.builder()
              .limits(Map.ofEntries(
                Map.entry("string", "string")
              ))
              .requests(Map.ofEntries(
                Map.entry("string", "string")
              ))
              .build())
            .securityContext(SecurityContextArgs.builder()
              .allowPrivilegeEscalation(true|false)
              .capabilities(CapabilitiesArgs.builder()
                .add(List.of("string"))
                .drop(List.of("string"))
                .build())
              .privileged(true|false)
              .procMount("string")
              .readOnlyRootFilesystem(true|false)
              .runAsGroup(0)
              .runAsNonRoot(true|false)
              .runAsUser(0)
              .seLinuxOptions(SELinuxOptionsArgs.builder()
                .level("string")
                .role("string")
                .type("string")
                .user("string")
                .build())
              .seccompProfile(SeccompProfileArgs.builder()
                .localhostProfile("string")
                .type("string")
                .build())
              .windowsOptions(WindowsSecurityContextOptionsArgs.builder()
                .gmsaCredentialSpec("string")
                .gmsaCredentialSpecName("string")
                .hostProcess(true|false)
                .runAsUserName("string")
                .build())
              .build())
            .startupProbe(ProbeArgs.builder()
              .exec(ExecActionArgs.builder()
                .command(List.of("string"))
                .build())
              .failureThreshold(0)
              .httpGet(HTTPGetActionArgs.builder()
                .host("string")
                .httpHeaders(List.of(
                  HTTPHeaderArgs.builder()
                    .name("string")
                    .value("string")
                    .build()
                ))
                .path("string")
                .port(0)
                .scheme("string")
                .build())
              .initialDelaySeconds(0)
              .periodSeconds(0)
              .successThreshold(0)
              .tcpSocket(TCPSocketActionArgs.builder()
                .host("string")
                .port(0)
                .build())
              .terminationGracePeriodSeconds(0)
              .timeoutSeconds(0)
              .build())
            .stdin(true|false)
            .stdinOnce(true|false)
            .terminationMessagePath("string")
            .terminationMessagePolicy("string")
            .tty(true|false)
            .volumeDevices(List.of(
              VolumeDeviceArgs.builder()
                .devicePath("string")
                .name("string")
                .build()
            ))
            .volumeMounts(List.of(
              VolumeMountArgs.builder()
                .mountPath("string")
                .mountPropagation("string")
                .name("string")
                .readOnly(true|false)
                .subPath("string")
                .subPathExpr("string")
                .build()
            ))
            .workingDir("string")
            .build()
        ))
        .nodeName("string")
        .nodeSelector(Map.ofEntries(
          Map.entry("string", "string")
        ))
        .overhead(Map.ofEntries(
          Map.entry("string", "string")
        ))
        .preemptionPolicy("string")
        .priority(0)
        .priorityClassName("string")
        .readinessGates(List.of(
          PodReadinessGateArgs.builder()
            .conditionType("string")
            .build()
        ))
        .restartPolicy("string")
        .runtimeClassName("string")
        .schedulerName("string")
        .securityContext(PodSecurityContextArgs.builder()
          .fsGroup(0)
          .fsGroupChangePolicy("string")
          .runAsGroup(0)
          .runAsNonRoot(true|false)
          .runAsUser(0)
          .seLinuxOptions(SELinuxOptionsArgs.builder()
            .level("string")
            .role("string")
            .type("string")
            .user("string")
            .build())
          .seccompProfile(SeccompProfileArgs.builder()
            .localhostProfile("string")
            .type("string")
            .build())
          .supplementalGroups(List.of(0))
          .sysctls(List.of(
            SysctlArgs.builder()
              .name("string")
              .value("string")
              .build()
          ))
          .windowsOptions(WindowsSecurityContextOptionsArgs.builder()
            .gmsaCredentialSpec("string")
            .gmsaCredentialSpecName("string")
            .hostProcess(true|false)
            .runAsUserName("string")
            .build())
          .build())
        .serviceAccount("string")
        .serviceAccountName("string")
        .setHostnameAsFQDN(true|false)
        .shareProcessNamespace(true|false)
        .subdomain("string")
        .terminationGracePeriodSeconds(0)
        .tolerations(List.of(
          TolerationArgs.builder()
            .effect("string")
            .key("string")
            .operator("string")
            .tolerationSeconds(0)
            .value("string")
            .build()
        ))
        .topologySpreadConstraints(List.of(
          TopologySpreadConstraintArgs.builder()
            .labelSelector(LabelSelectorArgs.builder()
              .matchExpressions(List.of(
                LabelSelectorRequirementArgs.builder()
                  .key("string")
                  .operator("string")
                  .values(List.of("string"))
                  .build()
              ))
              .matchLabels(Map.ofEntries(
                Map.entry("string", "string")
              ))
              .build())
            .maxSkew(0)
            .topologyKey("string")
            .whenUnsatisfiable("string")
            .build()
        ))
        .volumes(List.of(
          VolumeArgs.builder()
            .awsElasticBlockStore(AWSElasticBlockStoreVolumeSourceArgs.builder()
              .fsType("string")
              .partition(0)
              .readOnly(true|false)
              .volumeID("string")
              .build())
            .azureDisk(AzureDiskVolumeSourceArgs.builder()
              .cachingMode("string")
              .diskName("string")
              .diskURI("string")
              .fsType("string")
              .kind("string")
              .readOnly(true|false)
              .build())
            .azureFile(AzureFileVolumeSourceArgs.builder()
              .readOnly(true|false)
              .secretName("string")
              .shareName("string")
              .build())
            .cephfs(CephFSVolumeSourceArgs.builder()
              .monitors(List.of("string"))
              .path("string")
              .readOnly(true|false)
              .secretFile("string")
              .secretRef(LocalObjectReferenceArgs.builder()
                .name("string")
                .build())
              .user("string")
              .build())
            .cinder(CinderVolumeSourceArgs.builder()
              .fsType("string")
              .readOnly(true|false)
              .secretRef(LocalObjectReferenceArgs.builder()
                .name("string")
                .build())
              .volumeID("string")
              .build())
            .configMap(ConfigMapVolumeSourceArgs.builder()
              .defaultMode(0)
              .items(List.of(
                KeyToPathArgs.builder()
                  .key("string")
                  .mode(0)
                  .path("string")
                  .build()
              ))
              .name("string")
              .optional(true|false)
              .build())
            .csi(CSIVolumeSourceArgs.builder()
              .driver("string")
              .fsType("string")
              .nodePublishSecretRef(LocalObjectReferenceArgs.builder()
                .name("string")
                .build())
              .readOnly(true|false)
              .volumeAttributes(Map.ofEntries(
                Map.entry("string", "string")
              ))
              .build())
            .downwardAPI(DownwardAPIVolumeSourceArgs.builder()
              .defaultMode(0)
              .items(List.of(
                DownwardAPIVolumeFileArgs.builder()
                  .fieldRef(ObjectFieldSelectorArgs.builder()
                    .apiVersion("string")
                    .fieldPath("string")
                    .build())
                  .mode(0)
                  .path("string")
                  .resourceFieldRef(ResourceFieldSelectorArgs.builder()
                    .containerName("string")
                    .divisor("string")
                    .resource("string")
                    .build())
                  .build()
              ))
              .build())
            .emptyDir(EmptyDirVolumeSourceArgs.builder()
              .medium("string")
              .sizeLimit("string")
              .build())
            .ephemeral(EphemeralVolumeSourceArgs.builder()
              .readOnly(true|false)
              .volumeClaimTemplate(PersistentVolumeClaimTemplateArgs.builder()
                .metadata(ObjectMetaArgs.builder()
                  .annotations(Map.ofEntries(
                    Map.entry("string", "string")
                  ))
                  .clusterName("string")
                  .creationTimestamp("string")
                  .deletionGracePeriodSeconds(0)
                  .deletionTimestamp("string")
                  .finalizers(List.of("string"))
                  .generateName("string")
                  .generation(0)
                  .labels(Map.ofEntries(
                    Map.entry("string", "string")
                  ))
                  .managedFields(List.of(
                    ManagedFieldsEntryArgs.builder()
                      .apiVersion("string")
                      .fieldsType("string")
                      .fieldsV1()
                      .manager("string")
                      .operation("string")
                      .subresource("string")
                      .time("string")
                      .build()
                  ))
                  .name("string")
                  .namespace("string")
                  .ownerReferences(List.of(
                    OwnerReferenceArgs.builder()
                      .apiVersion("string")
                      .blockOwnerDeletion(true|false)
                      .controller(true|false)
                      .kind("string")
                      .name("string")
                      .uid("string")
                      .build()
                  ))
                  .resourceVersion("string")
                  .selfLink("string")
                  .uid("string")
                  .build())
                .spec(PersistentVolumeClaimSpecArgs.builder()
                  .accessModes(List.of("string"))
                  .dataSource(TypedLocalObjectReferenceArgs.builder()
                    .apiGroup("string")
                    .kind("string")
                    .name("string")
                    .build())
                  .dataSourceRef(TypedLocalObjectReferenceArgs.builder()
                    .apiGroup("string")
                    .kind("string")
                    .name("string")
                    .build())
                  .resources(ResourceRequirementsArgs.builder()
                    .limits(Map.ofEntries(
                      Map.entry("string", "string")
                    ))
                    .requests(Map.ofEntries(
                      Map.entry("string", "string")
                    ))
                    .build())
                  .selector(LabelSelectorArgs.builder()
                    .matchExpressions(List.of(
                      LabelSelectorRequirementArgs.builder()
                        .key("string")
                        .operator("string")
                        .values(List.of("string"))
                        .build()
                    ))
                    .matchLabels(Map.ofEntries(
                      Map.entry("string", "string")
                    ))
                    .build())
                  .storageClassName("string")
                  .volumeMode("string")
                  .volumeName("string")
                  .build())
                .build())
              .build())
            .fc(FCVolumeSourceArgs.builder()
              .fsType("string")
              .lun(0)
              .readOnly(true|false)
              .targetWWNs(List.of("string"))
              .wwids(List.of("string"))
              .build())
            .flexVolume(FlexVolumeSourceArgs.builder()
              .driver("string")
              .fsType("string")
              .options(Map.ofEntries(
                Map.entry("string", "string")
              ))
              .readOnly(true|false)
              .secretRef(LocalObjectReferenceArgs.builder()
                .name("string")
                .build())
              .build())
            .flocker(FlockerVolumeSourceArgs.builder()
              .datasetName("string")
              .datasetUUID("string")
              .build())
            .gcePersistentDisk(GCEPersistentDiskVolumeSourceArgs.builder()
              .fsType("string")
              .partition(0)
              .pdName("string")
              .readOnly(true|false)
              .build())
            .gitRepo(GitRepoVolumeSourceArgs.builder()
              .directory("string")
              .repository("string")
              .revision("string")
              .build())
            .glusterfs(GlusterfsVolumeSourceArgs.builder()
              .endpoints("string")
              .path("string")
              .readOnly(true|false)
              .build())
            .hostPath(HostPathVolumeSourceArgs.builder()
              .path("string")
              .type("string")
              .build())
            .iscsi(ISCSIVolumeSourceArgs.builder()
              .chapAuthDiscovery(true|false)
              .chapAuthSession(true|false)
              .fsType("string")
              .initiatorName("string")
              .iqn("string")
              .iscsiInterface("string")
              .lun(0)
              .portals(List.of("string"))
              .readOnly(true|false)
              .secretRef(LocalObjectReferenceArgs.builder()
                .name("string")
                .build())
              .targetPortal("string")
              .build())
            .name("string")
            .nfs(NFSVolumeSourceArgs.builder()
              .path("string")
              .readOnly(true|false)
              .server("string")
              .build())
            .persistentVolumeClaim(PersistentVolumeClaimVolumeSourceArgs.builder()
              .claimName("string")
              .readOnly(true|false)
              .build())
            .photonPersistentDisk(PhotonPersistentDiskVolumeSourceArgs.builder()
              .fsType("string")
              .pdID("string")
              .build())
            .portworxVolume(PortworxVolumeSourceArgs.builder()
              .fsType("string")
              .readOnly(true|false)
              .volumeID("string")
              .build())
            .projected(ProjectedVolumeSourceArgs.builder()
              .defaultMode(0)
              .sources(List.of(
                VolumeProjectionArgs.builder()
                  .configMap(ConfigMapProjectionArgs.builder()
                    .items(List.of(
                      KeyToPathArgs.builder()
                        .key("string")
                        .mode(0)
                        .path("string")
                        .build()
                    ))
                    .name("string")
                    .optional(true|false)
                    .build())
                  .downwardAPI(DownwardAPIProjectionArgs.builder()
                    .items(List.of(
                      DownwardAPIVolumeFileArgs.builder()
                        .fieldRef(ObjectFieldSelectorArgs.builder()
                          .apiVersion("string")
                          .fieldPath("string")
                          .build())
                        .mode(0)
                        .path("string")
                        .resourceFieldRef(ResourceFieldSelectorArgs.builder()
                          .containerName("string")
                          .divisor("string")
                          .resource("string")
                          .build())
                        .build()
                    ))
                    .build())
                  .secret(SecretProjectionArgs.builder()
                    .items(List.of(
                      KeyToPathArgs.builder()
                        .key("string")
                        .mode(0)
                        .path("string")
                        .build()
                    ))
                    .name("string")
                    .optional(true|false)
                    .build())
                  .serviceAccountToken(ServiceAccountTokenProjectionArgs.builder()
                    .audience("string")
                    .expirationSeconds(0)
                    .path("string")
                    .build())
                  .build()
              ))
              .build())
            .quobyte(QuobyteVolumeSourceArgs.builder()
              .group("string")
              .readOnly(true|false)
              .registry("string")
              .tenant("string")
              .user("string")
              .volume("string")
              .build())
            .rbd(RBDVolumeSourceArgs.builder()
              .fsType("string")
              .image("string")
              .keyring("string")
              .monitors(List.of("string"))
              .pool("string")
              .readOnly(true|false)
              .secretRef(LocalObjectReferenceArgs.builder()
                .name("string")
                .build())
              .user("string")
              .build())
            .scaleIO(ScaleIOVolumeSourceArgs.builder()
              .fsType("string")
              .gateway("string")
              .protectionDomain("string")
              .readOnly(true|false)
              .secretRef(LocalObjectReferenceArgs.builder()
                .name("string")
                .build())
              .sslEnabled(true|false)
              .storageMode("string")
              .storagePool("string")
              .system("string")
              .volumeName("string")
              .build())
            .secret(SecretVolumeSourceArgs.builder()
              .defaultMode(0)
              .items(List.of(
                KeyToPathArgs.builder()
                  .key("string")
                  .mode(0)
                  .path("string")
                  .build()
              ))
              .optional(true|false)
              .secretName("string")
              .build())
            .storageos(StorageOSVolumeSourceArgs.builder()
              .fsType("string")
              .readOnly(true|false)
              .secretRef(LocalObjectReferenceArgs.builder()
                .name("string")
                .build())
              .volumeName("string")
              .volumeNamespace("string")
              .build())
            .vsphereVolume(VsphereVirtualDiskVolumeSourceArgs.builder()
              .fsType("string")
              .storagePolicyID("string")
              .storagePolicyName("string")
              .volumePath("string")
              .build())
            .build()
        ))
        .build())
      .build())
    .build())
  .build());
```

### YAML

```yaml
name: example
runtime: yaml
resources:
  deployment:
    type: kubernetes:apps/v1:Deployment
    properties:
      apiVersion: "string"
      kind: "string"
      metadata: 
        annotations: 
          "string": "string"
        clusterName: "string"
        creationTimestamp: "string"
        deletionGracePeriodSeconds: 0
        deletionTimestamp: "string"
        finalizers: ["string"]
        generateName: "string"
        generation: 0
        labels: 
          "string": "string"
        managedFields: [
          apiVersion: "string"
          fieldsType: "string"
          fieldsV1: 
          manager: "string"
          operation: "string"
          subresource: "string"
          time: "string"
        ]
        name: "string"
        namespace: "string"
        ownerReferences: [
          apiVersion: "string"
          blockOwnerDeletion: true|false
          controller: true|false
          kind: "string"
          name: "string"
          uid: "string"
        ]
        resourceVersion: "string"
        selfLink: "string"
        uid: "string"
      spec: 
        minReadySeconds: 0
        paused: true|false
        progressDeadlineSeconds: 0
        replicas: 0
        revisionHistoryLimit: 0
        selector: 
          matchExpressions: [
            key: "string"
            operator: "string"
            values: ["string"]
          ]
          matchLabels: 
            "string": "string"
        strategy: 
          rollingUpdate: 
            maxSurge: 0
            maxUnavailable: 0
          type: "string"
        template: 
          metadata: 
            annotations: 
              "string": "string"
            clusterName: "string"
            creationTimestamp: "string"
            deletionGracePeriodSeconds: 0
            deletionTimestamp: "string"
            finalizers: ["string"]
            generateName: "string"
            generation: 0
            labels: 
              "string": "string"
            managedFields: [
              apiVersion: "string"
              fieldsType: "string"
              fieldsV1: 
              manager: "string"
              operation: "string"
              subresource: "string"
              time: "string"
            ]
            name: "string"
            namespace: "string"
            ownerReferences: [
              apiVersion: "string"
              blockOwnerDeletion: true|false
              controller: true|false
              kind: "string"
              name: "string"
              uid: "string"
            ]
            resourceVersion: "string"
            selfLink: "string"
            uid: "string"
          spec: 
            activeDeadlineSeconds: 0
            affinity: 
              nodeAffinity: 
                preferredDuringSchedulingIgnoredDuringExecution: [
                  preference: 
                    matchExpressions: [
                      key: "string"
                      operator: "string"
                      values: ["string"]
                    ]
                    matchFields: [
                      key: "string"
                      operator: "string"
                      values: ["string"]
                    ]
                  weight: 0
                ]
                requiredDuringSchedulingIgnoredDuringExecution: 
                  nodeSelectorTerms: [
                    matchExpressions: [
                      key: "string"
                      operator: "string"
                      values: ["string"]
                    ]
                    matchFields: [
                      key: "string"
                      operator: "string"
                      values: ["string"]
                    ]
                  ]
              podAffinity: 
                preferredDuringSchedulingIgnoredDuringExecution: [
                  podAffinityTerm: 
                    labelSelector: 
                      matchExpressions: [
                        key: "string"
                        operator: "string"
                        values: ["string"]
                      ]
                      matchLabels: 
                        "string": "string"
                    namespaceSelector: 
                      matchExpressions: [
                        key: "string"
                        operator: "string"
                        values: ["string"]
                      ]
                      matchLabels: 
                        "string": "string"
                    namespaces: ["string"]
                    topologyKey: "string"
                  weight: 0
                ]
                requiredDuringSchedulingIgnoredDuringExecution: [
                  labelSelector: 
                    matchExpressions: [
                      key: "string"
                      operator: "string"
                      values: ["string"]
                    ]
                    matchLabels: 
                      "string": "string"
                  namespaceSelector: 
                    matchExpressions: [
                      key: "string"
                      operator: "string"
                      values: ["string"]
                    ]
                    matchLabels: 
                      "string": "string"
                  namespaces: ["string"]
                  topologyKey: "string"
                ]
              podAntiAffinity: 
                preferredDuringSchedulingIgnoredDuringExecution: [
                  podAffinityTerm: 
                    labelSelector: 
                      matchExpressions: [
                        key: "string"
                        operator: "string"
                        values: ["string"]
                      ]
                      matchLabels: 
                        "string": "string"
                    namespaceSelector: 
                      matchExpressions: [
                        key: "string"
                        operator: "string"
                        values: ["string"]
                      ]
                      matchLabels: 
                        "string": "string"
                    namespaces: ["string"]
                    topologyKey: "string"
                  weight: 0
                ]
                requiredDuringSchedulingIgnoredDuringExecution: [
                  labelSelector: 
                    matchExpressions: [
                      key: "string"
                      operator: "string"
                      values: ["string"]
                    ]
                    matchLabels: 
                      "string": "string"
                  namespaceSelector: 
                    matchExpressions: [
                      key: "string"
                      operator: "string"
                      values: ["string"]
                    ]
                    matchLabels: 
                      "string": "string"
                  namespaces: ["string"]
                  topologyKey: "string"
                ]
            automountServiceAccountToken: true|false
            containers: [
              args: ["string"]
              command: ["string"]
              env: [
                name: "string"
                value: "string"
                valueFrom: 
                  configMapKeyRef: 
                    key: "string"
                    name: "string"
                    optional: true|false
                  fieldRef: 
                    apiVersion: "string"
                    fieldPath: "string"
                  resourceFieldRef: 
                    containerName: "string"
                    divisor: "string"
                    resource: "string"
                  secretKeyRef: 
                    key: "string"
                    name: "string"
                    optional: true|false
              ]
              envFrom: [
                configMapRef: 
                  name: "string"
                  optional: true|false
                prefix: "string"
                secretRef: 
                  name: "string"
                  optional: true|false
              ]
              image: "string"
              imagePullPolicy: "string"
              lifecycle: 
                postStart: 
                  exec: 
                    command: ["string"]
                  httpGet: 
                    host: "string"
                    httpHeaders: [
                      name: "string"
                      value: "string"
                    ]
                    path: "string"
                    port: 0
                    scheme: "string"
                  tcpSocket: 
                    host: "string"
                    port: 0
                preStop: 
                  exec: 
                    command: ["string"]
                  httpGet: 
                    host: "string"
                    httpHeaders: [
                      name: "string"
                      value: "string"
                    ]
                    path: "string"
                    port: 0
                    scheme: "string"
                  tcpSocket: 
                    host: "string"
                    port: 0
              livenessProbe: 
                exec: 
                  command: ["string"]
                failureThreshold: 0
                httpGet: 
                  host: "string"
                  httpHeaders: [
                    name: "string"
                    value: "string"
                  ]
                  path: "string"
                  port: 0
                  scheme: "string"
                initialDelaySeconds: 0
                periodSeconds: 0
                successThreshold: 0
                tcpSocket: 
                  host: "string"
                  port: 0
                terminationGracePeriodSeconds: 0
                timeoutSeconds: 0
              name: "string"
              ports: [
                containerPort: 0
                hostIP: "string"
                hostPort: 0
                name: "string"
                protocol: "string"
              ]
              readinessProbe: 
                exec: 
                  command: ["string"]
                failureThreshold: 0
                httpGet: 
                  host: "string"
                  httpHeaders: [
                    name: "string"
                    value: "string"
                  ]
                  path: "string"
                  port: 0
                  scheme: "string"
                initialDelaySeconds: 0
                periodSeconds: 0
                successThreshold: 0
                tcpSocket: 
                  host: "string"
                  port: 0
                terminationGracePeriodSeconds: 0
                timeoutSeconds: 0
              resources: 
                limits: 
                  "string": "string"
                requests: 
                  "string": "string"
              securityContext: 
                allowPrivilegeEscalation: true|false
                capabilities: 
                  add: ["string"]
                  drop: ["string"]
                privileged: true|false
                procMount: "string"
                readOnlyRootFilesystem: true|false
                runAsGroup: 0
                runAsNonRoot: true|false
                runAsUser: 0
                seLinuxOptions: 
                  level: "string"
                  role: "string"
                  type: "string"
                  user: "string"
                seccompProfile: 
                  localhostProfile: "string"
                  type: "string"
                windowsOptions: 
                  gmsaCredentialSpec: "string"
                  gmsaCredentialSpecName: "string"
                  hostProcess: true|false
                  runAsUserName: "string"
              startupProbe: 
                exec: 
                  command: ["string"]
                failureThreshold: 0
                httpGet: 
                  host: "string"
                  httpHeaders: [
                    name: "string"
                    value: "string"
                  ]
                  path: "string"
                  port: 0
                  scheme: "string"
                initialDelaySeconds: 0
                periodSeconds: 0
                successThreshold: 0
                tcpSocket: 
                  host: "string"
                  port: 0
                terminationGracePeriodSeconds: 0
                timeoutSeconds: 0
              stdin: true|false
              stdinOnce: true|false
              terminationMessagePath: "string"
              terminationMessagePolicy: "string"
              tty: true|false
              volumeDevices: [
                devicePath: "string"
                name: "string"
              ]
              volumeMounts: [
                mountPath: "string"
                mountPropagation: "string"
                name: "string"
                readOnly: true|false
                subPath: "string"
                subPathExpr: "string"
              ]
              workingDir: "string"
            ]
            dnsConfig: 
              nameservers: ["string"]
              options: [
                name: "string"
                value: "string"
              ]
              searches: ["string"]
            dnsPolicy: "string"
            enableServiceLinks: true|false
            ephemeralContainers: [
              args: ["string"]
              command: ["string"]
              env: [
                name: "string"
                value: "string"
                valueFrom: 
                  configMapKeyRef: 
                    key: "string"
                    name: "string"
                    optional: true|false
                  fieldRef: 
                    apiVersion: "string"
                    fieldPath: "string"
                  resourceFieldRef: 
                    containerName: "string"
                    divisor: "string"
                    resource: "string"
                  secretKeyRef: 
                    key: "string"
                    name: "string"
                    optional: true|false
              ]
              envFrom: [
                configMapRef: 
                  name: "string"
                  optional: true|false
                prefix: "string"
                secretRef: 
                  name: "string"
                  optional: true|false
              ]
              image: "string"
              imagePullPolicy: "string"
              lifecycle: 
                postStart: 
                  exec: 
                    command: ["string"]
                  httpGet: 
                    host: "string"
                    httpHeaders: [
                      name: "string"
                      value: "string"
                    ]
                    path: "string"
                    port: 0
                    scheme: "string"
                  tcpSocket: 
                    host: "string"
                    port: 0
                preStop: 
                  exec: 
                    command: ["string"]
                  httpGet: 
                    host: "string"
                    httpHeaders: [
                      name: "string"
                      value: "string"
                    ]
                    path: "string"
                    port: 0
                    scheme: "string"
                  tcpSocket: 
                    host: "string"
                    port: 0
              livenessProbe: 
                exec: 
                  command: ["string"]
                failureThreshold: 0
                httpGet: 
                  host: "string"
                  httpHeaders: [
                    name: "string"
                    value: "string"
                  ]
                  path: "string"
                  port: 0
                  scheme: "string"
                initialDelaySeconds: 0
                periodSeconds: 0
                successThreshold: 0
                tcpSocket: 
                  host: "string"
                  port: 0
                terminationGracePeriodSeconds: 0
                timeoutSeconds: 0
              name: "string"
              ports: [
                containerPort: 0
                hostIP: "string"
                hostPort: 0
                name: "string"
                protocol: "string"
              ]
              readinessProbe: 
                exec: 
                  command: ["string"]
                failureThreshold: 0
                httpGet: 
                  host: "string"
                  httpHeaders: [
                    name: "string"
                    value: "string"
                  ]
                  path: "string"
                  port: 0
                  scheme: "string"
                initialDelaySeconds: 0
                periodSeconds: 0
                successThreshold: 0
                tcpSocket: 
                  host: "string"
                  port: 0
                terminationGracePeriodSeconds: 0
                timeoutSeconds: 0
              resources: 
                limits: 
                  "string": "string"
                requests: 
                  "string": "string"
              securityContext: 
                allowPrivilegeEscalation: true|false
                capabilities: 
                  add: ["string"]
                  drop: ["string"]
                privileged: true|false
                procMount: "string"
                readOnlyRootFilesystem: true|false
                runAsGroup: 0
                runAsNonRoot: true|false
                runAsUser: 0
                seLinuxOptions: 
                  level: "string"
                  role: "string"
                  type: "string"
                  user: "string"
                seccompProfile: 
                  localhostProfile: "string"
                  type: "string"
                windowsOptions: 
                  gmsaCredentialSpec: "string"
                  gmsaCredentialSpecName: "string"
                  hostProcess: true|false
                  runAsUserName: "string"
              startupProbe: 
                exec: 
                  command: ["string"]
                failureThreshold: 0
                httpGet: 
                  host: "string"
                  httpHeaders: [
                    name: "string"
                    value: "string"
                  ]
                  path: "string"
                  port: 0
                  scheme: "string"
                initialDelaySeconds: 0
                periodSeconds: 0
                successThreshold: 0
                tcpSocket: 
                  host: "string"
                  port: 0
                terminationGracePeriodSeconds: 0
                timeoutSeconds: 0
              stdin: true|false
              stdinOnce: true|false
              targetContainerName: "string"
              terminationMessagePath: "string"
              terminationMessagePolicy: "string"
              tty: true|false
              volumeDevices: [
                devicePath: "string"
                name: "string"
              ]
              volumeMounts: [
                mountPath: "string"
                mountPropagation: "string"
                name: "string"
                readOnly: true|false
                subPath: "string"
                subPathExpr: "string"
              ]
              workingDir: "string"
            ]
            hostAliases: [
              hostnames: ["string"]
              ip: "string"
            ]
            hostIPC: true|false
            hostNetwork: true|false
            hostPID: true|false
            hostname: "string"
            imagePullSecrets: [
              name: "string"
            ]
            initContainers: [
              args: ["string"]
              command: ["string"]
              env: [
                name: "string"
                value: "string"
                valueFrom: 
                  configMapKeyRef: 
                    key: "string"
                    name: "string"
                    optional: true|false
                  fieldRef: 
                    apiVersion: "string"
                    fieldPath: "string"
                  resourceFieldRef: 
                    containerName: "string"
                    divisor: "string"
                    resource: "string"
                  secretKeyRef: 
                    key: "string"
                    name: "string"
                    optional: true|false
              ]
              envFrom: [
                configMapRef: 
                  name: "string"
                  optional: true|false
                prefix: "string"
                secretRef: 
                  name: "string"
                  optional: true|false
              ]
              image: "string"
              imagePullPolicy: "string"
              lifecycle: 
                postStart: 
                  exec: 
                    command: ["string"]
                  httpGet: 
                    host: "string"
                    httpHeaders: [
                      name: "string"
                      value: "string"
                    ]
                    path: "string"
                    port: 0
                    scheme: "string"
                  tcpSocket: 
                    host: "string"
                    port: 0
                preStop: 
                  exec: 
                    command: ["string"]
                  httpGet: 
                    host: "string"
                    httpHeaders: [
                      name: "string"
                      value: "string"
                    ]
                    path: "string"
                    port: 0
                    scheme: "string"
                  tcpSocket: 
                    host: "string"
                    port: 0
              livenessProbe: 
                exec: 
                  command: ["string"]
                failureThreshold: 0
                httpGet: 
                  host: "string"
                  httpHeaders: [
                    name: "string"
                    value: "string"
                  ]
                  path: "string"
                  port: 0
                  scheme: "string"
                initialDelaySeconds: 0
                periodSeconds: 0
                successThreshold: 0
                tcpSocket: 
                  host: "string"
                  port: 0
                terminationGracePeriodSeconds: 0
                timeoutSeconds: 0
              name: "string"
              ports: [
                containerPort: 0
                hostIP: "string"
                hostPort: 0
                name: "string"
                protocol: "string"
              ]
              readinessProbe: 
                exec: 
                  command: ["string"]
                failureThreshold: 0
                httpGet: 
                  host: "string"
                  httpHeaders: [
                    name: "string"
                    value: "string"
                  ]
                  path: "string"
                  port: 0
                  scheme: "string"
                initialDelaySeconds: 0
                periodSeconds: 0
                successThreshold: 0
                tcpSocket: 
                  host: "string"
                  port: 0
                terminationGracePeriodSeconds: 0
                timeoutSeconds: 0
              resources: 
                limits: 
                  "string": "string"
                requests: 
                  "string": "string"
              securityContext: 
                allowPrivilegeEscalation: true|false
                capabilities: 
                  add: ["string"]
                  drop: ["string"]
                privileged: true|false
                procMount: "string"
                readOnlyRootFilesystem: true|false
                runAsGroup: 0
                runAsNonRoot: true|false
                runAsUser: 0
                seLinuxOptions: 
                  level: "string"
                  role: "string"
                  type: "string"
                  user: "string"
                seccompProfile: 
                  localhostProfile: "string"
                  type: "string"
                windowsOptions: 
                  gmsaCredentialSpec: "string"
                  gmsaCredentialSpecName: "string"
                  hostProcess: true|false
                  runAsUserName: "string"
              startupProbe: 
                exec: 
                  command: ["string"]
                failureThreshold: 0
                httpGet: 
                  host: "string"
                  httpHeaders: [
                    name: "string"
                    value: "string"
                  ]
                  path: "string"
                  port: 0
                  scheme: "string"
                initialDelaySeconds: 0
                periodSeconds: 0
                successThreshold: 0
                tcpSocket: 
                  host: "string"
                  port: 0
                terminationGracePeriodSeconds: 0
                timeoutSeconds: 0
              stdin: true|false
              stdinOnce: true|false
              terminationMessagePath: "string"
              terminationMessagePolicy: "string"
              tty: true|false
              volumeDevices: [
                devicePath: "string"
                name: "string"
              ]
              volumeMounts: [
                mountPath: "string"
                mountPropagation: "string"
                name: "string"
                readOnly: true|false
                subPath: "string"
                subPathExpr: "string"
              ]
              workingDir: "string"
            ]
            nodeName: "string"
            nodeSelector: 
              "string": "string"
            overhead: 
              "string": "string"
            preemptionPolicy: "string"
            priority: 0
            priorityClassName: "string"
            readinessGates: [
              conditionType: "string"
            ]
            restartPolicy: "string"
            runtimeClassName: "string"
            schedulerName: "string"
            securityContext: 
              fsGroup: 0
              fsGroupChangePolicy: "string"
              runAsGroup: 0
              runAsNonRoot: true|false
              runAsUser: 0
              seLinuxOptions: 
                level: "string"
                role: "string"
                type: "string"
                user: "string"
              seccompProfile: 
                localhostProfile: "string"
                type: "string"
              supplementalGroups: [0]
              sysctls: [
                name: "string"
                value: "string"
              ]
              windowsOptions: 
                gmsaCredentialSpec: "string"
                gmsaCredentialSpecName: "string"
                hostProcess: true|false
                runAsUserName: "string"
            serviceAccount: "string"
            serviceAccountName: "string"
            setHostnameAsFQDN: true|false
            shareProcessNamespace: true|false
            subdomain: "string"
            terminationGracePeriodSeconds: 0
            tolerations: [
              effect: "string"
              key: "string"
              operator: "string"
              tolerationSeconds: 0
              value: "string"
            ]
            topologySpreadConstraints: [
              labelSelector: 
                matchExpressions: [
                  key: "string"
                  operator: "string"
                  values: ["string"]
                ]
                matchLabels: 
                  "string": "string"
              maxSkew: 0
              topologyKey: "string"
              whenUnsatisfiable: "string"
            ]
            volumes: [
              awsElasticBlockStore: 
                fsType: "string"
                partition: 0
                readOnly: true|false
                volumeID: "string"
              azureDisk: 
                cachingMode: "string"
                diskName: "string"
                diskURI: "string"
                fsType: "string"
                kind: "string"
                readOnly: true|false
              azureFile: 
                readOnly: true|false
                secretName: "string"
                shareName: "string"
              cephfs: 
                monitors: ["string"]
                path: "string"
                readOnly: true|false
                secretFile: "string"
                secretRef: 
                  name: "string"
                user: "string"
              cinder: 
                fsType: "string"
                readOnly: true|false
                secretRef: 
                  name: "string"
                volumeID: "string"
              configMap: 
                defaultMode: 0
                items: [
                  key: "string"
                  mode: 0
                  path: "string"
                ]
                name: "string"
                optional: true|false
              csi: 
                driver: "string"
                fsType: "string"
                nodePublishSecretRef: 
                  name: "string"
                readOnly: true|false
                volumeAttributes: 
                  "string": "string"
              downwardAPI: 
                defaultMode: 0
                items: [
                  fieldRef: 
                    apiVersion: "string"
                    fieldPath: "string"
                  mode: 0
                  path: "string"
                  resourceFieldRef: 
                    containerName: "string"
                    divisor: "string"
                    resource: "string"
                ]
              emptyDir: 
                medium: "string"
                sizeLimit: "string"
              ephemeral: 
                readOnly: true|false
                volumeClaimTemplate: 
                  metadata: 
                    annotations: 
                      "string": "string"
                    clusterName: "string"
                    creationTimestamp: "string"
                    deletionGracePeriodSeconds: 0
                    deletionTimestamp: "string"
                    finalizers: ["string"]
                    generateName: "string"
                    generation: 0
                    labels: 
                      "string": "string"
                    managedFields: [
                      apiVersion: "string"
                      fieldsType: "string"
                      fieldsV1: 
                      manager: "string"
                      operation: "string"
                      subresource: "string"
                      time: "string"
                    ]
                    name: "string"
                    namespace: "string"
                    ownerReferences: [
                      apiVersion: "string"
                      blockOwnerDeletion: true|false
                      controller: true|false
                      kind: "string"
                      name: "string"
                      uid: "string"
                    ]
                    resourceVersion: "string"
                    selfLink: "string"
                    uid: "string"
                  spec: 
                    accessModes: ["string"]
                    dataSource: 
                      apiGroup: "string"
                      kind: "string"
                      name: "string"
                    dataSourceRef: 
                      apiGroup: "string"
                      kind: "string"
                      name: "string"
                    resources: 
                      limits: 
                        "string": "string"
                      requests: 
                        "string": "string"
                    selector: 
                      matchExpressions: [
                        key: "string"
                        operator: "string"
                        values: ["string"]
                      ]
                      matchLabels: 
                        "string": "string"
                    storageClassName: "string"
                    volumeMode: "string"
                    volumeName: "string"
              fc: 
                fsType: "string"
                lun: 0
                readOnly: true|false
                targetWWNs: ["string"]
                wwids: ["string"]
              flexVolume: 
                driver: "string"
                fsType: "string"
                options: 
                  "string": "string"
                readOnly: true|false
                secretRef: 
                  name: "string"
              flocker: 
                datasetName: "string"
                datasetUUID: "string"
              gcePersistentDisk: 
                fsType: "string"
                partition: 0
                pdName: "string"
                readOnly: true|false
              gitRepo: 
                directory: "string"
                repository: "string"
                revision: "string"
              glusterfs: 
                endpoints: "string"
                path: "string"
                readOnly: true|false
              hostPath: 
                path: "string"
                type: "string"
              iscsi: 
                chapAuthDiscovery: true|false
                chapAuthSession: true|false
                fsType: "string"
                initiatorName: "string"
                iqn: "string"
                iscsiInterface: "string"
                lun: 0
                portals: ["string"]
                readOnly: true|false
                secretRef: 
                  name: "string"
                targetPortal: "string"
              name: "string"
              nfs: 
                path: "string"
                readOnly: true|false
                server: "string"
              persistentVolumeClaim: 
                claimName: "string"
                readOnly: true|false
              photonPersistentDisk: 
                fsType: "string"
                pdID: "string"
              portworxVolume: 
                fsType: "string"
                readOnly: true|false
                volumeID: "string"
              projected: 
                defaultMode: 0
                sources: [
                  configMap: 
                    items: [
                      key: "string"
                      mode: 0
                      path: "string"
                    ]
                    name: "string"
                    optional: true|false
                  downwardAPI: 
                    items: [
                      fieldRef: 
                        apiVersion: "string"
                        fieldPath: "string"
                      mode: 0
                      path: "string"
                      resourceFieldRef: 
                        containerName: "string"
                        divisor: "string"
                        resource: "string"
                    ]
                  secret: 
                    items: [
                      key: "string"
                      mode: 0
                      path: "string"
                    ]
                    name: "string"
                    optional: true|false
                  serviceAccountToken: 
                    audience: "string"
                    expirationSeconds: 0
                    path: "string"
                ]
              quobyte: 
                group: "string"
                readOnly: true|false
                registry: "string"
                tenant: "string"
                user: "string"
                volume: "string"
              rbd: 
                fsType: "string"
                image: "string"
                keyring: "string"
                monitors: ["string"]
                pool: "string"
                readOnly: true|false
                secretRef: 
                  name: "string"
                user: "string"
              scaleIO: 
                fsType: "string"
                gateway: "string"
                protectionDomain: "string"
                readOnly: true|false
                secretRef: 
                  name: "string"
                sslEnabled: true|false
                storageMode: "string"
                storagePool: "string"
                system: "string"
                volumeName: "string"
              secret: 
                defaultMode: 0
                items: [
                  key: "string"
                  mode: 0
                  path: "string"
                ]
                optional: true|false
                secretName: "string"
              storageos: 
                fsType: "string"
                readOnly: true|false
                secretRef: 
                  name: "string"
                volumeName: "string"
                volumeNamespace: "string"
              vsphereVolume: 
                fsType: "string"
                storagePolicyID: "string"
                storagePolicyName: "string"
                volumePath: "string"
            ]
```

