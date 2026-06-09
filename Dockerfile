ARG PROJECT_NAME=exporter

FROM golang:1.26 AS build

WORKDIR /src

ARG PROJECT_NAME
ARG LDFLAGS

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build \
  -buildvcs=false \
  -trimpath \
  -ldflags "${LDFLAGS}" \
  -o "/out/${PROJECT_NAME}" \
  "./cmd"

FROM debian:bookworm-slim

ARG PROJECT_NAME

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/${PROJECT_NAME} /usr/local/bin/exporter

USER nobody

ENTRYPOINT ["/usr/local/bin/exporter"]
