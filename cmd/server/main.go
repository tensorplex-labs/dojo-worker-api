package main

import (
	"context"
	"dojo-api/pkg/api"
	"dojo-api/pkg/orm"
	"dojo-api/utils"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Msg("Error loading .env file")
	}

	port := utils.LoadDotEnv("SERVER_PORT")

	router := gin.Default()
	// read allowedOrigins from environment variable which is a comma separated string
	allowedOrigins := strings.Split(utils.LoadDotEnv("CORS_ALLOWED_ORIGINS"), ",")
	allowedOrigins = append(allowedOrigins, "http://localhost")

	log.Info().Msgf("Allowed origins: %v", allowedOrigins)
	config := cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(config))
	api.LoginRoutes(router)
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

	// shutdown tasks
	onShutdown()

	numSeconds := 2
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(numSeconds)*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server Shutdown:")
	}
	// catching ctx.Done(). timeout of 5 seconds.
	<-ctx.Done()
	log.Info().Msgf("timeout of %v seconds.", numSeconds)
	log.Info().Msg("Server exiting")
}

func onShutdown() {
	log.Info().Msg("Shutting down server")
	connHandler := orm.GetConnHandler()
	connHandler.OnShutdown()
}
