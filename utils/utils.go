package utils

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

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

	// sanity checks
	LoadDotEnv("DATABASE_URL")
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
