package install

const packageRegistryDockerfile = `FROM docker.elastic.co/package-registry/distribution:snapshot

COPY ${CLUSTER_DIR}/package-registry.config.yml /package-registry/config.yml
COPY ${CLUSTER_DIR}/development/ /packages/development
`
