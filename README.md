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

## API Documentation with gin-swagger
To document your APIs and generate Swagger documentation using gin-swagger, follow these steps:

### Step 1: Add Swagger Annotations to Your API
Add [Swagger annotations]((https://github.com/swaggo/swag?tab=readme-ov-file#general-api-info)) to your Go code. Here’s an example of how to annotate an API:

```go
// GenerateNonceController godoc
// @Summary Generate nonce
// @Description Generate a nonce for a given wallet address
// @Tags Authentication
// @Accept json
// @Produce json
// @Param address path string true "Wallet address"
// @Success 200 {object} ApiResponse{body=main.GenerateNonceResponse} "Nonce generated successfully"
// @Failure 400 {object} ApiResponse "Address parameter is required"
// @Failure 500 {object} ApiResponse "Failed to store nonce"
// @Router /api/v1/auth/{address} [get]

func GenerateNonceController(c *gin.Context) {
}
```
> **Note:** Highly recommend to creating models for Swagger to ensures clarity and consistency, automatic documentation generation, and data validation. Without models, Swagger cannot accurately represent complex data structures or generate detailed API documentation.

### Step 2: Generate the Swagger Documentation
Run the swag command to generate the Swagger documentation. This command will scan your Go files for Swagger annotations and generate the required files for gin-swagger. Make sure to specify the main Go file.

```bash
swag init -g cmd/server/main.go
```
Format the Swagger annotation by using this commend (Optional)
```bash
swag fmt
```

### Step 3: Access the Documentation
Once you have your application running, you can access the Swagger UI at http://localhost:PORT/swagger/index.html.


### References

- [gin-swagger GitHub repository](https://github.com/swaggo/gin-swagger)
- [Swag command GitHub repository](https://github.com/swaggo/swag)
- [Swagger UI documentation](https://swagger.io/tools/swagger-ui/)
