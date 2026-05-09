package sqlite

// Config controls the app-owned SQLite database connection.
type Config struct {
	// Path overrides the SQLite database location. If empty, XDG data home is used.
	Path string `yaml:"path" env:"HECTOR_DB_PATH"`
}
