package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofrs/uuid"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Database configuration struct
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// Connect to database
func connectDB(config DBConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		config.Host, config.User, config.Password, config.DBName, config.Port)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// Structure to hold permission data from role_permissions
type RolePermission struct {
	Permission string `gorm:"column:permission"`
}

// Structure to hold full permission table data
type Permission struct {
	ID          uuid.UUID  `gorm:"type:uuid;column:id;primaryKey"`
	Name        string     `gorm:"column:name"`
	Code        string     `gorm:"column:code"`
	Description string     `gorm:"column:description"`
	IsDeleted   bool       `gorm:"column:is_deleted"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	CreatedBy   string     `gorm:"column:created_by"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
	UpdatedBy   string     `gorm:"column:updated_by"`
}

// loadConfig loads configuration from environment variables or .env file
func loadConfig() (DBConfig, error) {
	// Define flags
	envFile := flag.String("env", ".env", "Path to the .env file")
	flag.Parse()

	// Load environment variables from .env file
	err := godotenv.Load(*envFile)
	if err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
		log.Println("Using environment variables instead")
	}

	// Get database connection details from environment variables
	dbConfig := DBConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", ""),
		DBName:   getEnv("DB_NAME", ""),
	}

	// Validate database name
	if dbConfig.DBName == "" {
		return dbConfig, fmt.Errorf("database name not provided. Set DB_NAME in .env file or environment variable")
	}

	return dbConfig, nil
}

func main() {
	// Load configuration
	dbConfig, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Connect to database
	log.Println("Connecting to database...")
	db, err := connectDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Process missing permissions
	missingPermissions, err := findMissingPermissions(db)
	if err != nil {
		log.Fatalf("Error finding missing permissions: %v", err)
	}

	log.Printf("Found %d missing permissions", len(missingPermissions))

	// Insert missing permissions if any
	if len(missingPermissions) > 0 {
		err := insertMissingPermissions(db, missingPermissions)
		if err != nil {
			log.Fatalf("Error inserting missing permissions: %v", err)
		}
	} else {
		log.Println("No missing permissions found.")
	}

	log.Println("Operation completed successfully.")
}

// findMissingPermissions identifies permissions that exist in role_permissions but not in permissions table
func findMissingPermissions(db *gorm.DB) ([]string, error) {
	// Start transaction
	tx := db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer tx.Rollback() // Will be committed if no errors

	// Get permissions from role_permissions table
	var rolePermissions []RolePermission
	err := tx.Table("role_permissions").Select("DISTINCT permission_code as permission").Scan(&rolePermissions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query role_permissions table: %w", err)
	}

	log.Printf("Found %d unique permissions in role_permissions table", len(rolePermissions))

	// Get existing permissions from permissions table
	var existingPermissionCodes []string
	err = tx.Table("permissions").Select("code").Pluck("code", &existingPermissionCodes).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query permissions table: %w", err)
	}

	log.Printf("Found %d existing permissions in permissions table", len(existingPermissionCodes))

	// Create a map of existing permissions for quick lookup
	existingPermMap := make(map[string]bool)
	for _, code := range existingPermissionCodes {
		existingPermMap[code] = true
	}

	// Find missing permissions
	var missingPermissions []string
	for _, rp := range rolePermissions {
		if !existingPermMap[rp.Permission] {
			missingPermissions = append(missingPermissions, rp.Permission)
		}
	}

	tx.Commit()
	return missingPermissions, nil
}

// insertMissingPermissions inserts missing permissions into the permissions table
func insertMissingPermissions(db *gorm.DB, missingPermissions []string) error {
	// Create timestamp for logging
	timestamp := time.Now().Format("20060102_150405")
	logFile, err := os.Create(fmt.Sprintf("permissions_insert_%s.log", timestamp))
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()
	logger := log.New(logFile, "", log.LstdFlags)

	// Start transaction
	tx := db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer tx.Rollback() // Will be committed if no errors

	logger.Printf("Starting insertion of %d missing permissions", len(missingPermissions))

	// Insert each missing permission
	currentTime := time.Now()

	for _, permission := range missingPermissions {
		logger.Printf("Inserting permission: %s", permission)

		// Generate UUID7-like value
		// Since gofrs/uuid doesn't have UUID7, we'll simulate it by using V4 and setting timestamp bytes
		uuidV4, err := uuid.NewV4()
		if err != nil {
			logger.Printf("Error generating UUID: %v", err)
			return fmt.Errorf("failed to generate UUID: %w", err)
		}

		// Insert the permission with all required fields
		permissionName := permission // Default to using the code as the name

		// Insert the permission with all required fields
		query := `
			INSERT INTO permissions 
			(id, name, code, description, is_deleted, created_at, created_by, updated_at, updated_by) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`

		err = tx.Exec(
			query,
			uuidV4,                // id
			permissionName,        // name
			permission,            // code
			"Added automatically", // description
			false,                 // is_deleted
			currentTime,           // created_at
			"system",              // created_by
			currentTime,           // updated_at
			"system",              // updated_by
		).Error

		if err != nil {
			logger.Printf("Error inserting permission %s: %v", permission, err)
			return fmt.Errorf("failed to insert permission %s: %w", permission, err)
		}
	}

	// Commit transaction if all inserts are successful
	err = tx.Commit().Error
	if err != nil {
		logger.Printf("Error committing transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Printf("Successfully inserted %d permissions", len(missingPermissions))
	log.Printf("Successfully inserted %d permissions. Check permissions_insert_%s.log for details.",
		len(missingPermissions), timestamp)

	return nil
}

// Helper function to get environment variable with default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
