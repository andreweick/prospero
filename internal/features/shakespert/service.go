package shakespert

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"prospero/assets"

	_ "modernc.org/sqlite"
)

type Service struct {
	db           *sql.DB
	queries      *Queries
	tempFilePath string
}

// WorkSummary represents a simplified view of a work for listings
type WorkSummary struct {
	WorkID          string
	Title           string
	LongTitle       string
	Date            int64
	GenreType       string
	GenreName       string
	TotalWords      int64
	TotalParagraphs int64
}

// WorkDetail represents detailed information about a work
type WorkDetail struct {
	WorkID          string
	Title           string
	LongTitle       string
	ShortTitle      string
	Date            int64
	GenreType       string
	GenreName       string
	Notes           string
	Source          string
	TotalWords      int64
	TotalParagraphs int64
}

// NewService creates a new shakespert service with a temporary SQLite database file
func NewService(ctx context.Context) (*Service, error) {
	// Get embedded database data
	dbData := assets.GetEmbeddedShakespertDB()

	// Create a temporary file for the database
	tempFile, err := os.CreateTemp("", "shakespert-*.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempFilePath := tempFile.Name()

	// Write the database data to the temporary file
	if _, err := tempFile.Write(dbData); err != nil {
		tempFile.Close()
		os.Remove(tempFilePath)
		return nil, fmt.Errorf("failed to write database to temporary file: %w", err)
	}
	tempFile.Close()

	// Open the SQLite database in read-only mode
	dbPath := filepath.ToSlash(tempFilePath) + "?mode=ro"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		os.Remove(tempFilePath)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		os.Remove(tempFilePath)
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create queries instance
	queries := New(db)

	return &Service{
		db:           db,
		queries:      queries,
		tempFilePath: tempFilePath,
	}, nil
}

// Close closes the database connection and cleans up the temporary file
func (s *Service) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	// Clean up the temporary file
	if s.tempFilePath != "" {
		if err := os.Remove(s.tempFilePath); err != nil {
			// Log the error but don't fail the close operation
			fmt.Printf("Warning: failed to remove temporary database file %s: %v\n", s.tempFilePath, err)
		}
	}

	return nil
}

// ListWorks returns a list of all Shakespeare works
func (s *Service) ListWorks(ctx context.Context) ([]WorkSummary, error) {
	rows, err := s.queries.ListWorks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list works: %w", err)
	}

	works := make([]WorkSummary, len(rows))
	for i, row := range rows {
		works[i] = WorkSummary{
			WorkID:          row.Workid,
			Title:           nullStringToString(row.Title),
			LongTitle:       nullStringToString(row.Longtitle),
			Date:            nullInt64ToInt64(row.Date),
			GenreType:       nullStringToString(row.Genretype),
			GenreName:       nullStringToString(row.Genrename),
			TotalWords:      nullInt64ToInt64(row.Totalwords),
			TotalParagraphs: nullInt64ToInt64(row.Totalparagraphs),
		}
	}

	return works, nil
}

// GetWork returns detailed information about a specific work
func (s *Service) GetWork(ctx context.Context, workID string) (*WorkDetail, error) {
	row, err := s.queries.GetWork(ctx, workID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("work not found: %s", workID)
		}
		return nil, fmt.Errorf("failed to get work: %w", err)
	}

	return &WorkDetail{
		WorkID:          row.Workid,
		Title:           nullStringToString(row.Title),
		LongTitle:       nullStringToString(row.Longtitle),
		ShortTitle:      nullStringToString(row.Shorttitle),
		Date:            nullInt64ToInt64(row.Date),
		GenreType:       nullStringToString(row.Genretype),
		GenreName:       nullStringToString(row.Genrename),
		Notes:           string(row.Notes),
		Source:          nullStringToString(row.Source),
		TotalWords:      nullInt64ToInt64(row.Totalwords),
		TotalParagraphs: nullInt64ToInt64(row.Totalparagraphs),
	}, nil
}

// ListGenres returns all available genres
func (s *Service) ListGenres(ctx context.Context) ([]Genre, error) {
	return s.queries.ListGenres(ctx)
}

// GetWorksByGenre returns works filtered by genre
func (s *Service) GetWorksByGenre(ctx context.Context, genreType string) ([]WorkSummary, error) {
	rows, err := s.queries.GetWorksByGenre(ctx, sql.NullString{String: genreType, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to get works by genre: %w", err)
	}

	works := make([]WorkSummary, len(rows))
	for i, row := range rows {
		works[i] = WorkSummary{
			WorkID:          row.Workid,
			Title:           nullStringToString(row.Title),
			LongTitle:       nullStringToString(row.Longtitle),
			Date:            nullInt64ToInt64(row.Date),
			GenreType:       nullStringToString(row.Genretype),
			GenreName:       nullStringToString(row.Genrename),
			TotalWords:      nullInt64ToInt64(row.Totalwords),
			TotalParagraphs: nullInt64ToInt64(row.Totalparagraphs),
		}
	}

	return works, nil
}

// Helper functions to handle sql.Null types
func nullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullInt64ToInt64(ni sql.NullInt64) int64 {
	if ni.Valid {
		return ni.Int64
	}
	return 0
}
