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

var charthtml string

func maybeCreateTempDB(db *sql.DB) error {
	_, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS tempdata(
			time timestamp PRIMARY KEY,
			tempInside real NOT NULL,
			tempOutside real NOT NULL,
			humidity real NOT NULL
		)`)

	return err
}

func dropDB(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, err := db.Exec("DROP TABLE tempdata")
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Can't drop table: %q", err))
			return
		}

		charthtml = ""
		c.String(http.StatusOK, "Drop ok")
	}
}

func lastTemp(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := maybeCreateTempDB(db); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating table: %q", err))
			return
		}

		rows, err := db.Query(`SELECT time FROM tempdata WHERE time=(SELECT max(time) FROM tempdata)`)

		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error selecting data: %q", err))
			return
		}

		defer rows.Close()
		if !rows.Next() {
			c.String(http.StatusOK, "No data available")
			return
		}

		var timestamp1 time.Time
		if err := rows.Scan(&timestamp1); err != nil {
			c.String(http.StatusInternalServerError, "Failed to scan data: %q", err)
			return
		}

		timestamp2 := time.Now()
		t := timestamp2.Sub(timestamp1)
		c.String(http.StatusOK, fmt.Sprintf("Last update was %02d:%02d:%02d before", int(t.Hours()), int(t.Minutes()), int(t.Seconds())))
	}
}

func tempGet(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := maybeCreateTempDB(db); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating table: %q", err))
			return
		}

		rows, err := db.Query(`SELECT time, tempInside, tempOutside, humidity FROM tempdata`)

		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error selecting data: %q", err))
			return
		}

		defer rows.Close()

		isEmpty := true

		if len(charthtml) == 0 {
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
						Color: "#000",
					},
				}),
				charts.WithYAxisOpts(opts.YAxis{
					Min: "dataMin",
					Max: "dataMax",
				}),
			)
			itemsTInside := make([]opts.LineData, 0)
			itemsTOutside := make([]opts.LineData, 0)
			itemsHum := make([]opts.LineData, 0)
			xaxis := make([]string, 0)
			for rows.Next() {
				var timestamp time.Time
				var tempInside, tempOutside, humidity float64
				isEmpty = false

				if err := rows.Scan(&timestamp, &tempInside, &tempOutside, &humidity); err != nil {
					c.String(http.StatusInternalServerError, fmt.Sprintf("Error scanning data: %q", err))
					return
				}

				if tempInside < 2 || tempInside > 40 || tempOutside < 2 || tempOutside > 40 || humidity <= 0 || humidity > 100 {
					continue
				}

				itemsTInside = append(itemsTInside, opts.LineData{Value: tempInside})
				itemsTOutside = append(itemsTOutside, opts.LineData{Value: tempOutside})
				itemsHum = append(itemsHum, opts.LineData{Value: humidity})
				xaxis = append(xaxis, fmt.Sprintf("%02d:%02d:%02d", (timestamp.Hour()+3)%24, timestamp.Minute(), timestamp.Second()))
			}
			line.SetXAxis(xaxis).AddSeries("Inside", itemsTInside).
				SetXAxis(xaxis).AddSeries("Outside", itemsTOutside).
				SetSeriesOptions(
					charts.WithLineChartOpts(opts.LineChart{Smooth: true}),
				)
			line.
				SetXAxis(xaxis).AddSeries("Humidity", itemsHum).
				SetSeriesOptions(
					charts.WithLineChartOpts(opts.LineChart{Smooth: true, YAxisIndex: 1}),
				)

			html := new(bytes.Buffer)
			line.Render(html)
			charthtml = html.String()
		} else {
			isEmpty = false
		}

		if isEmpty {
			c.String(http.StatusOK, "No data available")
		} else {
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Write([]byte(charthtml))
		}
	}
}

func tempDump(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := maybeCreateTempDB(db); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating table: %q", err))
			return
		}

		rows, err := db.Query(`SELECT time, tempInside, tempOutside, humidity FROM tempdata`)

		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error selecting data: %q", err))
			return
		}

		defer rows.Close()

		isEmpty := true
		var tempInsideDump string
		var tempOutsideDump string
		var humDump string

		for rows.Next() {
			var timestamp time.Time
			var tempInside, tempOutside, hum float64
			isEmpty = false

			if err := rows.Scan(&timestamp, &tempInside, &tempOutside, &hum); err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error scanning data: %q", err))
				return
			}

			if tempInside < 2 || tempInside > 40 || tempOutside < 2 || tempOutside > 40 || hum <= 0 || hum > 100 {
				continue
			}

			tempInsideDump += fmt.Sprintf(" %0.2f", tempInside)
			tempOutsideDump += fmt.Sprintf(" %0.2f", tempOutside)
			humDump += fmt.Sprintf(" %0.2f", hum)
		}

		if isEmpty {
			c.String(http.StatusOK, "No data available")
		} else {
			c.String(http.StatusOK, tempInsideDump+"\n"+tempOutsideDump+"\n"+humDump)
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
		humidity, err := strconv.ParseFloat(c.Query("humidity"), 32)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("Wrong param \"humidity\": %q", err))
			return
		}

		if _, err := db.Exec("INSERT INTO tempdata VALUES($1, $2, $3, $4)", time.Now(), tempInside, tempOutside, humidity); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error inserting data: %q", err))
			return
		}

		charthtml = ""
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
	router.GET("/gettemp", tempDump(db))
	router.POST("/pushtemp", tempPush(db))
	router.GET("/pushtemp", tempPush(db))
	router.POST("/resettemp", dropDB(db))
	router.GET("/resettemp", dropDB(db))
	router.GET("/lastupdate", lastTemp(db))

	router.Run(":" + port)
}
