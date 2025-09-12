package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dpup/info.ersn.net/server/internal/lib/incident"
)

type MockHasherIncident struct {
	Description string
	Latitude    float64
	Longitude   float64
	Category    string
	URL         string
}

func main() {
	hasher := incident.NewIncidentContentHasher()
	ctx := context.Background()

	testIncident := MockHasherIncident{
		Description: "I-80 WESTBOUND CHAIN CONTROLS REQUIRED FROM DRUM TO NYACK",
		Latitude:    39.1234,
		Longitude:   -120.5678,
		Category:    "chain_control",
		URL:         "chain_controls.kml",
	}

	// Test deterministic hashing
	hash1, err := hasher.HashIncident(ctx, testIncident)
	if err != nil {
		log.Fatal("First hash generation error:", err)
	}

	hash2, err := hasher.HashIncident(ctx, testIncident)
	if err != nil {
		log.Fatal("Second hash generation error:", err)
	}

	// Same incident should produce identical hashes
	if hash1.ContentHash != hash2.ContentHash {
		log.Fatal("Same incident should produce same content hash")
	}

	fmt.Printf("✅ Hash generation working!\n")
	fmt.Printf("Content Hash: %s\n", hash1.ContentHash)
	fmt.Printf("Normalized Text: %s\n", hash1.NormalizedText)
	fmt.Printf("Location Key: %s\n", hash1.LocationKey)
	fmt.Printf("Category: %s\n", hash1.IncidentCategory)

	// Test normalization
	normalized := hasher.NormalizeIncidentText("  I-80 WESTBOUND CHAIN CONTROLS!!! ")
	fmt.Printf("Normalized: '%s'\n", normalized)

	// Test validation
	err = hasher.ValidateContentHash(hash1)
	if err != nil {
		log.Fatal("Validation should pass:", err)
	}

	fmt.Println("✅ All IncidentContentHasher tests passed!")
}