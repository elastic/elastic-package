// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

const kubernetesDeployerElasticAgentVersion = "7.10.0-SNAPSHOT"

const kubernetesDeployerElasticAgentYml = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agent-ingest-management-clusterscope
  namespace: kube-system
  labels:
    app: agent-ingest-management-clusterscope
    group: ingest-management
spec:
  selector:
    matchLabels:
      app: agent-ingest-management-clusterscope
  template:
    metadata:
      labels:
        app: agent-ingest-management-clusterscope
        group: ingest-management
    spec:
      serviceAccountName: agent-ingest-management
      containers:
        - name: agent-ingest-management-clusterscope
          image: docker.elastic.co/beats/elastic-agent:` + kubernetesDeployerElasticAgentVersion + `
          env:
            - name: FLEET_ENROLL
              value: "1"
            - name: FLEET_ENROLL_INSECURE
              value: "1"
            - name: KIBANA_HOST
              value: "http://kibana:5601"
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          securityContext:
            runAsUser: 0
          resources:
            limits:
              memory: 200Mi
            requests:
              cpu: 100m
              memory: 100Mi
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: agent-ingest-management-clusterscope
  namespace: kube-system
  labels:
    group: ingest-management
data:
  elastic-agent.yml: |-
    management:
      mode: "fleet"
    grpc:
      # listen address for the GRPC server that spawned processes connect back to.
      address: localhost
      # port for the GRPC server that spawned processes connect back to.
      port: 6789
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: agent-ingest-management
subjects:
  - kind: ServiceAccount
    name: agent-ingest-management
    namespace: kube-system
roleRef:
  kind: ClusterRole
  name: agent-ingest-management
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: agent-ingest-management
  labels:
    k8s-app: agent-ingest-management
rules:
  - apiGroups: [""]
    resources:
      - nodes
      - namespaces
      - events
      - pods
      - secrets
    verbs: ["get", "list", "watch"]
  - apiGroups: ["extensions"]
    resources:
      - replicasets
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources:
      - statefulsets
      - deployments
      - replicasets
    verbs: ["get", "list", "watch"]
  - apiGroups:
      - ""
    resources:
      - nodes/stats
    verbs:
      - get
  # required for apiserver
  - nonResourceURLs:
      - "/metrics"
    verbs:
      - get
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: agent-ingest-management
  namespace: kube-system
  labels:
    k8s-app: agent-ingest-management
---
`
