package orm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"dojo-api/db"
	"dojo-api/utils"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

type Secret struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type PrismaClientWrapper struct {
	QueryTracker *QueryTracker
	Client       *db.PrismaClient
}

func (p *PrismaClientWrapper) BeforeQuery() {
	p.QueryTracker.BeforeQuery()
}

func (p *PrismaClientWrapper) AfterQuery() {
	p.QueryTracker.AfterQuery()
}

type QueryTracker struct {
	activeQueries int32
}

func (qt *QueryTracker) BeforeQuery() {
	atomic.AddInt32(&qt.activeQueries, 1)
}

func (qt *QueryTracker) AfterQuery() {
	atomic.AddInt32(&qt.activeQueries, -1)
}

func (qt *QueryTracker) WaitForAllQueries() {
	for atomic.LoadInt32(&qt.activeQueries) > 0 {
		time.Sleep(100 * time.Millisecond)
	}
}

type ConnHandler struct {
	clientWrappers map[string]PrismaClientWrapper
	mu             sync.Mutex
}

var connHandler *ConnHandler
var once sync.Once

func GetConnHandler() *ConnHandler {
	once.Do(func() {
		connHandler = &ConnHandler{
			clientWrappers: make(map[string]PrismaClientWrapper),
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

	if utils.LoadDotEnv("RUNTIME_ENV") == "local" {
		return utils.LoadDotEnv("DATABASE_URL")
	} else {
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
		} else {
			username = os.Getenv("DB_USERNAME")
			password = os.Getenv("DB_PASSWORD")
		}

		return fmt.Sprintf(postgresBase, username, password, utils.LoadDotEnv("DATABASE_HOST"), "subnet_db")
	}
}

func GetPrismaClient() *PrismaClientWrapper {
	handler := GetConnHandler()
	credentials := getPostgresCredentials()
	currentConnString := buildPostgresConnString(credentials.username, credentials.password)
	handler.mu.Lock()
	defer handler.mu.Unlock()

	existingWrapper, exists := handler.clientWrappers[currentConnString]
	if exists {
		log.Info().Msg("Reusing existing Prisma client for connString")
		return &existingWrapper
	}

	clientWrapper := PrismaClientWrapper{
		QueryTracker: &QueryTracker{},
		Client:       db.NewClient(getPrismaConfig()),
	}

	defer func() {
		if r := recover(); r != nil {
			log.Error().Msgf("Recovered from panic while connecting to Prisma client: %v", r)
		}
	}()

	err := clientWrapper.Client.Prisma.Connect()
	log.Warn().Msg("Trying to connect to Prisma...")
	if err == nil {
		log.Info().Msg("Successfully connected for new connection string")
		handler.clientWrappers[currentConnString] = clientWrapper
		return &clientWrapper
	}

	log.Warn().Msg("Failed to connect Prisma client for new connection string, attempting reuse...")
	for _, wrapper := range handler.clientWrappers {
		log.Debug().Msg("Reusing existing Prisma client")
		return &wrapper
	}
	log.Error().Err(err).Msg("No existing Prisma clients to reuse")
	return nil
}

func (h *ConnHandler) OnShutdown() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for connString, clientWrapper := range h.clientWrappers {
		if err := clientWrapper.Client.Prisma.Disconnect(); err != nil {
			log.Error().Err(err).Msgf("Failed to disconnect from Prisma client with connection string: %s", connString)
		} else {
			log.Info().Msgf("Disconnected from Prisma client with connection string: %s", connString)
		}
		delete(h.clientWrappers, connString)
	}
	return nil
}

type DbSecrets struct {
	username string
	password string
	url      string
}

func getPostgresCredentials() DbSecrets {
	// TODO @dev pull from aws secrets
	username := utils.LoadDotEnv("DB_USERNAME")
	password := utils.LoadDotEnv("DB_PASSWORD")
	return DbSecrets{
		username: username,
		password: password,
	}
}

func buildPostgresConnString(user string, password string) string {
	url := utils.LoadDotEnv("DB_URL")
	dbName := utils.LoadDotEnv("DB_NAME")
	databaseUrl := "postgresql://" + user + ":" + password + "@" + url + "/" + dbName
	// hack this so Prisma can read it directly, handle complexities here
	os.Setenv("DATABASE_URL", databaseUrl)
	return databaseUrl
}

func getPrismaConfig() func(*db.PrismaConfig) {
	secrets := getPostgresCredentials()
	prismaConfig := db.WithDatasourceURL(buildPostgresConnString(secrets.username, secrets.password))
	return prismaConfig
}
