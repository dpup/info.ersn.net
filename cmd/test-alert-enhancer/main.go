package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/dpup/info.ersn.net/server/internal/lib/alerts"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "enhance-alert":
		handleEnhanceAlert()
	case "test-connection":
		handleTestConnection()
	case "test-prompt":
		handleTestPrompt()
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func handleEnhanceAlert() {
	fs := flag.NewFlagSet("enhance-alert", flag.ExitOnError)
	description := fs.String("description", "", "Raw Caltrans alert description")
	location := fs.String("location", "", "Alert location context")
	styleUrl := fs.String("style-url", "", "KML styleUrl for closure type (e.g., #lcs, #oneWayTrafficPath)")
	apiKey := fs.String("api-key", os.Getenv("PF__OPENAI__API_KEY"), "OpenAI API key (or set PF__OPENAI__API_KEY env var)")
	model := fs.String("model", "gpt-3.5-turbo", "OpenAI model to use")
	timeout := fs.Int("timeout", 30, "Timeout in seconds")

	fs.Parse(os.Args[2:])

	if *description == "" {
		fmt.Println("Example usage:")
		fmt.Println("  test-alert-enhancer enhance-alert --description \"Rte 4 EB of MM 31 - VEHICLE IN DITCH, EMS ENRT\" --location \"Highway 4\"")
		fmt.Println("  test-alert-enhancer enhance-alert --description \"raw text\" --location \"Hwy 4\" --api-key sk-xxx")
		os.Exit(1)
	}

	if *apiKey == "" {
		log.Fatal("OpenAI API key is required. Set PF__OPENAI__API_KEY environment variable or use --api-key flag")
	}

	enhancer := alerts.NewAlertEnhancer(*apiKey, *model)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	rawAlert := alerts.RawAlert{
		ID:          "cli-test-" + fmt.Sprintf("%d", time.Now().Unix()),
		Description: *description,
		Location:    *location,
		StyleUrl:    *styleUrl,
		Timestamp:   time.Now(),
	}

	fmt.Printf("Enhancing alert...\n")
	fmt.Printf("  Raw Description: %s\n", rawAlert.Description)
	fmt.Printf("  Location: %s\n", rawAlert.Location)
	if rawAlert.StyleUrl != "" {
		fmt.Printf("  StyleUrl: %s\n", rawAlert.StyleUrl)
	}
	fmt.Printf("  Using model: %s\n", *model)
	fmt.Printf("  Timeout: %d seconds\n\n", *timeout)

	enhanced, err := enhancer.EnhanceAlert(ctx, rawAlert)
	if err != nil {
		log.Fatalf("Error enhancing alert: %v", err)
	}

	fmt.Printf("✅ Alert enhanced successfully!\n\n")
	
	// Display structured output
	fmt.Printf("ENHANCED ALERT:\n")
	fmt.Printf("  ID: %s\n", enhanced.ID)
	fmt.Printf("  Original: %s\n", enhanced.OriginalDescription)
	fmt.Printf("  Processed At: %s\n\n", enhanced.ProcessedAt.Format("2006-01-02 15:04:05"))

	fmt.Printf("STRUCTURED DESCRIPTION:\n")
	structured := enhanced.StructuredDescription
	fmt.Printf("  Details: %s\n", structured.Details)
	fmt.Printf("  Location: %s (lat: %.4f, lon: %.4f)\n", structured.Location.Description, structured.Location.Latitude, structured.Location.Longitude)
	fmt.Printf("  Impact: %s\n", structured.Impact)
	fmt.Printf("  Duration: %s\n", structured.Duration)
	
	if structured.TimeReported != "" {
		fmt.Printf("  Time Reported: %s\n", structured.TimeReported)
	}
	if structured.LastUpdate != "" {
		fmt.Printf("  Last Update: %s\n", structured.LastUpdate)
	}

	if structured.AdditionalInfo != nil && len(structured.AdditionalInfo) > 0 {
		fmt.Printf("  Additional Info:\n")
		for key, value := range structured.AdditionalInfo {
			fmt.Printf("    %s: %s\n", key, value)
		}
	}

	fmt.Printf("\nCONDENSED SUMMARY:\n")
	fmt.Printf("  %s\n", enhanced.CondensedSummary)
	fmt.Printf("  Length: %d characters\n", len(enhanced.CondensedSummary))
}

func handleTestConnection() {
	fs := flag.NewFlagSet("test-connection", flag.ExitOnError)
	apiKey := fs.String("api-key", os.Getenv("PF__OPENAI__API_KEY"), "OpenAI API key to test")
	model := fs.String("model", "gpt-3.5-turbo", "OpenAI model to test")
	timeout := fs.Int("timeout", 10, "Timeout in seconds")

	fs.Parse(os.Args[2:])

	if *apiKey == "" {
		log.Fatal("OpenAI API key is required. Set PF__OPENAI__API_KEY environment variable or use --api-key flag")
	}

	enhancer := alerts.NewAlertEnhancer(*apiKey, *model)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	fmt.Printf("Testing OpenAI API connection...\n")
	fmt.Printf("  API Key: %s...%s\n", (*apiKey)[:8], (*apiKey)[len(*apiKey)-4:])
	fmt.Printf("  Model: %s\n", *model)
	fmt.Printf("  Timeout: %d seconds\n\n", *timeout)

	err := enhancer.HealthCheck(ctx)
	if err != nil {
		fmt.Printf("❌ Connection test failed: %v\n", err)
		
		// Provide helpful error guidance
		errStr := fmt.Sprintf("%v", err)
		if strings.Contains(errStr, "401") {
			fmt.Printf("\n💡 This looks like an authentication error. Please check:\n")
			fmt.Printf("   - Your API key is correct\n")
			fmt.Printf("   - Your API key has sufficient permissions\n")
			fmt.Printf("   - Your OpenAI account has credits available\n")
		} else if strings.Contains(errStr, "429") {
			fmt.Printf("\n💡 This looks like a rate limit error. Please:\n")
			fmt.Printf("   - Wait a moment and try again\n")
			fmt.Printf("   - Check your OpenAI usage limits\n")
		}
		
		os.Exit(1)
	}

	fmt.Printf("✅ Connection test successful!\n")
	fmt.Printf("   API is responding normally\n")
	fmt.Printf("   Ready to enhance alerts\n")
}


