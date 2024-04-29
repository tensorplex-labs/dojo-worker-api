package orm

import (
	"dojo-api/db"
	"log"
)

func NewPrismaClient() *db.PrismaClient {
	client := db.NewClient()
	if err := client.Prisma.Connect(); err != nil {
		log.Fatal("Failed to connect to Prisma client: ", err)
		return nil
	}
	return client
}
