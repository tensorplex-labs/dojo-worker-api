FROM golang:1.22 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go run github.com/steebchen/prisma-client-go generate
RUN go run github.com/steebchen/prisma-client-go db push

ARG PLATFORM=linux
ARG ARCH=amd64
RUN CGO_ENABLED=0 GOARCH=${ARCH} GOOS=${PLATFORM} go build -a -installsuffix cgo -o service ./cmd/server/main.go

FROM ubuntu:22.04

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /root/

COPY --from=builder /app/service .

#TODO: Remove this once we have a way to pass env vars to the container either through secrets manager when we deploy to EKS
COPY .env .

EXPOSE 8080

CMD ["./service"]