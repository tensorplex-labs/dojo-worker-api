package fixtures

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

type FixtureService struct {
	client *db.PrismaClient
}

// NewFixtureService creates a new FixtureService with the given PrismaClient.
func NewFixtureService(client *db.PrismaClient) *FixtureService {
	return &FixtureService{client: client}
}

const (
	mockHotKey = "mock-hot-key"
	mockSubKey = "mock-sub-key"
)

// resetMinerUser checks if a MinerUser with the given hotkey exists and resets it.
func (o *FixtureService) ResetMinerUser(ctx context.Context) error {
	var txns []db.PrismaTransaction

	// Check if MinerUser with hotkey mockHotKey exists
	existingMinerUser, err := o.client.MinerUser.FindUnique(
		db.MinerUser.Hotkey.Equals(mockHotKey),
	).Exec(ctx)
	if err == nil && existingMinerUser != nil {
		// Delete existing TaskResults linked to this MinerUser
		txns = append(txns, o.client.TaskResult.FindMany(
			db.TaskResult.Task.Where(db.Task.MinerUserID.Equals(existingMinerUser.ID)),
		).Delete().Tx())

		// Delete existing Tasks linked to this MinerUser
		txns = append(txns, o.client.Task.FindMany(
			db.Task.MinerUserID.Equals(existingMinerUser.ID),
		).Delete().Tx())

		// Delete existing WorkerPartners linked to this MinerUser
		txns = append(txns, o.client.WorkerPartner.FindMany(
			db.WorkerPartner.MinerSubscriptionKey.Equals(mockSubKey),
		).Delete().Tx())

		// Delete existing SubscriptionKeys linked to this MinerUser
		txns = append(txns, o.client.SubscriptionKey.FindMany(
			db.SubscriptionKey.MinerUserID.Equals(existingMinerUser.ID),
		).Delete().Tx())

		// Add delete transaction for the existing MinerUser
		txns = append(txns, o.client.MinerUser.FindUnique(
			db.MinerUser.ID.Equals(existingMinerUser.ID),
		).Delete().Tx())
	}

	// Add create transaction for the new MinerUser
	txns = append(txns, o.client.MinerUser.CreateOne(
		db.MinerUser.Hotkey.Set(mockHotKey),
	).Tx())

	// Add create transaction for the new SubscriptionKey
	txns = append(txns, o.client.SubscriptionKey.CreateOne(
		db.SubscriptionKey.Key.Set(mockSubKey),
		db.SubscriptionKey.MinerUser.Link(
			db.MinerUser.Hotkey.Equals(mockHotKey),
		),
	).Tx())

	// Execute all transactions
	if err := o.client.Prisma.Transaction(txns...).Exec(ctx); err != nil {
		log.Error().Err(err).Msg("Error resetting MinerUser")
		return err
	}

	log.Info().Msg("MinerUser reset successfully")
	return nil
}

// createTask is a helper function to create a task with a specified expiration duration.
func (o *FixtureService) CreateDefaultTask(ctx context.Context, title string, expireDuration time.Duration) (*db.TaskModel, error) {
	taskDataJSON, err := loadMockTaskData()
	if err != nil {
		log.Error().Err(err).Msg("Error loading task data")
		return nil, err
	}

	clientWrapper := orm.GetPrismaClient()

	clientWrapper.BeforeQuery()
	defer clientWrapper.AfterQuery()

	expireAt := time.Now().Add(expireDuration)

	createdTask, err := o.client.Task.CreateOne(
		db.Task.ExpireAt.Set(expireAt),
		db.Task.Title.Set(title),
		db.Task.Body.Set("This is a sample task body"),
		db.Task.Type.Set(db.TaskTypeCodeGeneration),
		db.Task.TaskData.Set(types.JSON(taskDataJSON)),
		db.Task.Status.Set(db.TaskStatusInProgress),
		db.Task.MaxResults.Set(10),
		db.Task.NumResults.Set(0),
		db.Task.TotalReward.Set(101.0),
		db.Task.MinerUser.Link(
			db.MinerUser.Hotkey.Equals(mockHotKey),
		),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error creating task")
		return nil, err
	}

	log.Info().Msgf("Task '%s' created with expiration in %v", title, expireDuration)
	return createdTask, nil
}

func loadMockTaskData() ([]byte, error) {
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Construct the full path to the JSON file
	taskDataPath := filepath.Join(cwd, "cmd/seed", "task_data.json")

	// Open the JSON file
	jsonFile, err := os.Open(taskDataPath)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	// Read the contents of the file
	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON data into a map
	var taskData map[string]interface{}
	err = json.Unmarshal(byteValue, &taskData)
	if err != nil {
		return nil, err
	}

	// Marshal the map back to JSON bytes
	taskDataJSON, err := json.Marshal(taskData)
	if err != nil {
		return nil, err
	}

	return taskDataJSON, nil
}
