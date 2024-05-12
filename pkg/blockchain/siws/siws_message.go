package siws

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// regex is not a strict as siwe-go because who gives a fuck as long as we can verify
const SiwsAccountExp = "(?P<account>.+?)\n\n"
const SiwsNonceExp = "Nonce: (?P<nonce>.+?)\n"
const SiwsIssuedAtExp = "Issued At: (?P<issuedAt>.+?)\n"
const SiwsStatementExp = "((?P<statement>[^\\n]+)\\n)?\\n"
const _RFC3986 = "(([^ :/?#]+):)?(//([^ /?#]*))?([^ ?#]*)(\\?([^ #]*))?(#(.*))?"
const _SIWE_DATETIME = "([0-9]+)-(0[1-9]|1[012])-(0[1-9]|[12][0-9]|3[01])[Tt]([01][0-9]|2[0-3]):([0-5][0-9]):([0-5][0-9]|60)(\\.[0-9]+)?(([Zz])|([\\+|\\-]([01][0-9]|2[0-3]):[0-5][0-9]))"

var SiwsExpireAtExp = fmt.Sprintf("Expiration Time: (?P<expireAt>%s)?", _SIWE_DATETIME)

var SiwsUriExp = fmt.Sprintf("URI: (?P<uri>%s?)\\n", _RFC3986)

const SiwsDomainExp = "(?P<domain>.+?) wants you to sign in with your Polkadot account:\n"

// use dotall as prefix
var SiwsMessageExp = regexp.MustCompile(strings.Join([]string{SiwsDomainExp, SiwsAccountExp, SiwsStatementExp, SiwsUriExp, SiwsNonceExp, SiwsIssuedAtExp, SiwsExpireAtExp}, ".*?"))

type SiwsMessage struct {
	rawMessage   string
	matchResults map[string]interface{}
	URI          string
	Statement    string
	Domain       string
	Address      string
	Nonce        string
	IssuedAt     time.Time
	ExpireAt     time.Time
}

func validateDomain(domain *string) (bool, error) {
	if domain == nil || len(strings.TrimSpace(*domain)) == 0 {
		log.Error().Msg("Domain is required")
		return false, nil
	}

	url, err := url.Parse(fmt.Sprintf("https://%s", *domain))
	if err != nil {
		return false, fmt.Errorf("invalid format for field `domain`")
	}

	authority := url.Host
	if url.User != nil {
		authority = fmt.Sprintf("%s@%s", url.User.String(), authority)
	}

	if authority != *domain {
		return false, fmt.Errorf("invalid format for field `domain`")
	}

	return true, nil
}

// TODO read nonce outside of this function to validate nonce generated from our backend
func ParseMessage(message string) (*SiwsMessage, error) {
	match := SiwsMessageExp.FindStringSubmatch(message)

	result := make(map[string]interface{})
	if match != nil {
		for i, name := range SiwsMessageExp.SubexpNames() {
			if i != 0 && name != "" {
				result[name] = match[i]
			}
		}

		fmt.Println("uri:", result["uri"])
		fmt.Println("statement:", result["statement"])
		fmt.Println("domain:", result["domain"])
		fmt.Println("Account:", result["account"])
		fmt.Println("Nonce:", result["nonce"])
		fmt.Println("Issued At:", result["issuedAt"])
		fmt.Println("Expire At", result["expireAt"])
	}

	if _, ok := result["uri"].(string); !ok {
		log.Error().Msg("URI is required")
		return nil, fmt.Errorf("URI is required")
	}

	uri := result["uri"].(string)
	siwsMessage := &SiwsMessage{}
	siwsMessage.rawMessage = message
	siwsMessage.matchResults = result
	url, err := url.Parse(uri)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse URI according to RFC3986")
		return nil, fmt.Errorf("failed to parse URI according to RFC3986: %v", err)
	}

	siwsMessage.URI = url.String()

	if _, ok := result["domain"]; !ok {
		log.Error().Msg("Domain is required")
		return nil, fmt.Errorf("domain is required")
	}

	domain := result["domain"].(string)
	isValid, err := validateDomain(&domain)
	if !isValid {
		log.Error().Err(err).Msg("Invalid domain")
		return nil, fmt.Errorf("invalid domain: %v\nError: %s", domain, err)
	}
	siwsMessage.Domain = domain

	if _, ok := result["account"]; !ok {
		log.Error().Msg("Account is required")
		return nil, fmt.Errorf("account is required")
	}

	_, err = SS58AddressToPublickey(result["account"].(string))
	if err != nil {
		log.Error().Err(err).Msg("Invalid Address format")
		return nil, fmt.Errorf("invalid address format: %v", err)
	}
	siwsMessage.Address = result["account"].(string)

	if _, ok := result["statement"].(string); ok {
		siwsMessage.Statement = result["statement"].(string)
	}

	if _, ok := result["nonce"]; !ok {
		log.Error().Msg("Nonce is required")
		return nil, fmt.Errorf("nonce is required")
	}
	siwsMessage.Nonce = result["nonce"].(string)

	if _, ok := result["issuedAt"]; !ok {
		log.Error().Msg("Issued At is required")
		return nil, fmt.Errorf("issued at is required")
	}
	if issuedAt, err := time.Parse(time.RFC3339, result["issuedAt"].(string)); err == nil {
		siwsMessage.IssuedAt = issuedAt
	} else {
		log.Error().Err(err).Msg("Failed to parse Issued At")
		return nil, fmt.Errorf("failed to parse issued at: %v", err)
	}

	if _, ok := result["expireAt"]; !ok {
		log.Error().Msg("Expire At is required")
		return nil, fmt.Errorf("expire at is required")
	}
	if expireAt, err := time.Parse(time.RFC3339, result["expireAt"].(string)); err == nil {
		if time.Now().After(expireAt) {
			log.Error().Msg("The message has expired")
			return nil, fmt.Errorf("the message has expired at: %v", expireAt)
		}
		siwsMessage.ExpireAt = expireAt
	} else {
		log.Error().Err(err).Msg("Failed to parse Expire At")
		return nil, fmt.Errorf("failed to parse expire at: %v", err)
	}

	return siwsMessage, nil
}
