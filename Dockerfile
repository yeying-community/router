FROM --platform=$BUILDPLATFORM node:24 AS builder

WORKDIR /web
COPY ./VERSION .
COPY ./web .

RUN npm config set registry https://registry.npmmirror.com

RUN npm install --prefix /web

RUN VITE_APP_VERSION=$(cat ./VERSION) npm run build --prefix /web

FROM golang:alpine AS builder2

# 设置 apk 镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

RUN apk add --no-cache \
    gcc \
    musl-dev \
    sqlite-dev \
    build-base

ENV GO111MODULE=on \
    CGO_ENABLED=1 \
    GOOS=linux

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=builder /web/build ./web/build

RUN go build -trimpath -ldflags "-s -w -X 'github.com/yeying-community/router/common.Version=$(cat VERSION)' -linkmode external -extldflags '-static'" -o router ./cmd/router

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder2 /build/router /

EXPOSE 3011
WORKDIR /data
ENTRYPOINT ["/router"]
