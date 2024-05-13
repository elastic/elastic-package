title: Docker custom agent image build 

# Creating elastic-agent-oracle-client

docker build -t service-integrations/elastic-agent-oracle-client .

docker images | grep 'service-integrations/elastic-agent-oracle-client' | grep -v grep (pick the image id)

docker tag 269e70f6e8b3 docker.elastic.co/employees/agithomas/elastic-agent-oracle-client:0.1

# Pushing elastic-agent-oracle-client to elastic repo

docker push docker.elastic.co/employees/agithomas/elastic-agent-oracle-client:0.1
