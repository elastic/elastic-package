// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

const terraformDeployerDockerfile = `FROM hashicorp/terraform:light
ENV TF_IN_AUTOMATION=true
HEALTHCHECK --timeout=3s CMD sh -c "[ -f /tmp/tf-applied ]"
ADD run.sh /
WORKDIR /workspace
ENTRYPOINT sh /run.sh
`
