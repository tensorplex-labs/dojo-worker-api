package athena

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"dojo-api/utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/rs/zerolog/log"
)

// Client cache for reusing Athena client
var (
	clientMu     sync.RWMutex
	clientCache  *athena.Client
	configCache  *utils.AthenaConfig
	clientExpiry time.Time

	// Concurrency limiter for Athena queries
	// Fixed at 20 concurrent queries to prevent overwhelming the Athena service
	queryLimiter = make(chan struct{}, 2)
)

// TTL of 30 minutes is reasonable for read-only operations
const clientCacheTTL = 30 * time.Minute

// Parameter represents a named parameter for Athena queries
type Parameter struct {
	Name  string
	Value any
}

// UnixTimestampDate represents a Unix timestamp that should be formatted as date-only (YYYY-MM-DD)
type UnixTimestampDate int64

// getClient returns a cached Athena client or creates a new one if needed
func getClient(ctx context.Context) (*athena.Client, *utils.AthenaConfig, error) {
	// Check cache with read lock first (fast path)
	clientMu.RLock()
	if clientCache != nil && time.Now().Before(clientExpiry) {
		client, config := clientCache, configCache
		clientMu.RUnlock()
		return client, config, nil
	}
	clientMu.RUnlock()

	// Need new client - acquire write lock
	clientMu.Lock()
	defer clientMu.Unlock()

	// Double-check after acquiring lock
	if clientCache != nil && time.Now().Before(clientExpiry) {
		return clientCache, configCache, nil
	}

	// Create new client
	client, config, err := utils.GetAthenaClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Athena client: %w", err)
	}

	// Update cache
	clientCache = client
	configCache = config
	clientExpiry = time.Now().Add(clientCacheTTL)
	log.Info().Msg("Created new cached Athena client")

	return client, config, nil
}

// ExecuteAthenaQuery executes a query in AWS Athena and returns the results.
// It handles query execution, polling for completion, and retrieving results.
// This function respects the concurrent query limit.
//
// It can be used in two ways:
//  1. With a regular query string: ExecuteAthenaQuery(ctx, "SELECT * FROM table")
//  2. With a parameterized query: ExecuteAthenaQuery(ctx, "SELECT * FROM table WHERE id = :id",
//     Parameter{Name: "id", Value: 123})
func ExecuteAthenaQuery(ctx context.Context, query string, params ...Parameter) (*athena.GetQueryResultsOutput, error) {
	var finalQuery string

	// Apply parameters if provided
	if len(params) > 0 {
		processedQuery, err := applyParameters(query, params)
		if err != nil {
			return nil, err
		}
		finalQuery = processedQuery
	} else {
		finalQuery = query
	}

	// Acquire a slot in the query limiter
	select {
	case queryLimiter <- struct{}{}:
		// We got a slot, make sure to release it when done
		defer func() { <-queryLimiter }()
	case <-ctx.Done():
		// Context was cancelled while waiting for a slot
		return nil, fmt.Errorf("context cancelled while waiting for query slot: %w", ctx.Err())
	}

	// Log query execution with slot information
	log.Debug().
		Int("running_queries", len(queryLimiter)).
		Int("total_capacity", cap(queryLimiter)).
		Int("slots_available", cap(queryLimiter)-len(queryLimiter)).
		Msg("Acquired concurrent query slot, executing Athena query")

	// Get Athena client from cache or create new one
	client, config, err := getClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Athena client: %w", err)
	}

	// Start the query with context awareness
	startOutput, err := client.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String(finalQuery),
		QueryExecutionContext: &types.QueryExecutionContext{
			Database: aws.String(config.Database),
		},
		WorkGroup: aws.String(config.Workgroup),
		ResultConfiguration: &types.ResultConfiguration{
			OutputLocation: aws.String(config.OutputLocation),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start Athena query: %w", err)
	}

	queryID := startOutput.QueryExecutionId
	log.Info().Str("query_id", *queryID).Msg("Athena query started")

	// Set up a timeout for the polling operation
	pollCtx, cancel := context.WithTimeout(ctx, config.MaxExecutionTime)
	defer cancel()

	// Create ticker for polling at regular intervals
	ticker := time.NewTicker(config.PollingInterval)
	defer ticker.Stop()

	// Poll for completion with proper cancellation
	for {
		select {
		case <-pollCtx.Done():
			return nil, fmt.Errorf("query execution timed out after %v: %w",
				config.MaxExecutionTime, pollCtx.Err())
		case <-ticker.C:
			// Check query status
			execution, err := client.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
				QueryExecutionId: queryID,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get query execution status: %w", err)
			}

			status := execution.QueryExecution.Status.State
			log.Debug().Str("status", string(status)).Msg("Athena query status")

			switch status {
			case types.QueryExecutionStateSucceeded:
				// Query completed successfully, get results
				results, err := client.GetQueryResults(ctx, &athena.GetQueryResultsInput{
					QueryExecutionId: queryID,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to get query results: %w", err)
				}
				return results, nil
			case types.QueryExecutionStateFailed, types.QueryExecutionStateCancelled:
				// Query failed
				reason := "unknown reason"
				if execution.QueryExecution.Status.StateChangeReason != nil {
					reason = *execution.QueryExecution.Status.StateChangeReason
				}
				return nil, fmt.Errorf("query execution failed: %s", reason)
			default:
				// Continue polling
				continue
			}
		}
	}
}

