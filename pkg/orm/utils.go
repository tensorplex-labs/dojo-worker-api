package orm

import (
	"context"
	"encoding/json"
	"time"
	"os"
	"sync"
	"fmt"

	"dojo-api/db"
	"dojo-api/utils"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type SimpleConnHandler struct {
	client      *db.PrismaClient
	isConnected bool
}

type Secret struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var connHandler *SimpleConnHandler
var once sync.Once

func GetConnHandler() *SimpleConnHandler {
	once.Do(func() {
		connString := GetPostgresConnString()
		println(connString)
		prismaConfig := db.WithDatasourceURL(connString)
		connHandler = &SimpleConnHandler{
			client:      db.NewClient(prismaConfig),
			isConnected: false,
		}
	})
	return connHandler
}

func getSecret(secretName string, region string) (Secret, error) {
    var unmarshalledSecrets Secret

    maxRetries := 10
    retryDelay := time.Second

    for i := 0; i < maxRetries; i++ {
        config, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
        if err != nil {
            log.Error().Err(err).Msg("Unable to load SDK config")
            time.Sleep(retryDelay)
            continue
        }

        // Create Secrets Manager client
        svc := secretsmanager.NewFromConfig(config)

        input := &secretsmanager.GetSecretValueInput{
            SecretId:     aws.String(secretName),
            VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
        }

        result, err := svc.GetSecretValue(context.TODO(), input)
        if err != nil {
            log.Error().Err(err).Msg("Unable to retrieve secret")
            time.Sleep(retryDelay)
            continue
        }

        // Decrypts secret using the associated KMS key.
        var secretString string = *result.SecretString

        if err := json.Unmarshal([]byte(secretString), &unmarshalledSecrets); err != nil {
            log.Error().Err(err).Msg("Unable to unmarshal secret")
            time.Sleep(retryDelay)
            continue
        }

        if unmarshalledSecrets.Username == "" || unmarshalledSecrets.Password == "" {
            log.Error().Msg("Unable to retrieve username or password from secret")
            time.Sleep(retryDelay)
            continue
        }

        return unmarshalledSecrets, nil
    }

    return unmarshalledSecrets, fmt.Errorf("failed to retrieve secret after %d retries", maxRetries)
}
func GetPostgresConnString() string {
	if err := godotenv.Load(); err != nil {
		log.Fatal().Err(err).Msg("Error loading .env file")
		return ""
	}

	if utils.LoadDotEnv("RUNTIME_ENV") == "local"{
		return utils.LoadDotEnv("DATABASE_URL")
	}else{
		var username string
		var password string
		postgresBase := "postgresql://%s:%s@%s:5432/%s?schema=public"

		if os.Getenv("DB_USERNAME") == "" || os.Getenv("DB_PASSWORD") == "" {
			secrets, err := getSecret(utils.LoadDotEnv("AWS_SECRET_NAME"), utils.LoadDotEnv("AWS_REGION"))
			if err != nil {
				log.Fatal().Err(err).Msg("Error getting secrets")
				return ""
			}
			username = secrets.Username
			password = secrets.Password
		}else{
			username = os.Getenv("DB_USERNAME")
			password = os.Getenv("DB_PASSWORD")
		}

		return fmt.Sprintf(postgresBase, username, password, utils.LoadDotEnv("DATABASE_HOST"), "subnet_db")
	}
}

func GetPrismaClient() *db.PrismaClient {
	handler := GetConnHandler()
	if !handler.isConnected {
		if err := handler.client.Prisma.Connect(); err != nil {
			log.Fatal().Err(err).Msg("Failed to connect to Prisma client")
			return nil
		}
		handler.isConnected = true
	}
	return handler.client
}

func (h *SimpleConnHandler) OnShutdown() error {
	if h.client == nil {
		log.Warn().Msg("Prisma client not initialised")
		return nil
	}

	if err := h.client.Prisma.Disconnect(); err != nil {
		log.Error().Err(err).Msg("Failed to disconnect from Prisma client")
		return err
	} else {
		h.isConnected = false
	}
	return nil
}
