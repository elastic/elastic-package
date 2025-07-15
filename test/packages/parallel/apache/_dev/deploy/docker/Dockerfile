ARG SERVICE_VERSION=${SERVICE_VERSION:-2.4.51}
FROM httpd:$SERVICE_VERSION
RUN apt-get update && apt-get install -y curl
HEALTHCHECK --interval=1s --retries=90 CMD curl -f http://localhost
COPY ./httpd.conf /usr/local/apache2/conf/httpd.conf
