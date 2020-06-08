package install

const localYml = `
version: '2.3'
services:
  kibana:
    ports:
      - "127.0.0.1:5601:5601"

  elasticsearch:
    ports:
      - "127.0.0.1:9200:9200"

  package-registry:
    ports:
      - "127.0.0.1:8080:8080"
`
