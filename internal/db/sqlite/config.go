package sqlite

// Config controls the app-owned SQLite database connection.
type Config struct {
	Path string `yaml:"path" env:"HECTOR_DB_PATH" default:".hector/hector.db"`
}
