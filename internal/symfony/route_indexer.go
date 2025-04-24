package symfony

// Route represents a Symfony route from YAML, PHP, or other sources
type Route struct {
	Name       string
	Path       string
	Controller string
	FilePath   string
	Line       int
}
