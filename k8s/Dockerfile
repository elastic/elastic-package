FROM kindest/node:v1.20.2@sha256:8f7ea6e7642c0da54f04a7ee10431549c0257315b3a634f6ef2fecaaedb19bab

HEALTHCHECK --interval=10s --timeout=3s \
    CMD curl --cacert /etc/kubernetes/pki/ca.crt --key /etc/kubernetes/pki/apiserver-kubelet-client.key \
        --cert /etc/kubernetes/pki/apiserver-kubelet-client.crt -k https://localhost:10250/pods -s -f || exit 1

RUN apt-get update -y
RUN apt-get install -y net-tools

ADD kubeadm.conf /kind/

ADD entrypoint /usr/local/bin/
ADD kubeadm-init /usr/local/bin/
