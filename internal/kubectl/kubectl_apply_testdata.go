// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kubectl

const singleDefinitionFile = `apiVersion: v1
kind: Service
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"v1","kind":"Service","metadata":{"annotations":{},"labels":{"app.kubernetes.io/name":"kube-state-metrics","app.kubernetes.io/version":"2.0.0-rc.1"},"name":"kube-state-metrics","namespace":"kube-system"},"spec":{"clusterIP":"None","ports":[{"name":"http-metrics","port":8080,"targetPort":"http-metrics"},{"name":"telemetry","port":8081,"targetPort":"telemetry"}],"selector":{"app.kubernetes.io/name":"kube-state-metrics"}}}
  creationTimestamp: "2021-04-13T10:50:22Z"
  labels:
    app.kubernetes.io/name: kube-state-metrics
    app.kubernetes.io/version: 2.0.0-rc.1
  managedFields:
  - apiVersion: v1
    fieldsType: FieldsV1
    fieldsV1:
      f:metadata:
        f:annotations:
          .: {}
          f:kubectl.kubernetes.io/last-applied-configuration: {}
        f:labels:
          .: {}
          f:app.kubernetes.io/name: {}
          f:app.kubernetes.io/version: {}
      f:spec:
        f:clusterIP: {}
        f:ports:
          .: {}
          k:{"port":8080,"protocol":"TCP"}:
            .: {}
            f:name: {}
            f:port: {}
            f:protocol: {}
            f:targetPort: {}
          k:{"port":8081,"protocol":"TCP"}:
            .: {}
            f:name: {}
            f:port: {}
            f:protocol: {}
            f:targetPort: {}
        f:selector:
          .: {}
          f:app.kubernetes.io/name: {}
        f:sessionAffinity: {}
        f:type: {}
    manager: kubectl
    operation: Update
    time: "2021-04-13T10:50:22Z"
  name: kube-state-metrics
  namespace: kube-system
  resourceVersion: "630"
  uid: 12a3a777-97bf-476d-9a96-4c9265bdb7d9
spec:
  clusterIP: None
  clusterIPs:
  - None
  ports:
  - name: http-metrics
    port: 8080
    protocol: TCP
    targetPort: http-metrics
  - name: telemetry
    port: 8081
    protocol: TCP
    targetPort: telemetry
  selector:
    app.kubernetes.io/name: kube-state-metrics
  sessionAffinity: None
  type: ClusterIP
status:
  loadBalancer: {}
`

