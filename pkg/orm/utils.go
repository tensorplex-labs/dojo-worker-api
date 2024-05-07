package orm

import (
	"dojo-api/db"
	"dojo-api/utils"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

type PrismaClientWrapper struct {
	QueryTracker *QueryTracker
	Client       *db.PrismaClient
}

func (p *PrismaClientWrapper) BeforeQuery() {
	p.QueryTracker.BeforeQuery()
}

func (p *PrismaClientWrapper) AfterQuery() {
	p.QueryTracker.AfterQuery()
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
	currentConnString := buildPostgresConnString(credentials.username, credentials.password)
	handler.mu.Lock()
	defer handler.mu.Unlock()

	existingWrapper, exists := handler.clientWrappers[currentConnString]
	if exists {
		log.Info().Msg("Reusing existing Prisma client for connString " + currentConnString)
		return &existingWrapper
	}

	clientWrapper := PrismaClientWrapper{
		QueryTracker: &QueryTracker{},
		Client:       db.NewClient(getPrismaConfig()),
	}
	defer func() {
		if r := recover(); r != nil {
			log.Error().Msgf("Recovered from panic while connecting to Prisma client: %v", r)
		}
		// // failed to use latest connection string, try to connect to any other client
		// for _, wrapper := range handler.clientWrappers {
		// 	log.Info().Interface("clientWrapper", wrapper).Msg("Existing client wrapper")
		// }

		// log.Error().Err(err).Msg("Failed to connect to Prisma client")
		// for key, wrapper := range handler.clientWrappers {
		// 	log.Info().Str("connString", key).Interface("clientWrapper", wrapper).Msg("Trying to connect to another Prisma client")
		// 	return &wrapper
		// }
		// return nil
	}()

	err := clientWrapper.Client.Prisma.Connect()
	if err == nil {
		log.Info().Msg("Successfully connected for new connection string")
		handler.clientWrappers[currentConnString] = clientWrapper
		return &clientWrapper
	}

	log.Warn().Msg("Failed to connect Prisma client for new connection string, attempting reuse...")
	for _, wrapper := range handler.clientWrappers {
		log.Info().Msg("Reusing existing Prisma client")
		return &wrapper
	}
	log.Error().Err(err).Msg("No existing Prisma clients to reuse")
	return nil

	// // remove older clients since probably invalid now
	// for connString, existingWrapper := range handler.clientWrappers {
	// 	if connString != currentConnString {
	// 		existingWrapper.QueryTracker.WaitForAllQueries()
	// 		if existingWrapper.QueryTracker.activeQueries == 0 {
	// 			if err := existingWrapper.Client.Prisma.Disconnect(); err != nil {
	// 				log.Error().Err(err).Msg("Failed to disconnect from Prisma client")
	// 			}
	// 			delete(handler.clientWrappers, connString)
	// 		} else {
	// 			log.Warn().Msgf("Not disconnecting from Prisma client with connection string: %s because there are still active queries", connString)
	// 		}
	// 	}
	// }
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
	username := utils.LoadDotEnv("DB_USERNAME")
	password := utils.LoadDotEnv("DB_PASSWORD")
	return DbSecrets{
		username: username,
		password: password,
	}
}

func buildPostgresConnString(user string, password string) string {
	url := utils.LoadDotEnv("DB_URL")
	dbName := utils.LoadDotEnv("DB_NAME")
	databaseUrl := "postgresql://" + user + ":" + password + "@" + url + "/" + dbName
	// hack this so Prisma can read it directly, handle complexities here
	os.Setenv("DATABASE_URL", databaseUrl)
	return databaseUrl
}

func getPrismaConfig() func(*db.PrismaConfig) {
	secrets := getPostgresCredentials()
	prismaConfig := db.WithDatasourceURL(buildPostgresConnString(secrets.username, secrets.password))
	return prismaConfig
}
