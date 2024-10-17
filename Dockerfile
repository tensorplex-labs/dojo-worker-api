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

FROM alpine:latest

RUN apk --no-cache add ca-certificates

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

WORKDIR /app

COPY --from=builder /app/service /app/service
COPY --from=builder /app/entrypoint.sh /app/entrypoint.sh

RUN chmod +x /app/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
