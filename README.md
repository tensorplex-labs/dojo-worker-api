# Dojo Subnet API

Repository for our Dojo Subnet APIs. Check request.REST file for developed APIs to test out.(make sure that REST client extension is already installed)
Extension ID: humao.rest-client

This mainly consists of our authentication services, and human task services.

## Run Locally

In order to setup the database connection, you can utilze docker compose to setup a local postgres instance.

You will need to install Docker and Docker Compose.
Make sure to update the .env file with the correct credentials.

```bash
docker-compose up -d
# setup local db
go run github.com/steebchen/prisma-client-go generate
go run github.com/steebchen/prisma-client-go db push
```

Currently the repo is structured to house multiple microservices, where each of the microservices' `main` functions are in `/cmd/service_name/main.go`

### To run all services
```bash
go run cmd/server/main.go
# go install github.com/cosmtrek/air@latest
air
```

## Deploy to Production

Clone the project