func handleTestPrompt() {
	fs := flag.NewFlagSet("test-prompt", flag.ExitOnError)
	rawFile := fs.String("raw-file", "", "Path to file containing raw incident descriptions (one per line)")
	apiKey := fs.String("api-key", os.Getenv("PF__OPENAI__API_KEY"), "OpenAI API key")
	model := fs.String("model", "gpt-3.5-turbo", "OpenAI model to use")
	count := fs.Int("count", 5, "Number of incidents to test")

	fs.Parse(os.Args[2:])

	if *rawFile == "" {
		fmt.Println("Example usage:")
		fmt.Println("  test-alert-enhancer test-prompt --raw-file sample_incidents.txt --count 3")
		fmt.Println("")
		fmt.Println("Sample incidents.txt content:")
		fmt.Println("  Rte 4 EB of MM 31 - VEHICLE IN DITCH, EMS ENRT")
		fmt.Println("  Rte 4 WB at Arnold Rim - OVERTURNED VEHICLE OFF ROADWAY, BLOCKING 1 LN")
		fmt.Println("  US 50 E at Mather Field Rd offramp - 3-VEHICLE CRASH, TOW REQ")
		os.Exit(1)
	}

	if *apiKey == "" {
		log.Fatal("OpenAI API key is required. Set PF__OPENAI__API_KEY environment variable or use --api-key flag")
	}

	// Read raw incidents from file
	data, err := os.ReadFile(*rawFile)
	if err != nil {
		log.Fatalf("Error reading file %s: %v", *rawFile, err)
	}

	lines := strings.Split(string(data), "\n")
	var incidents []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			incidents = append(incidents, line)
		}
	}

	if len(incidents) == 0 {
		log.Fatal("No incidents found in file")
	}

	if *count > len(incidents) {
		*count = len(incidents)
	}

	enhancer := alerts.NewAlertEnhancer(*apiKey, *model)
	ctx := context.Background()

	fmt.Printf("Testing prompt with %d incidents from %s\n", *count, *rawFile)
	fmt.Printf("Using model: %s\n\n", *model)

	successCount := 0
	for i := 0; i < *count; i++ {
		fmt.Printf("=== Test %d/%d ===\n", i+1, *count)
		fmt.Printf("Raw: %s\n", incidents[i])

		rawAlert := alerts.RawAlert{
			ID:          fmt.Sprintf("test-%d", i+1),
			Description: incidents[i],
			Location:    "Highway Test",
			Timestamp:   time.Now(),
		}

		enhanced, err := enhancer.EnhanceAlert(ctx, rawAlert)
		if err != nil {
			fmt.Printf("❌ Enhancement failed: %v\n\n", err)
			continue
		}

		fmt.Printf("✅ Enhanced: %s\n", enhanced.StructuredDescription.Details)
		fmt.Printf("   Impact: %s | Duration: %s\n", 
			enhanced.StructuredDescription.Impact,
			enhanced.StructuredDescription.Duration)
		fmt.Printf("   Summary: %s\n\n", enhanced.CondensedSummary)
		successCount++
	}

	fmt.Printf("=== RESULTS ===\n")
	fmt.Printf("Successful enhancements: %d/%d (%.1f%%)\n", 
		successCount, *count, float64(successCount)/float64(*count)*100)
	
	if successCount == *count {
		fmt.Printf("🎉 All tests passed! Prompt is working well.\n")
	} else if float64(successCount)/float64(*count) >= 0.95 {
		fmt.Printf("✅ Good success rate (>95%%), prompt is working well.\n")
	} else {
		fmt.Printf("⚠️  Success rate below 95%%, consider adjusting the prompt.\n")
	}
}

func printUsage() {
	fmt.Printf(`test-alert-enhancer - OpenAI alert enhancement testing tool

USAGE:
    test-alert-enhancer <command> [options]

COMMANDS:
    enhance-alert       Enhance a single alert with AI processing
    test-connection     Test OpenAI API connectivity and authentication
    generate-summary    Generate condensed summary from structured data
    test-prompt         Test prompt with multiple sample incidents
    help               Show this help message

EXAMPLES:
    # Enhance single alert
    test-alert-enhancer enhance-alert --description "Rte 4 EB of MM 31 - VEHICLE IN DITCH, EMS ENRT" --location "Highway 4"
    
    # Test API connection
    test-alert-enhancer test-connection --api-key sk-xxx --model gpt-3.5-turbo
    
    # Generate summary from JSON
    test-alert-enhancer generate-summary --enhanced-json structured.json
    
    # Test prompt with sample data
    test-alert-enhancer test-prompt --raw-file incidents.txt --count 5

ENVIRONMENT VARIABLES:
    PF__OPENAI__API_KEY          OpenAI API key (alternative to --api-key flag)

For more information, visit: https://github.com/dpup/info.ersn.net
`)
}

