// Package main provides a simple load testing tool for HTTP services.
// It supports configurable concurrency, request methods, and JSON payload modification.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/fatih/color"
)

// main is the entry point for the application. It parses command-line flags and optional .env configuration,
// and then starts the load test with the specified parameters.
func main() {
	envPath := flag.String("envpath", "", "📂 Path to the .env file")
	url := flag.String("url", "", "🌐 URL of the service to be tested")
	requests := flag.Int("requests", 100, "📊 Total number of requests")
	concurrency := flag.Int("concurrency", 10, "🚀 Number of simultaneous calls")
	verb := flag.String("verb", "GET", "🔀 HTTP method to use (GET or POST)")
	jsonPath := flag.String("jsonpath", "", "📄 Path to JSON file to use as body for POST requests")
	randIDType := flag.String("rand-id-type", "string", "🔢 Type of random ID to generate (number or string)")
	randIDChrs := flag.Int("rand-id-chrs", 10, "🔤 Number of characters or digits for the random ID")

	flag.Parse()

	// Load .env file if specified
	if *envPath != "" {
		err := godotenv.Load(*envPath)
		if err != nil {
			color.Red("❌ Error loading .env file from %s: %v", *envPath, err)
			return
		}
		color.Cyan("📝 Loaded .env file from %s", *envPath)
	} else {
		color.Cyan("📝 No .env file path provided, skipping .env loading.")
	}

	// Use environment variables if they exist, else fall back to flags
	finalURL := getEnv("URL", *url)
	finalRequests := getEnvAsInt("REQUESTS", *requests)
	finalConcurrency := getEnvAsInt("CONCURRENCY", *concurrency)
	finalVerb := getEnv("VERB", *verb)
	finalJsonPath := getEnv("JSONPATH", *jsonPath)
	finalRandIDType := getEnv("RAND_ID_TYPE", *randIDType)
	finalRandIDChrs := getEnvAsInt("RAND_ID_CHRS", *randIDChrs)

	if finalURL == "" {
		color.Red("❌ The service URL is required. Set it via --url flag or in the .env file.")
		return
	}

	color.Cyan("🏁 Starting the load test for %s...", finalURL)
	runLoadTest(finalURL, finalRequests, finalConcurrency, finalVerb, finalJsonPath, finalRandIDType, finalRandIDChrs)
}

// runLoadTest starts the load test with the specified parameters.
// It uses a goroutine for each worker, sending concurrent requests to the target URL.
func runLoadTest(url string, totalRequests int, concurrencyLevel int, verb string, jsonPath string, randIDType string, randIDChrs int) {
	var wg sync.WaitGroup
	requestsPerWorker := totalRequests / concurrencyLevel
	extraRequests := totalRequests % concurrencyLevel

	results := make(chan int, totalRequests)
	statusCodeCount := make(map[int]int)
	networkErrorCount := 0
	startTime := time.Now()

	for i := 0; i < concurrencyLevel; i++ {
		wg.Add(1)
		go func(requests int) {
			defer wg.Done()
			client := &http.Client{
				Timeout: 30 * time.Second,
			}

			var requestBody []byte

			if verb == "POST" && jsonPath != "" {
				body, err := os.ReadFile(jsonPath)
				if err != nil {
					color.Red("❌ Error reading JSON file: %v", err)
					return
				}
				if randIDType != "" {
					body, err = modifyJSONBody(body, randIDType, randIDChrs)
					if err != nil {
						color.Red("❌ Error modifying JSON body: %v", err)
						return
					}
				}
				requestBody = body
			}

			for j := 0; j < requests; j++ {
				req, err := http.NewRequest(verb, url, bytes.NewBuffer(requestBody))
				if err != nil {
					color.Red("❌ Error creating request: %v", err)
					results <- -1
					continue
				}
				if verb == "POST" {
					req.Header.Set("Content-Type", "application/json")
				}
				resp, err := client.Do(req)
				if err != nil {
					color.Red("❌ Network error: %v", err)
					results <- -1
					continue
				}
				results <- resp.StatusCode
				resp.Body.Close()
			}
		}(requestsPerWorker + boolToInt(i < extraRequests))
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for statusCode := range results {
		if statusCode == -1 {
			networkErrorCount++
		} else {
			statusCodeCount[statusCode]++
		}
	}

	totalTime := time.Since(startTime)

	generateReport(totalTime, totalRequests, statusCodeCount, networkErrorCount)
}

// modifyJSONBody modifies the JSON body by adding a random ID to the object.
// The ID type and length are specified by the parameters.
func modifyJSONBody(body []byte, idType string, length int) ([]byte, error) {
	var jsonObj map[string]interface{}
	err := json.Unmarshal(body, &jsonObj)
	if err != nil {
		return nil, err
	}

	id := generateRandomID(idType, length)
	jsonObj["id"] = id

	modifiedBody, err := json.Marshal(jsonObj)
	if err != nil {
		return nil, err
	}

	return modifiedBody, nil
}

// generateRandomID generates a random ID based on the specified type and length.
// Supported types are "number" and "string".
func generateRandomID(idType string, length int) interface{} {
	rand.Seed(time.Now().UnixNano())
	switch idType {
	case "number":
		id := rand.Intn(int(math.Pow10(length)))
		return id
	case "string":
		const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		id := make([]byte, length)
		for i := range id {
			id[i] = charset[rand.Intn(len(charset))]
		}
		return string(id)
	default:
		return nil
	}
}

// generateReport generates a summary report of the load test results, including
// the total time, successful and failed requests, and the distribution of HTTP status codes.
func generateReport(totalTime time.Duration, totalRequests int, statusCodeCount map[int]int, networkErrorCount int) {
	color.Green("\n===== 📝 Load Test Report =====")
	fmt.Printf("⏳ Total time: %v\n", totalTime)
	fmt.Printf("📊 Total requests: %d\n", totalRequests)
	color.Cyan("✅ Successful requests (HTTP 200): %d\n", statusCodeCount[200])

	delete(statusCodeCount, 200)

	if len(statusCodeCount) > 0 {
		color.Yellow("\n📉 Distribution of other HTTP status codes:")
		for status, count := range statusCodeCount {
			if status >= 400 {
				color.Red("  ❌ Failed requests (HTTP %d): %d", status, count)
			} else {
				fmt.Printf("  - HTTP %d: %d\n", status, count)
			}
		}
	}

	if networkErrorCount > 0 {
		color.Red("\n❌ Network errors: %d", networkErrorCount)
	}

	color.Magenta("\n⚡ Requests per second: %.2f\n", float64(totalRequests)/totalTime.Seconds())
}

// boolToInt converts a boolean to an integer (1 for true, 0 for false).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// getEnv retrieves the value of the environment variable named by the key.
// If the variable is not present, it returns the fallback value.
func getEnv(key string, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvAsInt retrieves the value of the environment variable named by the key and converts it to an integer.
// If the variable is not present or cannot be converted, it returns the fallback value.
func getEnvAsInt(name string, fallback int) int {
	if value, exists := os.LookupEnv(name); exists {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			color.Red("❌ Invalid value for %s in .env file: %v", name, err)
			return fallback
		}
		return intValue
	}
	return fallback
}
