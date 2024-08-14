package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"mime/multipart"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
	"github.com/playwright-community/playwright-go"
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
		os.Getenv("AWS_ACCESS_KEY_ID")
		os.Getenv("AWS_SECRET_ACCESS_KEY")
	}

	LoadDotEnv("SUBSTRATE_API_URL")
	LoadDotEnv("VALIDATOR_MIN_STAKE")
	LoadDotEnv("JWT_SECRET")
	LoadDotEnv("TOKEN_EXPIRY")
	LoadDotEnv("SERVER_PORT")
	LoadDotEnv("ETHEREUM_NODE")
	os.Getenv("AWS_S3_BUCKET_NAME")
	os.Getenv("S3_PUBLIC_URL")

	err = playwright.Install(
		&playwright.RunOptions{
			Browsers: []string{"firefox"},
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Error installing playwright")
	}

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
	return &parsedDate
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

	// Upload the file to S3
	result, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(file.Filename),
		Body:   src,
		// ContentType: aws.String(file.Header.Get("Content-Type")),
	})
	if err != nil {
		log.Error().Err(err).Msg("Error uploading file")
		return result, err
	}

	return result, nil
}
