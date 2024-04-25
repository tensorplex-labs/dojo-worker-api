package main

import (
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "dojo-api/pkg/api"
    "dojo-api/db"
    "context"
    "time"
    "encoding/json"
    "fmt"
    "log"
    "os"
)

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Error loading .env file")
    }

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080" // Default port if not specified
    }

    r := gin.Default()
    api.LoginRoutes(r)
    r.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "message": "Hello, this is dojo-go-api",
        })
    })
    r.Run(":" + port)
}

func run() error {
	client := db.NewClient()
	if err := client.Prisma.Connect(); err != nil {
	  return err
	}
   
	defer func() {
	  if err := client.Prisma.Disconnect(); err != nil {
		panic(err)
	  }
	}()
   
	ctx := context.Background()
   
	// create a post
	createdPost, err := client.NetworkUser.CreateOne(
		db.NetworkUser.Coldkey.Set("your-coldkey"),
		db.NetworkUser.Hotkey.Set("your-hotkey"),
		db.NetworkUser.APIKey.Set("test-123435646"),
		db.NetworkUser.KeyExpireAt.Set(time.Now().Add(24 * time.Hour)),
		db.NetworkUser.IPAddress.Set("127.0.0.1"),
		db.NetworkUser.UserType.Set("MINER"),
		db.NetworkUser.IsVerified.Set(true),
		db.NetworkUser.ID.Set("my-post"),
		db.NetworkUser.CreatedAt.Set(time.Now()),
		db.NetworkUser.UpdatedAt.Set(time.Now()),
	  ).Exec(ctx)
	if err != nil {
	  return err
	}
   
	result, _ := json.MarshalIndent(createdPost, "", "  ")
	fmt.Printf("created post: %s\n", result)
   
	// find a single post
	post, err := client.NetworkUser.FindUnique(
	  db.NetworkUser.ID.Equals(createdPost.ID),
	).Exec(ctx)
	if err != nil {
	  return err
	}
   
	result, _ = json.MarshalIndent(post, "", "  ")
	fmt.Printf("post: %s\n", result)
   
	// for optional/nullable values, you need to check the function and create two return values
	// `desc` is a string, and `ok` is a bool whether the record is null or not. If it's null,
	// `ok` is false, and `desc` will default to Go's default values; in this case an empty string (""). Otherwise,
	// `ok` is true and `desc` will be "my description".
   
	fmt.Printf("The posts's description is: %s\n", post.APIKey)
   
	return nil
  }