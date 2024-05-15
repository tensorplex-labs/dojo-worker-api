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
		// } else {
		// 	LoadDotEnv("DATABASE_URL")
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
	// TODO - maybe move under aws runtime env
	LoadDotEnv("AWS_S3_BUCKET_NAME")
	LoadDotEnv("AWS_ACCESS_KEY_ID")
	LoadDotEnv("AWS_SECRET_ACCESS_KEY")

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
	// Load the default AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	// Create an S3 client using the loaded configuration
	client := s3.NewFromConfig(cfg)
	return client, nil
}

// Get the S3 uploader
func getS3Uploader(client *s3.Client) *manager.Uploader {
	return manager.NewUploader(client)
}

func UploadFileToS3(file *multipart.FileHeader) (*manager.UploadOutput, error) {
	// Open the file
	bucketName := LoadDotEnv("AWS_S3_BUCKET_NAME")
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()
	// Generate a unique file name for the S3 object
	// fileName := fmt.Sprintf("uploads/%d_%s", time.Now().Unix(), file.Filename)

	// Create an S3 client
	log.Info().Interface("file", file).Msg("Uploading file")
	s3Client, err := getS3Client()
	if err != nil {
		log.Error().Err(err).Msg("Error creating S3 client")
		return nil, err
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
