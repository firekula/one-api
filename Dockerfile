FROM --platform=$BUILDPLATFORM node:16 AS builder

WORKDIR /web
COPY ./VERSION .
COPY ./web .

RUN cd /web/default && npm install --legacy-peer-deps && \
    DISABLE_ESLINT_PLUGIN='true' REACT_APP_VERSION=$(cat /web/VERSION) npm run build && \
    cd /web/berry && npm install --legacy-peer-deps && \
    DISABLE_ESLINT_PLUGIN='true' REACT_APP_VERSION=$(cat /web/VERSION) npm run build && \
    cd /web/air && npm install --legacy-peer-deps && \
    DISABLE_ESLINT_PLUGIN='true' REACT_APP_VERSION=$(cat /web/VERSION) npm run build

FROM golang:alpine AS builder2

RUN apk add --no-cache \
    gcc \
    musl-dev \
    sqlite-dev \
    build-base

ENV GO111MODULE=on \
    CGO_ENABLED=1 \
    GOOS=linux \
    GOPROXY=https://goproxy.cn,direct

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=builder /web/build ./web/build

RUN go build -trimpath -ldflags "-s -w -X 'github.com/songquanpeng/one-api/common.Version=$(cat VERSION)' -linkmode external -extldflags '-static'" -o one-api

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder2 /build/one-api /

EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/one-api"]