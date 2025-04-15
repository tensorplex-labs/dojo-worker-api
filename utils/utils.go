package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

// special init func that gets called for setting up all configs at the start
func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Msg("Error loading .env file")
	}
	LoadDotEnv("DB_HOST")
	LoadDotEnv("DB_NAME")
	// sanity checks
	if LoadDotEnv("RUNTIME_ENV") == "aws" {
		LoadDotEnv("AWS_SECRET_ID")
		LoadDotEnv("AWS_REGION")
		LoadDotEnv("AWS_ROLE_ARN")
	} else {
		LoadDotEnv("DB_USERNAME")
		LoadDotEnv("DB_PASSWORD")
	}

	LoadDotEnv("SUBSTRATE_API_URL")
	LoadDotEnv("VALIDATOR_MIN_STAKE")
	LoadDotEnv("JWT_SECRET")
	LoadDotEnv("TOKEN_EXPIRY")
	LoadDotEnv("SERVER_PORT")
	LoadDotEnv("ETHEREUM_NODE")

	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	debug := flag.Bool("debug", false, "sets log level to debug")
	trace := flag.Bool("trace", false, "sets log level to trace")
	flag.Parse()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if *trace {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
}

func IpDecimalToDotted(decimalIP interface{}) string {
	var ipInt int64
	switch v := decimalIP.(type) {
	case int64:
		ipInt = v
	case string:
		var err error
		ipInt, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			log.Error().Err(err).Msg("Error converting string to int64")
			return ""
		}
	default:
		fmt.Println("Unsupported type provided")
		return ""
	}
	b0 := ipInt & 0xff
	b1 := (ipInt >> 8) & 0xff
	b2 := (ipInt >> 16) & 0xff
	b3 := (ipInt >> 24) & 0xff
	return fmt.Sprintf("%d.%d.%d.%d", b3, b2, b1, b0)
}

func LoadDotEnv(varName string) string {
	envVar := os.Getenv(varName)
	if envVar == "" {
		log.Fatal().Msgf("Environment variable %s not set", varName)
	}
	return envVar
}

// Parse ISO8601 date string to time.Time
func ParseDate(date string) *time.Time {
	parsedDate, err := time.Parse(time.RFC3339, date)
	if err != nil {
		log.Error().Stack().Err(err).Msg("Error parsing date")
		return nil
	}

	utcDate := parsedDate.UTC()
	return &utcDate
}

func GenerateRandomMinerSubscriptionKey() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		log.Error().Stack().Err(err).Msg("Error generating random bytes")
		return "", err
	}
	key := hex.EncodeToString(b)
	key = "sk-" + key
	return key, nil
}

// Initialize the S3 client
func getS3Client() (*s3.Client, error) {
	AWS_REGION := os.Getenv("AWS_REGION")
	if AWS_REGION == "" {
		log.Warn().Msg("AWS_REGION not set. S3 functionality will be disabled.")
		return nil, nil
	}
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(AWS_REGION))
	if err != nil {
		log.Error().Err(err).Str("aws region", AWS_REGION).Msg("Error loading default AWS config")
	}

	var s3Client *s3.Client
	if runtimeEnv := LoadDotEnv("RUNTIME_ENV"); runtimeEnv == "aws" {
		// AWS_ROLE_ARN := LoadDotEnv("AWS_ROLE_ARN")
		// stsClient := sts.NewFromConfig(cfg)
		// assumeRoleOutput, err := stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
		// 	RoleArn:         aws.String(AWS_ROLE_ARN),
		// 	RoleSessionName: aws.String("dojo-go-api-session"),
		// })
		// if err != nil {
		// 	log.Error().Err(err).Msg("Error assuming role")
		// 	return nil, err
		// }

		// // Create new configuration with assumed role credentials
		// _, err = config.LoadDefaultConfig(ctx,
		// 	config.WithRegion(AWS_REGION),
		// 	config.WithCredentialsProvider(
		// 		credentials.NewStaticCredentialsProvider(
		// 			*assumeRoleOutput.Credentials.AccessKeyId,
		// 			*assumeRoleOutput.Credentials.SecretAccessKey,
		// 			*assumeRoleOutput.Credentials.SessionToken,
		// 		),
		// 	),
		// )
		// if err != nil {
		// 	log.Error().Err(err).Msg("Error loading assumed role config")
		// 	return nil, err
		// }

		// log.Info().Interface("cfg", cfg).Msg("Creating S3 client")
		// // TODO try without assume role first
		// s3Client = s3.NewFromConfig(assumedCfg)
		s3Client = s3.NewFromConfig(cfg)
	} else {
		s3Client = s3.NewFromConfig(cfg)
	}

	log.Info().Interface("cfg", cfg).Msg("Creating S3 client")

	return s3Client, nil
}

