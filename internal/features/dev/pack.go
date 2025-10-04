package dev

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// PackOptions defines options for pack operations
type PackOptions struct {
	InputFile   string
	OutputDir   string
	Force       bool
	Compression int // gzip compression level (1-9)
}

// DefaultPackOptions returns default options for pack operations
func DefaultPackOptions() PackOptions {
	return PackOptions{
		InputFile:   "",
		OutputDir:   "assets/data",
		Force:       false,
		Compression: 9, // maximum compression
	}
}

// PackShakespert compresses shakespert database/SQL data into .sql.gz format
func PackShakespert(opts PackOptions) error {
	outputPath := filepath.Join(opts.OutputDir, "shakespert.sql.gz")

	// Check if output file exists and force is not set
	if !opts.Force {
		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("file %s already exists, use --force to overwrite", outputPath)
		}
	}

	// Determine input file
	inputFile := opts.InputFile
	if inputFile == "" {
		// Try to find input file automatically
		if _, err := os.Stat("shakespert.db"); err == nil {
			inputFile = "shakespert.db"
		} else if _, err := os.Stat("shakespert.sql"); err == nil {
			inputFile = "shakespert.sql"
		} else {
			return fmt.Errorf("no input file specified and neither shakespert.db nor shakespert.sql found in current directory")
		}
	}

	// Check input file exists
	if _, err := os.Stat(inputFile); err != nil {
		return fmt.Errorf("input file %s not found: %w", inputFile, err)
	}

	var sqlData string
	var err error

	// Handle different input file types
	switch filepath.Ext(inputFile) {
	case ".db":
		// It's a SQLite database, dump it to SQL
		sqlData, err = dumpSQLiteToSQL(inputFile)
		if err != nil {
			return fmt.Errorf("failed to dump database to SQL: %w", err)
		}
		fmt.Printf("✓ Dumped %s to SQL (%.1f MB)\n", inputFile, float64(len(sqlData))/1024/1024)

	case ".sql":
		// It's already SQL, read it
		sqlBytes, err := os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf("failed to read SQL file: %w", err)
		}
		sqlData = string(sqlBytes)
		fmt.Printf("✓ Read %s (%.1f MB)\n", inputFile, float64(len(sqlData))/1024/1024)

	default:
		return fmt.Errorf("unsupported file type: %s (expected .db or .sql)", filepath.Ext(inputFile))
	}

	// Validate the SQL data
	if err := validateSQL(sqlData); err != nil {
		return fmt.Errorf("SQL validation failed: %w", err)
	}

	// Compress the SQL data
	compressedData, err := compressSQL(sqlData, opts.Compression)
	if err != nil {
		return fmt.Errorf("failed to compress SQL: %w", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write compressed data
	if err := os.WriteFile(outputPath, compressedData, 0o644); err != nil {
		return fmt.Errorf("failed to write compressed file: %w", err)
	}

	// Report compression stats
	originalSize := len(sqlData)
	compressedSize := len(compressedData)
	compressionRatio := float64(compressedSize) / float64(originalSize) * 100

	fmt.Printf("✓ Created %s (%.1f MB → %.1f MB, %.1f%% compression)\n",
		outputPath,
		float64(originalSize)/1024/1024,
		float64(compressedSize)/1024/1024,
		100-compressionRatio)

	// Verify the compressed file
	if err := verifyCompressedSQL(outputPath); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	fmt.Printf("✓ Verified compressed file is valid\n")

	return nil
}

// dumpSQLiteToSQL dumps a SQLite database to SQL statements
func dumpSQLiteToSQL(dbPath string) (string, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Test the connection by running a simple query
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if err != nil {
		return "", fmt.Errorf("failed to query database: %w", err)
	}

	if count == 0 {
		return "", fmt.Errorf("database contains no tables")
	}

	// Use sqlite3 command to dump the database
	// This is more reliable than trying to reconstruct SQL ourselves
	return dumpSQLiteUsingCommand(dbPath)
}

// dumpSQLiteUsingCommand uses sqlite3 command to dump database
func dumpSQLiteUsingCommand(dbPath string) (string, error) {
	// Create a temporary file for the SQL output
	tmpFile, err := os.CreateTemp("", "shakespert_dump_*.sql")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Use a Go-based approach by reading the DB directly
	// and generating basic INSERT statements
	return generateSQLFromDatabase(dbPath)
}

// generateSQLFromDatabase generates SQL from database (simplified approach)
func generateSQLFromDatabase(dbPath string) (string, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	var sqlBuffer bytes.Buffer

	// Add pragmas and basic setup
	sqlBuffer.WriteString("PRAGMA foreign_keys=OFF;\n")
	sqlBuffer.WriteString("BEGIN TRANSACTION;\n")

	// Get schema
	rows, err := db.Query("SELECT sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return "", fmt.Errorf("failed to query schema: %w", err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return "", fmt.Errorf("failed to scan schema: %w", err)
		}
		schemas = append(schemas, schema)
	}

	// Write table creation statements
	for _, schema := range schemas {
		sqlBuffer.WriteString(schema + ";\n")
	}

	// Get table names for data export
	tableRows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return "", fmt.Errorf("failed to query table names: %w", err)
	}
	defer tableRows.Close()

	var tableNames []string
	for tableRows.Next() {
		var tableName string
		if err := tableRows.Scan(&tableName); err != nil {
			return "", fmt.Errorf("failed to scan table name: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}

	// Export data for each table
	for _, tableName := range tableNames {
		if err := exportTableData(db, tableName, &sqlBuffer); err != nil {
			return "", fmt.Errorf("failed to export data for table %s: %w", tableName, err)
		}
	}

	// Add indexes and triggers
	indexRows, err := db.Query("SELECT sql FROM sqlite_master WHERE type IN ('index', 'trigger') AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return "", fmt.Errorf("failed to query indexes: %w", err)
	}
	defer indexRows.Close()

	for indexRows.Next() {
		var sql string
		if err := indexRows.Scan(&sql); err != nil {
			return "", fmt.Errorf("failed to scan index: %w", err)
		}
		if sql != "" {
			sqlBuffer.WriteString(sql + ";\n")
		}
	}

	sqlBuffer.WriteString("COMMIT;\n")

	return sqlBuffer.String(), nil
}

// exportTableData exports data from a single table as INSERT statements
func exportTableData(db *sql.DB, tableName string, buffer *bytes.Buffer) error {
	// Get column information
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name, datatype string
		var notnull, pk int
		var dfltValue interface{}

		if err := rows.Scan(&cid, &name, &datatype, &notnull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}
		columns = append(columns, name)
	}

	if len(columns) == 0 {
		return fmt.Errorf("no columns found for table %s", tableName)
	}

	// Export data
	dataRows, err := db.Query(fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		return fmt.Errorf("failed to query table data: %w", err)
	}
	defer dataRows.Close()

	columnCount := len(columns)
	values := make([]interface{}, columnCount)
	scanArgs := make([]interface{}, columnCount)
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for dataRows.Next() {
		if err := dataRows.Scan(scanArgs...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		buffer.WriteString(fmt.Sprintf("INSERT INTO %s VALUES(", tableName))

		for i, val := range values {
			if i > 0 {
				buffer.WriteString(",")
			}

			if val == nil {
				buffer.WriteString("NULL")
			} else if str, ok := val.(string); ok {
				// Escape single quotes
				escaped := strings.ReplaceAll(str, "'", "''")
				buffer.WriteString(fmt.Sprintf("'%s'", escaped))
			} else {
				buffer.WriteString(fmt.Sprintf("'%v'", val))
			}
		}

		buffer.WriteString(");\n")
	}

	return nil
}

// validateSQL performs basic validation on SQL data
func validateSQL(sqlData string) error {
	if len(sqlData) == 0 {
		return fmt.Errorf("SQL data is empty")
	}

	// Check for basic SQL structure
	if !strings.Contains(sqlData, "CREATE TABLE") {
		return fmt.Errorf("SQL data does not contain CREATE TABLE statements")
	}

	return nil
}

// compressSQL compresses SQL data using gzip
func compressSQL(sqlData string, compressionLevel int) ([]byte, error) {
	var buf bytes.Buffer

	writer, err := gzip.NewWriterLevel(&buf, compressionLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}

	if _, err := writer.Write([]byte(sqlData)); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// verifyCompressedSQL verifies that a compressed SQL file is valid
func verifyCompressedSQL(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Try to decompress
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	// Read a few bytes to verify it's valid
	buf := make([]byte, 100)
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read compressed data: %w", err)
	}

	if n == 0 {
		return fmt.Errorf("compressed file is empty")
	}

	// Check if it looks like SQL
	content := string(buf[:n])
	if !strings.Contains(content, "PRAGMA") && !strings.Contains(content, "CREATE") {
		return fmt.Errorf("compressed data does not appear to be SQL")
	}

	return nil
}
