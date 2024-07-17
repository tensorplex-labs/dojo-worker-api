FROM golang:1.22 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go run github.com/steebchen/prisma-client-go generate
RUN go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps

ARG PLATFORM=linux
ARG ARCH=amd64
RUN CGO_ENABLED=0 GOARCH=${ARCH} GOOS=${PLATFORM} go build -a -installsuffix cgo -o service ./cmd/server/main.go

FROM ubuntu:22.04

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    build-essential \
    xorg \
    gnome-core \
    libgtk-3-dev && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /dojo-api

COPY --from=builder /app/service .

EXPOSE 8080

CMD ["./service"]
