package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func resp(c *gin.Context) {
	c.String(http.StatusOK, "Hello, world!")
}

func maybeCreateTempDB(db *sql.DB) error {
	_, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS tempdata(
			time timestamp PRIMARY KEY,
			tempInside real NOT NULL,
			tempOutside real NOT NULL
		)`)

	return err
}

func tempGet(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := maybeCreateTempDB(db); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating table: %q", err))
			return
		}

		rows, err := db.Query(`SELECT time, tempInside, tempOutside FROM tempdata`)

		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error selecting data: %q", err))
			return
		}

		defer rows.Close()

		isEmpty := true

		for rows.Next() {
			var timestamp time.Time
			var tempInside, tempOutside float32
			isEmpty = true

			if err := rows.Scan(&timestamp, &tempInside, &tempOutside); err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error scanning data: %q", err))
				return
			}

			c.String(http.StatusOK, fmt.Sprintf("%s %.1f %.1f", timestamp.String(), tempInside, tempOutside))
		}

		if isEmpty {
			c.String(http.StatusOK, "No data available")
		}
	}
}

func tempPush(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := maybeCreateTempDB(db); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating table: %q", err))
			return
		}

		tempInside, err := strconv.ParseFloat(c.Request.URL.Query().Get("inside"), 32)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("Wrong tempInside: %q", err))
			return
		}
		tempOutside, err := strconv.ParseFloat(c.Request.URL.Query().Get("outside"), 32)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("Wrong tempOutside: %q", err))
			return
		}

		if _, err := db.Exec("INSERT INTO tempdata VALUES($1, $2, $3)", time.Now(), tempInside, tempOutside); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error inserting data: %q", err))
			return
		}

		c.String(http.StatusOK, "Data added")
	}
}

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Error opening database: %q", err)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.GET("/", resp)
	router.GET("/gettemp", tempGet(db))
	router.POST("/pushtemp", tempPush(db))

	router.Run(":" + port)
}
