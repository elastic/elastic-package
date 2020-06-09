package install

const packageRegistryVolumeYml = `version: '2.3'
services:
  package-registry:
	volume:
	  ${PACKAGES_PATH}:/registry/public

`
