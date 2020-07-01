package install

const packageRegistryVolumeYml = `version: '2.3'
services:
  package-registry:
    volumes:
      - ./package-registry.config.yml:/registry/config.yml
      - ${PACKAGES_PATH}:/registry/packages/integrations
`
