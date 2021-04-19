// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

const kubernetesDeployerElasticAgentYml = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kind-fleet-agent-clusterscope
  namespace: kube-system
  labels:
    app: kind-fleet-agent-clusterscope
    group: fleet
spec:
  selector:
    matchLabels:
      app: kind-fleet-agent-clusterscope
  replicas: 1
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    metadata:
      labels:
        app: kind-fleet-agent-clusterscope
        group: fleet
    spec:
      serviceAccountName: kind-fleet-agent
      containers:
        - name: kind-fleet-agent-clusterscope
          image: {{ ELASTIC_AGENT_IMAGE_REF }}
          env:
            - name: FLEET_ENROLL
              value: "1"
            - name: FLEET_INSECURE
              value: "1"
            - name: FLEET_URL
              value: "http://fleet-server:8220"
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          securityContext:
            runAsUser: 0
          resources:
            limits:
              memory: 400Mi
            requests:
              cpu: 200m
              memory: 400Mi
          startupProbe:
            exec:
              command:
              - sh
              - -c
              - grep "Agent is starting" -r . --include=elastic-agent-json.log
  
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kind-fleet-agent-clusterscope
  namespace: kube-system
  labels:
    group: fleet
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
  name: kind-fleet-agent
subjects:
  - kind: ServiceAccount
    name: kind-fleet-agent
    namespace: kube-system
roleRef:
  kind: ClusterRole
  name: kind-fleet-agent
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kind-fleet-agent
  labels:
    k8s-app: kind-fleet-agent
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
  name: kind-fleet-agent
  namespace: kube-system
  labels:
    k8s-app: kind-fleet-agent
---
`
