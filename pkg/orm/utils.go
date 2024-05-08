package orm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"dojo-api/db"
	"dojo-api/utils"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/rs/zerolog/log"
)

type AwsSecret struct {
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

func getAwsSecret(secretId string, region string) (AwsSecret, error) {
	var awsSecret AwsSecret

	maxRetries := 10
	retryDelay := time.Second

	for i := 0; i < maxRetries; i++ {
		// TODO try without region on staging first
		config, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			log.Error().Err(err).Msg("Unable to load SDK config")
			time.Sleep(retryDelay)
			continue
		}

		// Create Secrets Manager client
		svc := secretsmanager.NewFromConfig(config)

		input := &secretsmanager.GetSecretValueInput{
			SecretId:     aws.String(secretId),
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

		if err := json.Unmarshal([]byte(secretString), &awsSecret); err != nil {
			log.Error().Err(err).Msg("Unable to unmarshal secret")
			time.Sleep(retryDelay)
			continue
		}

		if awsSecret.Username == "" || awsSecret.Password == "" {
			log.Error().Msg("Unable to retrieve username or password from secret")
			time.Sleep(retryDelay)
			continue
		}

		return awsSecret, nil
	}

	return awsSecret, fmt.Errorf("failed to retrieve secret after %d retries", maxRetries)
}

func GetPrismaClient() *PrismaClientWrapper {
	handler := GetConnHandler()
	credentials := getPostgresCredentials()
	currentConnString := buildPostgresConnString(credentials)
	handler.mu.Lock()
	defer handler.mu.Unlock()

	existingWrapper, exists := handler.clientWrappers[currentConnString]
	if exists {
		log.Debug().Msg("Reusing existing Prisma client for connection string")
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
		log.Info().Msg("Reusing existing Prisma client")
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
}

func getPostgresCredentials() *DbSecrets {
	if utils.LoadDotEnv("RUNTIME_ENV") == "aws" {
		secretId := utils.LoadDotEnv("AWS_SECRET_ID")
		region := utils.LoadDotEnv("AWS_REGION")
		awsSecret, err := getAwsSecret(secretId, region)
		log.Info().Interface("secrets", awsSecret).Msg("Got AWS secrets")
		if err != nil {
			log.Fatal().Err(err).Msg("Error getting secrets")
			return nil
		}

		return &DbSecrets{
			username: awsSecret.Username,
			password: awsSecret.Password,
		}
	}

	username := utils.LoadDotEnv("DB_USERNAME")
	password := utils.LoadDotEnv("DB_PASSWORD")
	return &DbSecrets{
		username: username,
		password: password,
	}
}

func buildPostgresConnString(secrets *DbSecrets) string {
	if secrets == nil {
		log.Warn().Msg("No secrets provided, using DATABASE_URL directly")
		return utils.LoadDotEnv("DATABASE_URL")
	}

	host := utils.LoadDotEnv("DB_HOST")
	dbName := utils.LoadDotEnv("DB_NAME")
	safePassword := url.QueryEscape(secrets.password)
	databaseUrl := "postgresql://" + secrets.username + ":" + safePassword + "@" + host + "/" + dbName
	// hack this so Prisma can read it directly, handle complexities here
	os.Setenv("DATABASE_URL", databaseUrl)
	log.Info().Msgf("Built database connection string")
	return databaseUrl
}

func getPrismaConfig() func(*db.PrismaConfig) {
	secrets := getPostgresCredentials()
	prismaConfig := db.WithDatasourceURL(buildPostgresConnString(secrets))
	return prismaConfig
}
