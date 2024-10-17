package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"dojo-api/db"
	"dojo-api/pkg/orm"

	"github.com/rs/zerolog/log"
	"github.com/steebchen/prisma-client-go/runtime/types"
)

func main() {
	client := db.NewClient()
	if err := client.Prisma.Connect(); err != nil {
		log.Error().Err(err).Msg("Failed to connect to database")
		return
	}
	defer client.Prisma.Disconnect()

	seedData(client)
}

func seedData(client *db.PrismaClient) {
	mockHotKey := "5F4tQyWrhfGVcNhoqeiNsR6KjD4wMZ2kfhLj4oHYuyHb_123"
	mockSubKey := "sk-456"
	ctx := context.Background()
	clientWrapper := orm.GetPrismaClient()

	clientWrapper.BeforeQuery()
	defer clientWrapper.AfterQuery()

	// Begin transaction
	var txns []db.PrismaTransaction

	// Check if MinerUser with hotkey mockHotKey  exists
	existingMinerUser, err := client.MinerUser.FindUnique(
		db.MinerUser.Hotkey.Equals(mockHotKey),
	).Exec(ctx)
	if err == nil && existingMinerUser != nil {
		// Delete existing tasks linked to this MinerUser
		txns = append(txns, client.Task.FindMany(
			db.Task.MinerUserID.Equals(existingMinerUser.ID),
		).Delete().Tx())

		// Delete existing WorkerPartners linked to this MinerUser
		txns = append(txns, client.WorkerPartner.FindMany(
			db.WorkerPartner.MinerSubscriptionKey.Equals(mockSubKey),
		).Delete().Tx())

		// Delete existing SubscriptionKeys linked to this MinerUser
		txns = append(txns, client.SubscriptionKey.FindMany(
			db.SubscriptionKey.MinerUserID.Equals(existingMinerUser.ID),
		).Delete().Tx())

		// Add delete transaction for the existing MinerUser
		txns = append(txns, client.MinerUser.FindUnique(
			db.MinerUser.ID.Equals(existingMinerUser.ID),
		).Delete().Tx())
	}

	// Add create transaction for the new MinerUser
	txns = append(txns, client.MinerUser.CreateOne(
		db.MinerUser.Hotkey.Set(mockHotKey),
	).Tx())

	txns = append(txns, client.SubscriptionKey.CreateOne(
		db.SubscriptionKey.Key.Set(mockSubKey),
		db.SubscriptionKey.MinerUser.Link(
			db.MinerUser.Hotkey.Equals(mockHotKey),
		),
	).Tx())

	// Print the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get current working directory")
		return
	}

	taskDataPath := filepath.Join(cwd, "cmd/seed", "task_data.json")
	// Open the jsonFile
	jsonFile, err := os.Open(taskDataPath)
	if err != nil {
		log.Error().Err(err).Msg("Error opening task_data.json")
		return
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		log.Error().Err(err).Msg("Error reading task_data.json")
		return
	}

	var taskData map[string]interface{}
	err = json.Unmarshal(byteValue, &taskData)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal task data")
		return
	}

	// Convert taskData to types.JSON
	taskDataJSON, err := json.Marshal(taskData)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal task data")
		return
	}

	// Add create transaction for the new task
	txns = append(txns, client.Task.CreateOne(
		db.Task.ExpireAt.Set(time.Now().Add(24*7*time.Hour)),
		db.Task.Title.Set("Mock Task1"),
		db.Task.Body.Set("This is a sample task body"),
		db.Task.Type.Set(db.TaskTypeCodeGeneration),
		db.Task.TaskData.Set(types.JSON(taskDataJSON)),
		db.Task.Status.Set(db.TaskStatusInProgress),
		db.Task.MaxResults.Set(10),
		db.Task.NumResults.Set(0),
		db.Task.NumCriteria.Set(0),
		db.Task.TotalReward.Set(101.0),
		db.Task.MinerUser.Link(
			db.MinerUser.Hotkey.Equals(mockHotKey),
		),
	).Tx())

	// Execute all transactions
	if err := client.Prisma.Transaction(txns...).Exec(ctx); err != nil {
		log.Error().Err(err).Msg("Error executing transaction")
		return
	}

	log.Info().Msg("Successfully seeded data !!!")
}
