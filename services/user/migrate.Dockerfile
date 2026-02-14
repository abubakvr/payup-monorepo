FROM golang:1.22-alpine

RUN apk add --no-cache bash curl

RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.1/migrate.linux-amd64.tar.gz \
  | tar xvz && mv migrate /usr/local/bin/migrate

WORKDIR /migrations
COPY migrations ./migrations
COPY migrate.sh /migrate.sh

ENTRYPOINT ["/migrate.sh"]
