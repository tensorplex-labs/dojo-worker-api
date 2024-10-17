FROM golang:1.22 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download
# prefetch the prisma binaries, so that they will be cached and not downloaded on each change
RUN go run github.com/steebchen/prisma-client-go prefetch

COPY . .

RUN go run github.com/steebchen/prisma-client-go generate

ARG PLATFORM=linux
ARG ARCH=amd64
RUN CGO_ENABLED=0 GOARCH=${ARCH} GOOS=${PLATFORM} go build -a -installsuffix cgo -o service ./cmd/server/main.go

FROM ubuntu:22.04

# RUN apt-get update && \
#     DEBIAN_FRONTEND=noninteractive \
#     apt-get install -y --no-install-recommends \
#     ca-certificates \
#     build-essential \
#     xorg \
#     gnome-core \
#     libgtk-3-dev && \
#     apt-get clean && \
#     rm -rf /var/lib/apt/lists/*


WORKDIR /dojo-api

COPY --from=builder /app/service /dojo-api/service
COPY --from=builder /app/entrypoint.sh /dojo-api/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/dojo-api/entrypoint.sh"]