package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/symfony"
)

func main() {
	// Get the project root from command line or use current directory
	projectRoot := "."
	if len(os.Args) > 1 {
		projectRoot = os.Args[1]
	}

	// Get absolute path
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		log.Fatalf("Failed to get absolute path: %v", err)
	}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "container-test-*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize the service index
	serviceIndex, err := symfony.NewServiceIndex(absRoot, tempDir)
	if err != nil {
		log.Fatalf("Failed to create service index: %v", err)
	}
	defer serviceIndex.Close()

	// Print container watcher status
	fmt.Println("Checking for Symfony container XML file...")
	
	// Get all services
	services := serviceIndex.GetAllServices()
	fmt.Printf("Found %d services in total\n", len(services))

	// Sample a few services if available
	if len(services) > 0 {
		maxSample := 5
		if len(services) < maxSample {
			maxSample = len(services)
		}
		
		fmt.Println("\nSample services:")
		for i := 0; i < maxSample; i++ {
			service, found := serviceIndex.GetServiceByID(services[i])
			if found {
				fmt.Printf("  - %s (Class: %s)\n", service.ID, service.Class)
			}
		}
	}
}
