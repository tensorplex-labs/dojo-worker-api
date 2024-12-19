package main

import (
	"net/url"

	"dojo-api/pkg/orm"
	"dojo-api/utils"

	"github.com/rs/zerolog/log"
)

func main() {
	secretId := utils.LoadDotEnv("AWS_SECRET_ID")
	region := utils.LoadDotEnv("AWS_REGION")
	secret, err := orm.GetAwsSecret(secretId, region)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get AWS secret")
		return
	}
	log.Info().Interface("secret", secret).Msg("Successfully retrieved AWS secret")
	safePassword := url.QueryEscape(secret.Password)
	log.Info().Str("url escaped password", safePassword).Msg("password")
}
