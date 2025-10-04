package dev

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
	_ "modernc.org/sqlite"

	"prospero/assets"
	"prospero/internal/features/topten"
)

// ExtractOptions defines options for extraction operations
type ExtractOptions struct {
	OutputDir string
	Force     bool
}

// DefaultExtractOptions returns default options for extraction
func DefaultExtractOptions() ExtractOptions {
	return ExtractOptions{
		OutputDir: "tmp",
		Force:     false,
	}
}

// ExtractAll extracts all embedded data files for development
func ExtractAll(ctx context.Context, opts ExtractOptions) error {
	if err := ExtractTopTen(ctx, opts); err != nil {
		return fmt.Errorf("failed to extract topten: %w", err)
	}

	if err := ExtractHostKey(ctx, opts); err != nil {
		return fmt.Errorf("failed to extract hostkey: %w", err)
	}

	if err := ExtractShakespert(ctx, opts); err != nil {
		return fmt.Errorf("failed to extract shakespert: %w", err)
	}

	return nil
}

// ExtractSecrets extracts all age-encrypted files
func ExtractSecrets(ctx context.Context, opts ExtractOptions) error {
	if err := ExtractTopTen(ctx, opts); err != nil {
		return fmt.Errorf("failed to extract topten: %w", err)
	}

	if err := ExtractHostKey(ctx, opts); err != nil {
		return fmt.Errorf("failed to extract hostkey: %w", err)
	}

	return nil
}

// ExtractTopTen decrypts and extracts topten.json.age to topten.json
func ExtractTopTen(ctx context.Context, opts ExtractOptions) error {
	// Ensure output directory exists
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(opts.OutputDir, "topten.json")

	// Check if file exists and force is not set
	if !opts.Force {
		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("file %s already exists, use --force to overwrite", outputPath)
		}
	}

	// Decrypt the topten data
	decryptedData, err := decryptAgeData(ctx, assets.GetEmbeddedTopTenData())
	if err != nil {
		return fmt.Errorf("failed to decrypt topten data: %w", err)
	}

	// Parse as JSON to validate and pretty-print
	var collection topten.TopTenCollection
	if err := json.Unmarshal(decryptedData, &collection); err != nil {
		return fmt.Errorf("failed to parse topten data: %w", err)
	}

	// Pretty-print the JSON
	prettyJSON, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, prettyJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write topten.json: %w", err)
	}

	fmt.Printf("✓ Extracted topten.json (%d lists, %.1f KB)\n",
		len(collection.Lists), float64(len(prettyJSON))/1024)

	return nil
}

// ExtractHostKey decrypts and extracts hostkey.age to hostkey
func ExtractHostKey(ctx context.Context, opts ExtractOptions) error {
	outputPath := filepath.Join(opts.OutputDir, "hostkey")

	// Check if file exists and force is not set
	if !opts.Force {
		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("file %s already exists, use --force to overwrite", outputPath)
		}
	}

	// Decrypt the host key
	decryptedKey, err := decryptAgeData(ctx, assets.GetEmbeddedSSHKey())
	if err != nil {
		return fmt.Errorf("failed to decrypt host key: %w", err)
	}

	// Validate it's an SSH key
	keyStr := string(decryptedKey)
	if !strings.HasPrefix(keyStr, "-----BEGIN OPENSSH PRIVATE KEY-----") {
		return fmt.Errorf("decrypted data does not appear to be a valid OpenSSH private key")
	}

	// Write to file
	if err := os.WriteFile(outputPath, decryptedKey, 0o600); err != nil {
		return fmt.Errorf("failed to write hostkey: %w", err)
	}

	fmt.Printf("✓ Extracted hostkey (%.1f KB)\n", float64(len(decryptedKey))/1024)

	return nil
}

// ExtractShakespert decompresses and extracts shakespert data to both .sql and .db files
func ExtractShakespert(ctx context.Context, opts ExtractOptions) error {
	sqlPath := filepath.Join(opts.OutputDir, "shakespert.sql")
	dbPath := filepath.Join(opts.OutputDir, "shakespert.db")

	// Check if files exist and force is not set
	if !opts.Force {
		if _, err := os.Stat(sqlPath); err == nil {
			return fmt.Errorf("file %s already exists, use --force to overwrite", sqlPath)
		}
		if _, err := os.Stat(dbPath); err == nil {
			return fmt.Errorf("file %s already exists, use --force to overwrite", dbPath)
		}
	}

	// Decompress the SQL data
	sqlData, err := decompressGzipData(assets.GetEmbeddedShakespertDB())
	if err != nil {
		return fmt.Errorf("failed to decompress shakespert data: %w", err)
	}

	// Write SQL file
	if err := os.WriteFile(sqlPath, []byte(sqlData), 0o644); err != nil {
		return fmt.Errorf("failed to write shakespert.sql: %w", err)
	}

	// Create SQLite database from SQL
	if err := createSQLiteFromSQL(sqlData, dbPath); err != nil {
		return fmt.Errorf("failed to create shakespert.db: %w", err)
	}

	fmt.Printf("✓ Extracted shakespert.sql (%.1f MB)\n", float64(len(sqlData))/1024/1024)

	// Get database file size for reporting
	if stat, err := os.Stat(dbPath); err == nil {
		fmt.Printf("✓ Created shakespert.db (%.1f MB)\n", float64(stat.Size())/1024/1024)
	}

	return nil
}

// decryptAgeData decrypts age-encrypted data using AGE_ENCRYPTION_PASSWORD
func decryptAgeData(ctx context.Context, encryptedData []byte) ([]byte, error) {
	// Get the password from environment variable
	password := os.Getenv("AGE_ENCRYPTION_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("AGE_ENCRYPTION_PASSWORD environment variable is not set")
	}

	// Create age identity for decryption
	identity, err := age.NewScryptIdentity(password)
	if err != nil {
		return nil, fmt.Errorf("failed to create age identity: %w", err)
	}

	// Check if the data is armored (ASCII format)
	var ageReader io.Reader
	if bytes.HasPrefix(encryptedData, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		// It's armored, decode it first
		armorReader := armor.NewReader(bytes.NewReader(encryptedData))
		ageReader = armorReader
	} else {
		// It's binary format
		ageReader = bytes.NewReader(encryptedData)
	}

	// Decrypt the age data
	reader, err := age.Decrypt(ageReader, identity)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt age data: %w", err)
	}

	// Read the decrypted data
	decryptedData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted data: %w", err)
	}

	return decryptedData, nil
}

// decompressGzipData decompresses gzip-compressed data
func decompressGzipData(compressedData []byte) (string, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read compressed data: %w", err)
	}

	return string(data), nil
}

// createSQLiteFromSQL creates a SQLite database from SQL statements
func createSQLiteFromSQL(sqlData, dbPath string) error {
	// Remove existing database file
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing database: %w", err)
	}

	// Create new database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	// Execute SQL statements
	if _, err := db.Exec(sqlData); err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	return nil
}
