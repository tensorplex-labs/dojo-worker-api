package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "dojo-api/docs"
	"dojo-api/pkg/api"
	"dojo-api/pkg/cache"
	"dojo-api/pkg/orm"
	"dojo-api/pkg/sandbox"
	"dojo-api/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

//	@title			Dojo Worker API
//	@version		1.0
//	@description	This is the worker API for the Dojo project.

func main() {
	loadEnvVars()
	go continuouslyReadEnv()
	go orm.NewTaskORM().UpdateExpiredTasks(context.Background())
	port := utils.LoadDotEnv("SERVER_PORT")
	router := gin.Default()
	// read allowedOrigins from environment variable which is a comma separated string
	allowedOrigins := strings.Split(utils.LoadDotEnv("CORS_ALLOWED_ORIGINS"), ",")
	allowedOrigins = append(allowedOrigins, "http://localhost*")

	log.Info().Msgf("Allowed origins: %v", allowedOrigins)
	config := cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-KEY"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
		AllowWildcard:    true,
	}
	router.Use(cors.New(config))
	api.LoginRoutes(router)

	if os.Getenv("RUNTIME_ENV") == "local" {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello, this is dojo-go-api",
		})
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		// service connections
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("listen")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Msgf("Received signal: %s. Shutting down...", sig)

	numSeconds := 2
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(numSeconds)*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server Shutdown:")
		// shutdown tasks
		onShutdown()
	}
	// catching ctx.Done(). timeout of 5 seconds.
	<-ctx.Done()
	log.Info().Msg("Server exiting")
}

func onShutdown() {
	log.Info().Msg("Shutting down server")
	connHandler := orm.GetConnHandler()
	connHandler.OnShutdown()
	cache := cache.GetCacheInstance()
	cache.Shutdown()
	browser := sandbox.GetBrowser()
	browser.Close()
}

func loadEnvVars() {
	// we need this to grab latest env varsfrom .env
	err := godotenv.Overload()
	if err != nil {
		log.Error().Err(err).Msg("Error loading .env file")
	}
}

func continuouslyReadEnv() {
	loadEnvVars()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		log.Debug().Msg("Reloading & overloading .env file")
		loadEnvVars()
	}
}
