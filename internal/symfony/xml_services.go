package symfony

import (
	"encoding/xml"
	"io"
	"log"
	"os"
	"strings"
)

// Service represents a Symfony service definition
type Service struct {
	ID    string            // Service ID
	Class string            // Service class
	Tags  map[string]string // Service tags
	Path  string            // Source file path
	Line  int               // Line number in source file
}

// ServiceAlias represents a Symfony service alias
type ServiceAlias struct {
	ID     string // Alias ID
	Target string // Target service ID
	Path   string // Source file path
	Line   int    // Line number in source file
}

// ParseXMLServices parses Symfony XML service definitions and returns a list of services and aliases.
func ParseXMLServices(path string) ([]Service, []ServiceAlias, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	// Skip debug logging in production code

	// Find line numbers for services and aliases
	contentLines := strings.Split(string(data), "\n")
	serviceLineMap := make(map[string]int) // Maps service ID to line number
	aliasLineMap := make(map[string]int)   // Maps alias ID to line number

	// Scan through the file line by line to find service and alias definitions
	for i, line := range contentLines {
		// Check for service definitions
		if strings.Contains(line, "<service") && strings.Contains(line, "id=") {
			// Extract the service ID
			idStart := strings.Index(line, "id=") + 4 // +4 to skip 'id="'
			idEnd := strings.Index(line[idStart:], "\"") + idStart
			if idStart > 3 && idEnd > idStart {
				serviceID := line[idStart:idEnd]
				serviceLineMap[serviceID] = i + 1 // +1 because line numbers are 1-based
			}
		}

		// Check for alias definitions
		if strings.Contains(line, "<alias") && strings.Contains(line, "id=") {
			// Extract the alias ID
			idStart := strings.Index(line, "id=") + 4 // +4 to skip 'id="'
			idEnd := strings.Index(line[idStart:], "\"") + idStart
			if idStart > 3 && idEnd > idStart {
				aliasID := line[idStart:idEnd]
				aliasLineMap[aliasID] = i + 1 // +1 because line numbers are 1-based
			}
		}
	}

	// XML structures to decode the file
	type XMLTag struct {
		Name string `xml:"name,attr"`
	}

	type XMLService struct {
		ID    string   `xml:"id,attr"`
		Class string   `xml:"class,attr"`
		Tags  []XMLTag `xml:"tag"`
	}

	type XMLAlias struct {
		ID      string `xml:"id,attr"`
		Service string `xml:"service,attr"`
	}

	type XMLServices struct {
		Services  []XMLService `xml:"service"`
		Defaults  struct{}     `xml:"defaults"`
		Prototype struct{}     `xml:"prototype"`
	}

	type XMLContainer struct {
		Services []XMLService `xml:"service"`
		Aliases  []XMLAlias   `xml:"alias"`
		// Handle nested services tag
		ServicesTag XMLServices `xml:"services"`
	}

	// Parse the XML
	var container XMLContainer
	decoder := xml.NewDecoder(strings.NewReader(string(data)))

	// Handle XML namespaces by ignoring them
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity

	// Skip namespace declarations
	decoder.DefaultSpace = "http://symfony.com/schema/dic/services"

	err = decoder.Decode(&container)
	if err != nil && err != io.EOF {
		log.Printf("Error decoding XML: %v", err)
		return nil, nil, err
	}

	// Convert to our service structures
	var services []Service

	// Process direct services
	for _, xmlService := range container.Services {
		service := Service{
			ID:    xmlService.ID,
			Class: xmlService.Class,
			Tags:  make(map[string]string),
			Path:  path,
			Line:  serviceLineMap[xmlService.ID], // Set the line number
		}

		// If service has no class, use ID as class (Symfony default behavior)
		if service.Class == "" {
			service.Class = service.ID
		}

		// Process tags
		for _, tag := range xmlService.Tags {
			if tag.Name != "" {
				service.Tags[tag.Name] = ""
			}
		}

		if service.ID != "" {
			services = append(services, service)
		}
	}

	// Process services inside the services tag
	for _, xmlService := range container.ServicesTag.Services {
		service := Service{
			ID:    xmlService.ID,
			Class: xmlService.Class,
			Tags:  make(map[string]string),
			Path:  path,
			Line:  serviceLineMap[xmlService.ID], // Set the line number
		}

		// If service has no class, use ID as class (Symfony default behavior)
		if service.Class == "" {
			service.Class = service.ID
		}

		// Process tags
		for _, tag := range xmlService.Tags {
			if tag.Name != "" {
				service.Tags[tag.Name] = ""
			}
		}

		if service.ID != "" {
			services = append(services, service)
		}
	}

	// Convert to our alias structures
	var aliases []ServiceAlias
	for _, xmlAlias := range container.Aliases {
		alias := ServiceAlias{
			ID:     xmlAlias.ID,
			Target: xmlAlias.Service,
			Path:   path,
			Line:   aliasLineMap[xmlAlias.ID], // Set the line number
		}

		if alias.ID != "" && alias.Target != "" {
			aliases = append(aliases, alias)
		}
	}

	return services, aliases, nil
}

// GetServiceIDs extracts just the service IDs from a list of services
func GetServiceIDs(services []Service, aliases []ServiceAlias) []string {
	result := make([]string, 0, len(services)+len(aliases))

	// Add service IDs
	for _, service := range services {
		result = append(result, service.ID)
	}

	// Add alias IDs
	for _, alias := range aliases {
		result = append(result, alias.ID)
	}

	return result
}
