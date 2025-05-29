package storage

import (
	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nickhuang/allama/internal/config"
	"github.com/nickhuang/allama/internal/models"
)

// Storage represents the database connection and operations
type Storage struct {
	db *sql.DB
}

// NewStorage initializes a new database connection and creates necessary tables
func NewStorage(cfg *config.Config) (*Storage, error) {
	db, err := sql.Open("sqlite3", cfg.DatabasePath)
	if err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Storage{db: db}, nil
}

// createTables sets up the database schema
func createTables(db *sql.DB) error {
	// Create providers table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS providers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			api_key TEXT,
			endpoint TEXT,
			is_active BOOLEAN DEFAULT true
		);
	`)
	if err != nil {
		return err
	}

	// Create models table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS models (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			model_id TEXT NOT NULL,
			is_active BOOLEAN DEFAULT true,
			FOREIGN KEY (provider_id) REFERENCES providers(id)
		);
	`)
	if err != nil {
		return err
	}

	return nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// ResetDatabase deletes the existing database file and recreates it with the initial schema
func (s *Storage) ResetDatabase(databasePath string) error {
	// Close the current database connection
	if err := s.Close(); err != nil {
		return err
	}

	// Delete the database file if it exists
	if _, err := os.Stat(databasePath); err == nil {
		if err := os.Remove(databasePath); err != nil {
			return err
		}
	}

	// Reopen a new database connection
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return err
	}

	// Recreate the tables
	if err := createTables(db); err != nil {
		db.Close()
		return err
	}

	// Update the storage instance with the new database connection
	s.db = db
	return nil
}

// AddProvider adds a new provider to the database
func (s *Storage) AddProvider(provider *models.Provider) error {
	result, err := s.db.Exec(
		"INSERT INTO providers (name, api_key, endpoint, is_active) VALUES (?, ?, ?, ?)",
		provider.Name, provider.APIKey, provider.Endpoint, provider.IsActive,
	)
	if err != nil {
		return err
	}

	id, _ := result.LastInsertId()
	provider.ID = int(id)
	return nil
}

// GetProviderByName retrieves a provider by its name
func (s *Storage) GetProviderByName(name string) (*models.Provider, error) {
	provider := &models.Provider{}
	err := s.db.QueryRow(
		"SELECT id, name, api_key, endpoint, is_active FROM providers WHERE name = ?",
		name,
	).Scan(&provider.ID, &provider.Name, &provider.APIKey, &provider.Endpoint, &provider.IsActive)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// GetActiveProviders retrieves all active providers
func (s *Storage) GetActiveProviders() ([]models.Provider, error) {
	rows, err := s.db.Query("SELECT id, name, api_key, endpoint, is_active FROM providers WHERE is_active = true")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []models.Provider
	for rows.Next() {
		var p models.Provider
		if err := rows.Scan(&p.ID, &p.Name, &p.APIKey, &p.Endpoint, &p.IsActive); err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, nil
}

// AddModel adds a new model to the database
func (s *Storage) AddModel(model *models.Model) error {
	result, err := s.db.Exec(
		"INSERT INTO models (provider_id, name, model_id, is_active) VALUES (?, ?, ?, ?)",
		model.ProviderID, model.Name, model.ModelID, model.IsActive,
	)
	if err != nil {
		return err
	}

	id, _ := result.LastInsertId()
	model.ID = int(id)
	return nil
}

// GetModelsByProviderID retrieves all models for a specific provider
func (s *Storage) GetModelsByProviderID(providerID int) ([]models.Model, error) {
	rows, err := s.db.Query(
		"SELECT id, provider_id, name, model_id, is_active FROM models WHERE provider_id = ?",
		providerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modelsList []models.Model
	for rows.Next() {
		var m models.Model
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.ModelID, &m.IsActive); err != nil {
			return nil, err
		}
		modelsList = append(modelsList, m)
	}
	return modelsList, nil
}

// GetActiveModels retrieves all active models
func (s *Storage) GetActiveModels() ([]models.Model, error) {
	rows, err := s.db.Query("SELECT id, provider_id, name, model_id, is_active FROM models WHERE is_active = true")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modelsList []models.Model
	for rows.Next() {
		var m models.Model
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.ModelID, &m.IsActive); err != nil {
			return nil, err
		}
		modelsList = append(modelsList, m)
	}
	return modelsList, nil
}
