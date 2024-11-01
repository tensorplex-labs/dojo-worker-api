package main

import (
	"context"
	"os"
	"time"

	"dojo-api/cmd/seed/fixtures"
	"dojo-api/db"

	"github.com/rs/zerolog/log"
)

/*
Usage:
This script is used to manage and generate tasks in the database.
Run the script with one of the following commands:

go run cmd/seed/main.go reset
  - Resets the MinerUser and creates a single default task.

go run cmd/seed/main.go gen-task-expired
  - Generates tasks that are already expired.

go run cmd/seed/main.go gen-task-short
  - Generates tasks with a short expiration time.

go run cmd/seed/main.go gen-task-normal
  - Generates tasks with a normal expiration time.
*/

func main() {
	// Check if an action argument is provided
	if len(os.Args) < 2 {
		log.Error().Msg("No action provided. Use 'reset', 'gen-task-expired', 'gen-task-short', or 'gen-task-normal'")
		return
	}

	// Get the action from command-line arguments
	taskType := os.Args[1]

	// Initialize the database client
	client := db.NewClient()
	if err := client.Prisma.Connect(); err != nil {
		log.Error().Err(err).Msg("Failed to connect to database")
		return
	}
	defer client.Prisma.Disconnect()

	// Set up the context
	ctx := context.Background()

	// Execute the appropriate function based on the command-line argument
	switch taskType {
	case "reset":
		resetMinerUserAndCreateTask(client, ctx)
	case "gen-task-expired":
		generateExpiredTasks(client, ctx)
	case "gen-task-short":
		generateShortExpireTasks(client, ctx)
	case "gen-task-normal":
		generateNormalExpireTasks(client, ctx)
	default:
		log.Error().Msg("Unknown task type. Use 'reset', 'gen-task-expired', 'gen-task-short', or 'gen-task-normal'")
	}
}

// Function to reset MinerUser and create a single default task
func resetMinerUserAndCreateTask(client *db.PrismaClient, ctx context.Context) {
	fixtureService := fixtures.NewFixtureService(client)

	// Reset the MinerUser
	if err := fixtureService.ResetMinerUser(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to reset MinerUser")
		return
	}

	// Create a single default task
	title := "Default Mock Task"
	expireDuration := 6 * time.Hour
	if _, err := fixtureService.CreateDefaultTask(ctx, title, expireDuration); err != nil {
		log.Error().Err(err).Msg("Failed to create default task")
		return
	}

	log.Info().Msg("Reset MinerUser and created a single default task successfully")
}

// Function to generate expired tasks
func generateExpiredTasks(client *db.PrismaClient, ctx context.Context) {
	fixtureService := fixtures.NewFixtureService(client)

	for i := 0; i < 3; i++ {
		title := "Expired Task"
		expireDuration := -6 * time.Hour
		if _, err := fixtureService.CreateDefaultTask(ctx, title, expireDuration); err != nil {
			log.Error().Err(err).Msg("Failed to create expired task")
			return
		}
	}

	log.Info().Msg("Expired tasks created successfully")
}

// Function to generate tasks with short expiration
func generateShortExpireTasks(client *db.PrismaClient, ctx context.Context) {
	fixtureService := fixtures.NewFixtureService(client)

	for i := 0; i < 3; i++ {
		title := "Task with Short Expiration"
		expireDuration := 5 * time.Minute
		if _, err := fixtureService.CreateDefaultTask(ctx, title, expireDuration); err != nil {
			log.Error().Err(err).Msg("Failed to create short expiration task")
			return
		}
	}

	log.Info().Msg("Short expiration tasks created successfully")
}

// Function to generate tasks with normal expiration
func generateNormalExpireTasks(client *db.PrismaClient, ctx context.Context) {
	fixtureService := fixtures.NewFixtureService(client)

	for i := 0; i < 3; i++ {
		title := "Task with Normal Expiration"
		expireDuration := 6 * time.Hour
		if _, err := fixtureService.CreateDefaultTask(ctx, title, expireDuration); err != nil {
			log.Error().Err(err).Msg("Failed to create normal expiration task")
			return
		}
	}

	log.Info().Msg("Normal expiration tasks created successfully")
}
