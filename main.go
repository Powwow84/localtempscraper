package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly"
)

type WeatherData struct {
	Current string
	ExpectedHigh string
	ExpectedLow string
}

var prometheusOutput string
var mu sync.Mutex

func transformCSVToPrometheus(weatherData []WeatherData) (string, error) {
	output := ""

	for _, data := range weatherData {
	output += fmt.Sprintf("current_temp{location=\"Las Vegas\"} %s\n", data.Current)
	output += fmt.Sprintf("expected_high{location=\"Las Vegas\"} %s\n", data.ExpectedHigh)
	output += fmt.Sprintf("expected_low{location=\"Las Vegas\"} %s\n", data.ExpectedLow)
	}

	return output, nil
}

func scrapeAndStore() {
	var current, expectedHigh, expectedLow string
	weatherData := make([]WeatherData, 0)
	c := colly.NewCollector(colly.AllowedDomains("weather.com", "www.weather.com"))

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	c.OnHTML("span.CurrentConditions--tempValue--MHmYY", func(e *colly.HTMLElement) {
		// fmt.Println(e.ChildText("span.CurrentConditions--tempValue--MHmYY"))
		current = strings.Replace(e.Text,  "°", "", -1)
		fmt.Println(current)
	})

	c.OnHTML("div.CurrentConditions--tempHiLoValue--3T1DG  span", func(e *colly.HTMLElement) {
		if expectedHigh == "" {
			expectedHigh = strings.Replace(e.Text, "°", "", -1)
		} else if expectedLow == "" {
			expectedLow = strings.Replace(e.Text, "°", "", -1)
		}
	


		fmt.Println(expectedHigh)
		fmt.Println(expectedLow)

		if expectedHigh != "" && expectedLow != "" {
			_, errHigh := strconv.ParseFloat(expectedHigh, 64)
			_, errLow := strconv.ParseFloat(expectedLow, 64)
			_, errCurrent := strconv.ParseFloat(current, 64)

			if errHigh == nil && errLow == nil && errCurrent == nil {
				weatherData = append(weatherData, WeatherData{Current: current, ExpectedHigh: expectedHigh, ExpectedLow: expectedLow})
				// Reset expectedHigh and expectedLow for next pair of spans
				expectedHigh = ""
				expectedLow = ""
			}
		}
	})

	err := c.Visit("https://weather.com/weather/today/l/fe3c78e80c47c404a4e64ec7c86ceccdb814894cedefdb528f9d8d95c3e4eb74")
	if err != nil {
		log.Fatal(err)
	}

	prometheusOutput, err = transformCSVToPrometheus(weatherData)
	if err != nil {
		log.Fatal((err))
	}

	fmt.Println("Scraped and transformed data at", time.Now())
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprint(w, prometheusOutput)
}

func main() {

	scrapeAndStore()
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			scrapeAndStore()
		}
	}()

	http.HandleFunc("/metrics", metricsHandler)
	http.ListenAndServe(":8080", nil)
}