// Get the S3 uploader
func getS3Uploader(client *s3.Client) *manager.Uploader {
	return manager.NewUploader(client)
}

func UploadFileToS3(file *multipart.FileHeader) (*manager.UploadOutput, error) {
	// Open the file
	bucketName := os.Getenv("AWS_S3_BUCKET_NAME")
	if bucketName == "" {
		log.Warn().Msg("AWS_S3_BUCKET_NAME not set. File upload skipped.")
		return nil, nil
	}
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// Create an S3 client
	log.Info().Interface("file", file).Msg("Uploading file")
	s3Client, err := getS3Client()
	if err != nil {
		log.Error().Err(err).Msg("Error creating S3 client")
		return nil, err
	}
	if s3Client == nil {
		log.Warn().Msg("S3 client is not available. File upload skipped.")
		return nil, nil
	}
	uploader := getS3Uploader(s3Client)

	// Determine the content type
	contentType, err := getContentType(src)
	if err != nil {
		return nil, fmt.Errorf("error determining content type: %w", err)
	}

	// Generate a unique filename to prevent duplicates
	uniqueFilename := generateUniqueFilename(file.Filename)

	// Upload the file to S3
	result, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket:             aws.String(bucketName),
		Key:                aws.String(uniqueFilename),
		Body:               src,
		ContentType:        aws.String(contentType),
		ContentDisposition: aws.String(fmt.Sprintf("attachment; filename=\"%s\"", file.Filename)),
	})
	if err != nil {
		log.Error().Err(err).Msg("Error uploading file")
		return result, err
	}

	return result, nil
}

func getContentType(file multipart.File) (string, error) {
	// Detect content type based on file content
	buffer := make([]byte, 512)
	_, err := file.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("error reading file content for MIME type detection: %w", err)
	}

	// Reset the file pointer
	_, err = file.Seek(0, 0)
	if err != nil {
		return "", fmt.Errorf("error resetting file pointer: %w", err)
	}

	// Detect content type
	contentType := http.DetectContentType(buffer)

	// validate against allowed types
	allowedTypes := map[string]bool{
		"image/jpeg":               true,
		"image/png":                true,
		"image/gif":                true,
		"image/webp":               true,
		"application/vnd.ply":      true,
		"application/octet-stream": true,
	}

	if !allowedTypes[contentType] {
		return "", fmt.Errorf("unsupported content type detected: %s", contentType)
	}

	return contentType, nil
}

func generateUniqueFilename(originalFilename string) string {
	ext := filepath.Ext(originalFilename)
	name := strings.TrimSuffix(originalFilename, ext)
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_%d%s", name, timestamp, ext)
}

// AthenaConfig holds configuration for Athena queries
type AthenaConfig struct {
	Database         string
	Workgroup        string
	OutputLocation   string
	MaxExecutionTime time.Duration
	PollingInterval  time.Duration
	MaxRetries       int
}

// DefaultAthenaConfig creates a configuration with default values
func DefaultAthenaConfig() AthenaConfig {
	return AthenaConfig{
		Database:         "default",
		Workgroup:        "primary",
		OutputLocation:   os.Getenv("ATHENA_OUTPUT_LOCATION"),
		MaxExecutionTime: 60 * time.Second,
		PollingInterval:  2 * time.Second,
		MaxRetries:       3,
	}
}

// GetAthenaClient creates an AWS Athena client using the provided context
func GetAthenaClient(ctx context.Context) (*athena.Client, *AthenaConfig, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("context cannot be nil")
	}

	// Get default configuration
	athenaConfig := DefaultAthenaConfig()

	// Validate the required OutputLocation
	if athenaConfig.OutputLocation == "" {
		return nil, nil, fmt.Errorf("ATHENA_OUTPUT_LOCATION not set")
	}

	// Load optional variables
	if dbName := os.Getenv("ATHENA_DATABASE"); dbName != "" {
		athenaConfig.Database = dbName
	}

	if workgroup := os.Getenv("ATHENA_WORKGROUP"); workgroup != "" {
		athenaConfig.Workgroup = workgroup
	}

	// Get AWS region
	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		return nil, nil, fmt.Errorf("AWS_REGION not set")
	}

	// Create AWS configuration
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(awsRegion),
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(retry.NewStandard(), athenaConfig.MaxRetries)
		}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create and return Athena client
	startTime := time.Now()
	client := athena.NewFromConfig(awsCfg)
	createTime := time.Since(startTime)

	log.Info().
		Str("database", athenaConfig.Database).
		Str("workgroup", athenaConfig.Workgroup).
		Str("outputLocation", athenaConfig.OutputLocation).
		Dur("createTime", createTime).
		Msg("Athena client created")

	// metrics.RecordDuration("athena.client.create", createTime)

	return client, &athenaConfig, nil
}
