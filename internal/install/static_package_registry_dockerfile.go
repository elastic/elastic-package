package install

const packageRegistryDockerfile = `FROM docker.elastic.co/package-registry/distribution:snapshot

COPY ${STACK_DIR}/package-registry.config.yml /package-registry/config.yml
COPY ${STACK_DIR}/development/ /packages/development
`
