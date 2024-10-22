package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dojo-api/pkg/api"
	"dojo-api/pkg/cache"
	"dojo-api/pkg/orm"
	"dojo-api/utils"

	_ "dojo-api/docs"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title			Dojo Worker API
// @version		1.0
// @description	This is the worker API for the Dojo project.

func main() {
	loadEnvVars()
	go continuouslyReadEnv()
	go orm.NewTaskORM().UpdateExpiredTasks(context.Background())

	runtimeEnv := utils.LoadDotEnv("RUNTIME_ENV")
	if runtimeEnv == "aws" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	port := utils.LoadDotEnv("SERVER_PORT")
	// read allowedOrigins from environment variable which is a comma separated string
	allowedOrigins := strings.Split(utils.LoadDotEnv("CORS_ALLOWED_ORIGINS"), ",")

	log.Info().Msgf("Allowed origins: %v", allowedOrigins)
	config := cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-KEY"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
		AllowWildcard:    true,
	}

	api.InitializeLimiters()
	log.Info().Msg("Rate limiters initialized")

	router := gin.New()                          // empty engine
	router.Use(gin.Recovery())                   // add recovery middleware
	router.Use(api.CustomGinLogger(&log.Logger)) // add our custom gin logger

	router.Use(cors.New(config))
	router.Use(api.GenerousRateLimiter())
	router.ForwardedByClientIP = true
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

	done := make(chan bool, 1)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-quit
		log.Info().Msgf("Received signal: %s. Shutting down...", sig)

		numSeconds := 5 // Increased timeout for graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(numSeconds)*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Server Shutdown:")
		}

		onShutdown()

		close(done)
	}()

	log.Info().Msgf("Server starting on port %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server startup failed")
	}

	<-done
	log.Info().Msg("Server exiting")
}

func onShutdown() {
	log.Info().Msg("Performing shutting down server")
	connHandler := orm.GetConnHandler()
	connHandler.OnShutdown()
	cache := cache.GetCacheInstance()
	cache.Shutdown()
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
