ARG PROJECT_NAME=exporter

FROM golang:1.27.0-alpine3.24 AS build

WORKDIR /src

ARG PROJECT_NAME
ARG LDFLAGS
ARG TARGETOS
ARG TARGETARCH
ARG BUILDER_PACKAGES="make=4.4.1-r4"

RUN apk add --no-cache ${BUILDER_PACKAGES}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make build \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    ${LDFLAGS:+LDFLAGS="${LDFLAGS}"}

FROM alpine:3.24

ARG PROJECT_NAME

ARG RUNTIME_PACKAGES="ca-certificates=20260611-r0 libcrypto3=3.5.8-r0 libssl3=3.5.8-r0"
RUN apk add --no-cache ${RUNTIME_PACKAGES}

COPY --from=build /src/dist/${PROJECT_NAME} /usr/local/bin/exporter

EXPOSE 9853

USER nobody

ENV HEALTHCHECK_URL=http://127.0.0.1:9853/healthz

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -T 2 --spider "${HEALTHCHECK_URL}" || exit 1

ENTRYPOINT ["/usr/local/bin/exporter"]