// applyParameters safely substitutes parameters into a query string
func applyParameters(query string, params []Parameter) (string, error) {
	result := query

	// Create a map for easier parameter lookups
	paramMap := make(map[string]any)
	for _, p := range params {
		paramMap[p.Name] = p.Value
	}

	// Find all parameter placeholders in the format :param_name
	for name, value := range paramMap {
		placeholder := ":" + name
		if !strings.Contains(result, placeholder) {
			return "", fmt.Errorf("parameter %s not found in query", name)
		}

		// Format and escape the parameter value based on its type
		formattedValue := formatParameterValue(value)

		// Replace the placeholder with the escaped value
		result = strings.ReplaceAll(result, placeholder, formattedValue)
	}

	return result, nil
}

// formatParameterValue converts a parameter to its SQL representation with proper escaping
func formatParameterValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case string:
		// Escape single quotes by doubling them (SQL standard)
		escaped := strings.ReplaceAll(v, "'", "''")
		return "'" + escaped + "'"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%f", v)
	case bool:
		if v {
			return "TRUE"
		}
		return "FALSE"
	case time.Time:
		// Format as ISO 8601 timestamp
		return "'" + v.Format("2006-01-02 15:04:05") + "'"
	case UnixTimestampDate:
		// Format Unix timestamp as date only (YYYY-MM-DD)
		dateOnly := time.Unix(int64(v), 0).UTC().Format("2006-01-02")
		return "'" + dateOnly + "'"
	default:
		// For complex types, JSON encode them and return as a string
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return "NULL" // Fallback
		}
		jsonStr := string(jsonBytes)
		escaped := strings.ReplaceAll(jsonStr, "'", "''")
		return "'" + escaped + "'"
	}
}

// EmptyResult returns an appropriate empty value based on type
func EmptyResult[T any]() T {
	var result T
	switch any(result).(type) {
	case map[string]any:
		return any(make(map[string]any)).(T)
	case []map[string]any:
		return any(make([]map[string]any, 0)).(T)
	default:
		return result // Zero value
	}
}

// ProcessAthenaQueryIntoJSON executes a query and returns the result as the specified JSON type
func ProcessAthenaQueryIntoJSON[T any](ctx context.Context, query string, params ...Parameter) (T, error) {
	// Execute the query with parameters
	result, err := ExecuteAthenaQuery(ctx, query, params...)
	if err != nil {
		return EmptyResult[T](), err
	}

	// Check if we have results
	if len(result.ResultSet.Rows) <= 1 ||
		len(result.ResultSet.Rows[1].Data) == 0 ||
		result.ResultSet.Rows[1].Data[0].VarCharValue == nil {
		// Return proper empty result for the type
		return EmptyResult[T](), nil
	}

	// Get JSON string and parse
	jsonStr := *result.ResultSet.Rows[1].Data[0].VarCharValue

	var parsedResult T
	if err := json.Unmarshal([]byte(jsonStr), &parsedResult); err != nil {
		log.Error().Err(err).Str("jsonStr", jsonStr).Msg("Failed to parse JSON")
		return EmptyResult[T](), fmt.Errorf("failed to parse JSON: %w", err)
	}

	return parsedResult, nil
}