const multipleDefinitionFiles = `apiVersion: v1
items:
- apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRoleBinding
  metadata:
    annotations:
      kubectl.kubernetes.io/last-applied-configuration: |
        {"apiVersion":"rbac.authorization.k8s.io/v1","kind":"ClusterRoleBinding","metadata":{"annotations":{},"labels":{"app.kubernetes.io/name":"kube-state-metrics","app.kubernetes.io/version":"2.0.0-rc.1"},"name":"kube-state-metrics"},"roleRef":{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"kube-state-metrics"},"subjects":[{"kind":"ServiceAccount","name":"kube-state-metrics","namespace":"kube-system"}]}
    creationTimestamp: "2021-04-13T10:50:48Z"
    labels:
      app.kubernetes.io/name: kube-state-metrics
      app.kubernetes.io/version: 2.0.0-rc.1
    managedFields:
    - apiVersion: rbac.authorization.k8s.io/v1
      fieldsType: FieldsV1
      fieldsV1:
        f:metadata:
          f:annotations:
            .: {}
            f:kubectl.kubernetes.io/last-applied-configuration: {}
          f:labels:
            .: {}
            f:app.kubernetes.io/name: {}
            f:app.kubernetes.io/version: {}
        f:roleRef:
          f:apiGroup: {}
          f:kind: {}
          f:name: {}
        f:subjects: {}
      manager: kubectl
      operation: Update
      time: "2021-04-13T10:50:48Z"
    name: kube-state-metrics
    resourceVersion: "682"
    uid: 5cf12f32-0253-42ee-8b35-89123e35da8d
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: kube-state-metrics
  subjects:
  - kind: ServiceAccount
    name: kube-state-metrics
    namespace: kube-system
- apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    annotations:
      kubectl.kubernetes.io/last-applied-configuration: |
        {"apiVersion":"rbac.authorization.k8s.io/v1","kind":"ClusterRole","metadata":{"annotations":{},"labels":{"app.kubernetes.io/name":"kube-state-metrics","app.kubernetes.io/version":"2.0.0-rc.1"},"name":"kube-state-metrics"},"rules":[{"apiGroups":[""],"resources":["configmaps","secrets","nodes","pods","services","resourcequotas","replicationcontrollers","limitranges","persistentvolumeclaims","persistentvolumes","namespaces","endpoints"],"verbs":["list","watch"]},{"apiGroups":["apps"],"resources":["statefulsets","daemonsets","deployments","replicasets"],"verbs":["list","watch"]},{"apiGroups":["batch"],"resources":["cronjobs","jobs"],"verbs":["list","watch"]},{"apiGroups":["autoscaling"],"resources":["horizontalpodautoscalers"],"verbs":["list","watch"]},{"apiGroups":["authentication.k8s.io"],"resources":["tokenreviews"],"verbs":["create"]},{"apiGroups":["authorization.k8s.io"],"resources":["subjectaccessreviews"],"verbs":["create"]},{"apiGroups":["policy"],"resources":["poddisruptionbudgets"],"verbs":["list","watch"]},{"apiGroups":["certificates.k8s.io"],"resources":["certificatesigningrequests"],"verbs":["list","watch"]},{"apiGroups":["storage.k8s.io"],"resources":["storageclasses","volumeattachments"],"verbs":["list","watch"]},{"apiGroups":["admissionregistration.k8s.io"],"resources":["mutatingwebhookconfigurations","validatingwebhookconfigurations"],"verbs":["list","watch"]},{"apiGroups":["networking.k8s.io"],"resources":["networkpolicies","ingresses"],"verbs":["list","watch"]},{"apiGroups":["coordination.k8s.io"],"resources":["leases"],"verbs":["list","watch"]}]}
    creationTimestamp: "2021-04-13T10:50:48Z"
    labels:
      app.kubernetes.io/name: kube-state-metrics
      app.kubernetes.io/version: 2.0.0-rc.1
    managedFields:
    - apiVersion: rbac.authorization.k8s.io/v1
      fieldsType: FieldsV1
      fieldsV1:
        f:metadata:
          f:annotations:
            .: {}
            f:kubectl.kubernetes.io/last-applied-configuration: {}
          f:labels:
            .: {}
            f:app.kubernetes.io/name: {}
            f:app.kubernetes.io/version: {}
        f:rules: {}
      manager: kubectl
      operation: Update
      time: "2021-04-13T10:50:48Z"
    name: kube-state-metrics
    resourceVersion: "683"
    uid: 5a310f16-bf5d-44d2-8588-2029104172af
  rules:
  - apiGroups:
    - ""
    resources:
    - configmaps
    - secrets
    - nodes
    - pods
    - services
    - resourcequotas
    - replicationcontrollers
    - limitranges
    - persistentvolumeclaims
    - persistentvolumes
    - namespaces
    - endpoints
    verbs:
    - list
    - watch
  - apiGroups:
    - apps
    resources:
    - statefulsets
    - daemonsets
    - deployments
    - replicasets
    verbs:
    - list
    - watch
  - apiGroups:
    - batch
    resources:
    - cronjobs
    - jobs
    verbs:
    - list
    - watch
  - apiGroups:
    - autoscaling
    resources:
    - horizontalpodautoscalers
    verbs:
    - list
    - watch
  - apiGroups:
    - authentication.k8s.io
    resources:
    - tokenreviews
    verbs:
    - create
  - apiGroups:
    - authorization.k8s.io
    resources:
    - subjectaccessreviews
    verbs:
    - create
  - apiGroups:
    - policy
    resources:
    - poddisruptionbudgets
    verbs:
    - list
    - watch
  - apiGroups:
    - certificates.k8s.io
    resources:
    - certificatesigningrequests
    verbs:
    - list
    - watch
  - apiGroups:
    - storage.k8s.io
    resources:
    - storageclasses
    - volumeattachments
    verbs:
    - list
    - watch
  - apiGroups:
    - admissionregistration.k8s.io
    resources:
    - mutatingwebhookconfigurations
    - validatingwebhookconfigurations
    verbs:
    - list
    - watch
  - apiGroups:
    - networking.k8s.io
    resources:
    - networkpolicies
    - ingresses
    verbs:
    - list
    - watch
  - apiGroups:
    - coordination.k8s.io
    resources:
    - leases
    verbs:
    - list
    - watch
- apiVersion: apps/v1
  kind: Deployment
  metadata:
    annotations:
      deployment.kubernetes.io/revision: "1"
      kubectl.kubernetes.io/last-applied-configuration: |
        {"apiVersion":"apps/v1","kind":"Deployment","metadata":{"annotations":{},"labels":{"app.kubernetes.io/name":"kube-state-metrics","app.kubernetes.io/version":"2.0.0-rc.1"},"name":"kube-state-metrics","namespace":"kube-system"},"spec":{"replicas":1,"selector":{"matchLabels":{"app.kubernetes.io/name":"kube-state-metrics"}},"template":{"metadata":{"labels":{"app.kubernetes.io/name":"kube-state-metrics","app.kubernetes.io/version":"2.0.0-rc.1"}},"spec":{"containers":[{"image":"k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0-rc.1","livenessProbe":{"httpGet":{"path":"/healthz","port":8080},"initialDelaySeconds":5,"timeoutSeconds":5},"name":"kube-state-metrics","ports":[{"containerPort":8080,"name":"http-metrics"},{"containerPort":8081,"name":"telemetry"}],"readinessProbe":{"httpGet":{"path":"/","port":8081},"initialDelaySeconds":5,"timeoutSeconds":5},"securityContext":{"runAsUser":65534}}],"nodeSelector":{"kubernetes.io/os":"linux"},"serviceAccountName":"kube-state-metrics"}}}}
    creationTimestamp: "2021-04-13T10:50:48Z"
    generation: 1
    labels:
      app.kubernetes.io/name: kube-state-metrics
      app.kubernetes.io/version: 2.0.0-rc.1
    managedFields:
    - apiVersion: apps/v1
      fieldsType: FieldsV1
      fieldsV1:
        f:metadata:
          f:annotations:
            .: {}
            f:kubectl.kubernetes.io/last-applied-configuration: {}
          f:labels:
            .: {}
            f:app.kubernetes.io/name: {}
            f:app.kubernetes.io/version: {}
        f:spec:
          f:progressDeadlineSeconds: {}
          f:replicas: {}
          f:revisionHistoryLimit: {}
          f:selector: {}
          f:strategy:
            f:rollingUpdate:
              .: {}
              f:maxSurge: {}
              f:maxUnavailable: {}
            f:type: {}
          f:template:
            f:metadata:
              f:labels:
                .: {}
                f:app.kubernetes.io/name: {}
                f:app.kubernetes.io/version: {}
            f:spec:
              f:containers:
                k:{"name":"kube-state-metrics"}:
                  .: {}
                  f:image: {}
                  f:imagePullPolicy: {}
                  f:livenessProbe:
                    .: {}
                    f:failureThreshold: {}
                    f:httpGet:
                      .: {}
                      f:path: {}
                      f:port: {}
                      f:scheme: {}
                    f:initialDelaySeconds: {}
                    f:periodSeconds: {}
                    f:successThreshold: {}
                    f:timeoutSeconds: {}
                  f:name: {}
                  f:ports:
                    .: {}
                    k:{"containerPort":8080,"protocol":"TCP"}:
                      .: {}
                      f:containerPort: {}
                      f:name: {}
                      f:protocol: {}
                    k:{"containerPort":8081,"protocol":"TCP"}:
                      .: {}
                      f:containerPort: {}
                      f:name: {}
                      f:protocol: {}
                  f:readinessProbe:
                    .: {}
                    f:failureThreshold: {}
                    f:httpGet:
                      .: {}
                      f:path: {}
                      f:port: {}
                      f:scheme: {}
                    f:initialDelaySeconds: {}
                    f:periodSeconds: {}
                    f:successThreshold: {}
                    f:timeoutSeconds: {}
                  f:resources: {}
                  f:securityContext:
                    .: {}
                    f:runAsUser: {}
                  f:terminationMessagePath: {}
                  f:terminationMessagePolicy: {}
              f:dnsPolicy: {}
              f:nodeSelector:
                .: {}
                f:kubernetes.io/os: {}
              f:restartPolicy: {}
              f:schedulerName: {}
              f:securityContext: {}
              f:serviceAccount: {}
              f:serviceAccountName: {}
              f:terminationGracePeriodSeconds: {}
      manager: kubectl
      operation: Update
      time: "2021-04-13T10:50:48Z"
    - apiVersion: apps/v1
      fieldsType: FieldsV1
      fieldsV1:
        f:metadata:
          f:annotations:
            f:deployment.kubernetes.io/revision: {}
        f:status:
          f:availableReplicas: {}
          f:conditions:
            .: {}
            k:{"type":"Available"}:
              .: {}
              f:lastTransitionTime: {}
              f:lastUpdateTime: {}
              f:message: {}
              f:reason: {}
              f:status: {}
              f:type: {}
            k:{"type":"Progressing"}:
              .: {}
              f:lastTransitionTime: {}
              f:lastUpdateTime: {}
              f:message: {}
              f:reason: {}
              f:status: {}
              f:type: {}
          f:observedGeneration: {}
          f:readyReplicas: {}
          f:replicas: {}
          f:updatedReplicas: {}
      manager: kube-controller-manager
      operation: Update
      time: "2021-04-13T10:51:05Z"
    name: kube-state-metrics
    namespace: kube-system
    resourceVersion: "742"
    uid: 2cc60038-13e4-45a7-a168-f443efb5c9d8
  spec:
    progressDeadlineSeconds: 600
    replicas: 1
    revisionHistoryLimit: 10
    selector:
      matchLabels:
        app.kubernetes.io/name: kube-state-metrics
    strategy:
      rollingUpdate:
        maxSurge: 25%
        maxUnavailable: 25%
      type: RollingUpdate
    template:
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/name: kube-state-metrics
          app.kubernetes.io/version: 2.0.0-rc.1
      spec:
        containers:
        - image: k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0-rc.1
          imagePullPolicy: IfNotPresent
          livenessProbe:
            failureThreshold: 3
            httpGet:
              path: /healthz
              port: 8080
              scheme: HTTP
            initialDelaySeconds: 5
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 5
          name: kube-state-metrics
          ports:
          - containerPort: 8080
            name: http-metrics
            protocol: TCP
          - containerPort: 8081
            name: telemetry
            protocol: TCP
          readinessProbe:
            failureThreshold: 3
            httpGet:
              path: /
              port: 8081
              scheme: HTTP
            initialDelaySeconds: 5
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 5
          resources: {}
          securityContext:
            runAsUser: 65534
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
        dnsPolicy: ClusterFirst
        nodeSelector:
          kubernetes.io/os: linux
        restartPolicy: Always
        schedulerName: default-scheduler
        securityContext: {}
        serviceAccount: kube-state-metrics
        serviceAccountName: kube-state-metrics
        terminationGracePeriodSeconds: 30
  status:
    availableReplicas: 1
    conditions:
    - lastTransitionTime: "2021-04-13T10:51:05Z"
      lastUpdateTime: "2021-04-13T10:51:05Z"
      message: Deployment has minimum availability.
      reason: MinimumReplicasAvailable
      status: "True"
      type: Available
    - lastTransitionTime: "2021-04-13T10:50:48Z"
      lastUpdateTime: "2021-04-13T10:51:05Z"
      message: ReplicaSet "kube-state-metrics-57d65f448" has successfully progressed.
      reason: NewReplicaSetAvailable
      status: "True"
      type: Progressing
    observedGeneration: 1
    readyReplicas: 1
    replicas: 1
    updatedReplicas: 1
- apiVersion: v1
  kind: ServiceAccount
  metadata:
    annotations:
      kubectl.kubernetes.io/last-applied-configuration: |
        {"apiVersion":"v1","kind":"ServiceAccount","metadata":{"annotations":{},"labels":{"app.kubernetes.io/name":"kube-state-metrics","app.kubernetes.io/version":"2.0.0-rc.1"},"name":"kube-state-metrics","namespace":"kube-system"}}
    creationTimestamp: "2021-04-13T10:50:48Z"
    labels:
      app.kubernetes.io/name: kube-state-metrics
      app.kubernetes.io/version: 2.0.0-rc.1
    managedFields:
    - apiVersion: v1
      fieldsType: FieldsV1
      fieldsV1:
        f:secrets:
          .: {}
          k:{"name":"kube-state-metrics-token-9ptlg"}:
            .: {}
            f:name: {}
      manager: kube-controller-manager
      operation: Update
      time: "2021-04-13T10:50:48Z"
    - apiVersion: v1
      fieldsType: FieldsV1
      fieldsV1:
        f:metadata:
          f:annotations:
            .: {}
            f:kubectl.kubernetes.io/last-applied-configuration: {}
          f:labels:
            .: {}
            f:app.kubernetes.io/name: {}
            f:app.kubernetes.io/version: {}
      manager: kubectl
      operation: Update
      time: "2021-04-13T10:50:48Z"
    name: kube-state-metrics
    namespace: kube-system
    resourceVersion: "691"
    uid: 475391da-2c16-41f0-b909-43368a66f87b
  secrets:
  - name: kube-state-metrics-token-9ptlg
- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      kubectl.kubernetes.io/last-applied-configuration: |
        {"apiVersion":"v1","kind":"Service","metadata":{"annotations":{},"labels":{"app.kubernetes.io/name":"kube-state-metrics","app.kubernetes.io/version":"2.0.0-rc.1"},"name":"kube-state-metrics","namespace":"kube-system"},"spec":{"clusterIP":"None","ports":[{"name":"http-metrics","port":8080,"targetPort":"http-metrics"},{"name":"telemetry","port":8081,"targetPort":"telemetry"}],"selector":{"app.kubernetes.io/name":"kube-state-metrics"}}}
    creationTimestamp: "2021-04-13T10:50:22Z"
    labels:
      app.kubernetes.io/name: kube-state-metrics
      app.kubernetes.io/version: 2.0.0-rc.1
    managedFields:
    - apiVersion: v1
      fieldsType: FieldsV1
      fieldsV1:
        f:metadata:
          f:annotations:
            .: {}
            f:kubectl.kubernetes.io/last-applied-configuration: {}
          f:labels:
            .: {}
            f:app.kubernetes.io/name: {}
            f:app.kubernetes.io/version: {}
        f:spec:
          f:clusterIP: {}
          f:ports:
            .: {}
            k:{"port":8080,"protocol":"TCP"}:
              .: {}
              f:name: {}
              f:port: {}
              f:protocol: {}
              f:targetPort: {}
            k:{"port":8081,"protocol":"TCP"}:
              .: {}
              f:name: {}
              f:port: {}
              f:protocol: {}
              f:targetPort: {}
          f:selector:
            .: {}
            f:app.kubernetes.io/name: {}
          f:sessionAffinity: {}
          f:type: {}
      manager: kubectl
      operation: Update
      time: "2021-04-13T10:50:22Z"
    name: kube-state-metrics
    namespace: kube-system
    resourceVersion: "630"
    uid: 12a3a777-97bf-476d-9a96-4c9265bdb7d9
  spec:
    clusterIP: None
    clusterIPs:
    - None
    ports:
    - name: http-metrics
      port: 8080
      protocol: TCP
      targetPort: http-metrics
    - name: telemetry
      port: 8081
      protocol: TCP
      targetPort: telemetry
    selector:
      app.kubernetes.io/name: kube-state-metrics
    sessionAffinity: None
    type: ClusterIP
  status:
    loadBalancer: {}
kind: List
metadata: {}
`
