// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

// PackageRegistryBaseImage is the base Docker image of the Elastic Package Registry.
const PackageRegistryBaseImage = "docker.elastic.co/package-registry/distribution:snapshot"

const packageRegistryDockerfile = `FROM ` + PackageRegistryBaseImage + `

COPY ${STACK_DIR}/package-registry.config.yml /package-registry/config.yml
COPY ${STACK_DIR}/development/ /packages/development
`
