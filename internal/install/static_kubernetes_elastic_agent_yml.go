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
  template:
    metadata:
      labels:
        app: kind-fleet-agent-clusterscope
        group: fleet
    spec:
      serviceAccountName: kind-fleet-agent
      containers:
        - name: kind-fleet-agent-clusterscope
          # Temporary workaround for: https://github.com/elastic/beats/issues/24310
          image: docker.elastic.co/beats/elastic-agent@sha256:6182d3ebb975965c4501b551dfed2ddc6b7f47c05187884c62fe6192f7df4625
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
