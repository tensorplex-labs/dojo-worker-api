# Dojo Subnet API

Repository for our Dojo Subnet APIs.

This mainly consists of our authentication services, and human task services.

## Features

- Feature 1
- Feature 2
- Feature 3


## Tech Stack

**Server:**
- Gin for web
- Zerolog for logging
- Prisma Go ORM


## Run Locally

```bash
# setup local db
go run github.com/steebchen/prisma-client-go generate
go run github.com/steebchen/prisma-client-go db push
```

Currently the repo is structured to house multiple microservices, where each of the microservices' `main` functions are in `/cmd/service_name/main.go`

### To run auth service
```bash
go run /cmd/auth/main.go
```

### To run task service
```bash
go run /cmd/task/main.go
```

## Deploy to Production

Clone the project