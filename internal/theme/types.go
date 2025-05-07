package theme

// ThemeConfigField represents a field in the theme configuration
type ThemeConfigField struct {
	Key      string
	Label    map[string]string
	Type     string
	Value    string
	Editable bool
	Block    string
	Order    int
	Path     string // Path to the theme.json file
	Line     int    // Line number where the field is defined
	Scss     bool
}
