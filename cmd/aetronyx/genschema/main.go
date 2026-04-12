package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/karthikcodes/aetronyx/internal/spec"
)

func main() {
	// Generate the JSON schema
	schemaData, err := spec.GenerateSchema()
	if err != nil {
		log.Fatalf("Failed to generate schema: %v", err)
	}

	// Create output directory
	outputDir := "docs/reference"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create directory %s: %v", outputDir, err)
	}

	// Write schema to file
	outputPath := filepath.Join(outputDir, "spec-schema.json")
	if err := os.WriteFile(outputPath, schemaData, 0644); err != nil {
		log.Fatalf("Failed to write schema to %s: %v", outputPath, err)
	}

	fmt.Printf("Generated schema at %s\n", outputPath)
}
