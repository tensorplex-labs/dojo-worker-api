package main

import (
	"context"
	"dojo-api/pkg/api"
	"dojo-api/pkg/orm"
	"dojo-api/utils"
	"net/http"
	"os"
	"os/signal"
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
	router.Use(cors.Default())
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server Shutdown:")
	}
	// catching ctx.Done(). timeout of 5 seconds.
	<-ctx.Done()
	log.Info().Msg("timeout of 5 seconds.")
	log.Info().Msg("Server exiting")
}

func onShutdown() {
	log.Info().Msg("Shutting down server")
	connHandler := orm.GetConnHandler()
	connHandler.OnShutdown()
}
