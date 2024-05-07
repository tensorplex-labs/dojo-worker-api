package orm

import (
	"dojo-api/db"
	"dojo-api/utils"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

type PrismaClientWrapper struct {
	QueryTracker *QueryTracker
	Client       *db.PrismaClient
}

type QueryTracker struct {
	activeQueries int32
}

func (qt *QueryTracker) BeforeQuery() {
	atomic.AddInt32(&qt.activeQueries, 1)
}

func (qt *QueryTracker) AfterQuery() {
	atomic.AddInt32(&qt.activeQueries, -1)
}

func (qt *QueryTracker) WaitForAllQueries() {
	for atomic.LoadInt32(&qt.activeQueries) > 0 {
		time.Sleep(100 * time.Millisecond)
	}
}

type ConnHandler struct {
	clientWrappers map[string]PrismaClientWrapper
	mu             sync.Mutex
}

var connHandler *ConnHandler
var once sync.Once

func GetConnHandler() *ConnHandler {
	once.Do(func() {
		connHandler = &ConnHandler{
			clientWrappers: make(map[string]PrismaClientWrapper),
		}
	})
	return connHandler
}

func GetPrismaClient() *PrismaClientWrapper {
	handler := GetConnHandler()
	credentials := getPostgresCredentials()
	currentConnString := buildPostgresConnString(credentials.username, credentials.password, credentials.url)
	handler.mu.Lock()
	defer handler.mu.Unlock()

	clientWrapper, exists := handler.clientWrappers[currentConnString]
	if !exists {
		// Create a new client and connect
		clientWrapper = PrismaClientWrapper{
			QueryTracker: &QueryTracker{},
			Client:       db.NewClient(getPrismaConfig()),
		}
		if err := clientWrapper.Client.Prisma.Connect(); err != nil {
			log.Fatal().Err(err).Msg("Failed to connect to Prisma client")
			return nil
		}
	}

	// remove older clients since probably invalid now
	for connString, existingWrapper := range handler.clientWrappers {
		if connString != currentConnString {
			existingWrapper.QueryTracker.WaitForAllQueries()
			if existingWrapper.QueryTracker.activeQueries == 0 {
				if err := existingWrapper.Client.Prisma.Disconnect(); err != nil {
					log.Error().Err(err).Msg("Failed to disconnect from Prisma client")
				}
				delete(handler.clientWrappers, connString)
			} else {
				log.Warn().Msgf("Not disconnecting from Prisma client with connection string: %s because there are still active queries", connString)
			}
		}
	}
	return &clientWrapper
}

func (h *ConnHandler) OnShutdown() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for connString, clientWrapper := range h.clientWrappers {
		if err := clientWrapper.Client.Prisma.Disconnect(); err != nil {
			log.Error().Err(err).Msgf("Failed to disconnect from Prisma client with connection string: %s", connString)
		} else {
			log.Info().Msgf("Disconnected from Prisma client with connection string: %s", connString)
		}
		delete(h.clientWrappers, connString)
	}
	return nil
}

type DbSecrets struct {
	username string
	password string
	url      string
}

func getPostgresCredentials() DbSecrets {
	url := utils.LoadDotEnv("DB_URL")
	username := utils.LoadDotEnv("DB_USERNAME")
	password := utils.LoadDotEnv("DB_PASSWORD")
	return DbSecrets{
		username: username,
		password: password,
		url:      url,
	}
}

func buildPostgresConnString(user string, password string, url string) string {
	return "postgresql://" + user + ":" + password + "@" + url + "/postgres"
}

func getPrismaConfig() func(*db.PrismaConfig) {
	secrets := getPostgresCredentials()
	prismaConfig := db.WithDatasourceURL(buildPostgresConnString(secrets.username, secrets.password, secrets.url))
	return prismaConfig
}
