package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	_ "github.com/lib/pq"
)

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

		line := charts.NewLine()
		line.SetGlobalOptions(
			charts.WithInitializationOpts(opts.Initialization{Theme: types.ThemeMacarons}),
			charts.WithTitleOpts(opts.Title{
				Title:    "Temperature char",
				Subtitle: "My temp chart",
			}),
			charts.WithLegendOpts(opts.Legend{
				Show:   true,
				Bottom: "5px",
				TextStyle: &opts.TextStyle{
					Color: "#eee",
				},
			}),
		)
		itemsTInside := make([]opts.LineData, 0)
		itemsTOutside := make([]opts.LineData, 0)
		xaxis := make([]string, 0)
		for rows.Next() {
			var timestamp time.Time
			var tempInside, tempOutside float32
			isEmpty = false

			if err := rows.Scan(&timestamp, &tempInside, &tempOutside); err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error scanning data: %q", err))
				return
			}

			itemsTInside = append(itemsTInside, opts.LineData{Value: tempInside})
			itemsTOutside = append(itemsTOutside, opts.LineData{Value: tempOutside})
			xaxis = append(xaxis, fmt.Sprintf("%02d:%02d:%02d", timestamp.Hour(), timestamp.Minute(), timestamp.Second()))
		}
		line.SetXAxis(xaxis).AddSeries("Inside", itemsTInside).
			SetXAxis(xaxis).AddSeries("Outside", itemsTOutside).
			SetSeriesOptions(
				charts.WithLineChartOpts(opts.LineChart{Smooth: true}),
				charts.WithLabelOpts(opts.Label{Show: true}),
			)

		if isEmpty {
			c.String(http.StatusOK, "No data available")
		} else {
			html := new(bytes.Buffer)
			line.Render(html)

			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Write([]byte(html.String()))
		}
	}
}

func tempPush(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := maybeCreateTempDB(db); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating table: %q", err))
			return
		}

		tempInside, err := strconv.ParseFloat(c.Query("inside"), 32)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("Wrong param \"inside\": %q", err))
			return
		}
		tempOutside, err := strconv.ParseFloat(c.Query("outside"), 32)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("Wrong param \"outside\": %q", err))
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
	router.GET("/", tempGet(db))
	router.POST("/pushtemp", tempPush(db))
	router.GET("/pushtemp", tempPush(db))

	router.Run(":" + port)
}
