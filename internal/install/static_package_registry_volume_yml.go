package install

const packageRegistryVolumeYml = `version: '2.3'
services:
  package-registry:
    volumes:
      - ${PACKAGES_PATH}:/registry/public
`
