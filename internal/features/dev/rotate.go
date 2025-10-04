package dev

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"filippo.io/age"
	"filippo.io/age/armor"

	"prospero/assets"
)

// RotateKeyOptions defines options for key rotation
type RotateKeyOptions struct {
	DryRun bool
	Backup bool
}

// DefaultRotateKeyOptions returns default options for key rotation
func DefaultRotateKeyOptions() RotateKeyOptions {
	return RotateKeyOptions{
		DryRun: false,
		Backup: false,
	}
}

// ageFile represents an age-encrypted file for rotation
type ageFile struct {
	Name          string
	EmbeddedData  []byte
	OutputPath    string
	DecryptedData []byte
	EncryptedData []byte
}

// RotateKeys safely rotates encryption keys for all age-encrypted files
func RotateKeys(ctx context.Context, opts RotateKeyOptions) error {
	// Get both passwords
	currentPassword := os.Getenv("AGE_ENCRYPTION_PASSWORD")
	if currentPassword == "" {
		return fmt.Errorf("AGE_ENCRYPTION_PASSWORD environment variable is not set")
	}

	previousPassword := os.Getenv("PREVIOUS_AGE_ENCRYPTION_PASSWORD")
	if previousPassword == "" {
		return fmt.Errorf("PREVIOUS_AGE_ENCRYPTION_PASSWORD environment variable is not set\nPlease set both AGE_ENCRYPTION_PASSWORD (new) and PREVIOUS_AGE_ENCRYPTION_PASSWORD (old)")
	}

	if currentPassword == previousPassword {
		return fmt.Errorf("AGE_ENCRYPTION_PASSWORD and PREVIOUS_AGE_ENCRYPTION_PASSWORD cannot be the same")
	}

	// Discover all age files to rotate
	files := []ageFile{
		{
			Name:         "topten.json.age",
			EmbeddedData: assets.GetEmbeddedTopTenData(),
			OutputPath:   "assets/data/topten.json.age",
		},
		{
			Name:         "hostkey.age",
			EmbeddedData: assets.GetEmbeddedSSHKey(),
			OutputPath:   "assets/data/hostkey.age",
		},
	}

	fmt.Printf("🔑 Starting key rotation for %d files...\n", len(files))

	if opts.DryRun {
		fmt.Printf("🧪 DRY RUN MODE - no files will be modified\n")
	}

	// Step 1: Create age identities
	previousIdentity, err := age.NewScryptIdentity(previousPassword)
	if err != nil {
		return fmt.Errorf("failed to create previous identity: %w", err)
	}

	currentRecipient, err := age.NewScryptRecipient(currentPassword)
	if err != nil {
		return fmt.Errorf("failed to create current recipient: %w", err)
	}

	// Step 2: Decrypt all files with previous password
	fmt.Printf("📖 Decrypting files with previous password...\n")
	for i := range files {
		decrypted, err := decryptWithIdentity(files[i].EmbeddedData, previousIdentity)
		if err != nil {
			return fmt.Errorf("failed to decrypt %s with previous password: %w", files[i].Name, err)
		}
		files[i].DecryptedData = decrypted
		fmt.Printf("  ✓ Decrypted %s (%.1f KB)\n", files[i].Name, float64(len(decrypted))/1024)
	}

	// Step 3: Re-encrypt all files with current password
	fmt.Printf("🔒 Re-encrypting files with new password...\n")
	for i := range files {
		encrypted, err := encryptWithRecipient(files[i].DecryptedData, currentRecipient)
		if err != nil {
			return fmt.Errorf("failed to re-encrypt %s with new password: %w", files[i].Name, err)
		}
		files[i].EncryptedData = encrypted
		fmt.Printf("  ✓ Re-encrypted %s (%.1f KB)\n", files[i].Name, float64(len(encrypted))/1024)
	}

	// Step 4: Verify new encryption by trying to decrypt with current password
	fmt.Printf("🔍 Verifying new encryption...\n")
	currentIdentity, err := age.NewScryptIdentity(currentPassword)
	if err != nil {
		return fmt.Errorf("failed to create current identity for verification: %w", err)
	}

	for i := range files {
		verified, err := decryptWithIdentity(files[i].EncryptedData, currentIdentity)
		if err != nil {
			return fmt.Errorf("verification failed for %s: %w", files[i].Name, err)
		}

		if !bytes.Equal(verified, files[i].DecryptedData) {
			return fmt.Errorf("verification failed for %s: decrypted data doesn't match", files[i].Name)
		}

		fmt.Printf("  ✓ Verified %s\n", files[i].Name)
	}

	if opts.DryRun {
		fmt.Printf("✅ DRY RUN SUCCESSFUL - All files can be safely rotated\n")
		fmt.Printf("Run without --dry-run to perform actual rotation\n")
		return nil
	}

	// Step 5: Create backups if requested
	if opts.Backup {
		fmt.Printf("💾 Creating backups...\n")
		for _, file := range files {
			backupPath := file.OutputPath + ".backup"
			if err := createBackup(file.OutputPath, backupPath); err != nil {
				return fmt.Errorf("failed to create backup for %s: %w", file.Name, err)
			}
			fmt.Printf("  ✓ Backed up to %s\n", backupPath)
		}
	}

	// Step 6: Write new encrypted files atomically
	fmt.Printf("💾 Writing rotated files...\n")
	for _, file := range files {
		if err := writeFileAtomically(file.OutputPath, file.EncryptedData); err != nil {
			return fmt.Errorf("failed to write %s: %w", file.Name, err)
		}
		fmt.Printf("  ✓ Wrote %s\n", file.OutputPath)
	}

	fmt.Printf("🎉 Key rotation completed successfully!\n")
	fmt.Printf("You can now remove PREVIOUS_AGE_ENCRYPTION_PASSWORD from your environment\n")

	return nil
}

// decryptWithIdentity decrypts age data using a specific identity
func decryptWithIdentity(encryptedData []byte, identity age.Identity) ([]byte, error) {
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

// encryptWithRecipient encrypts data using a specific recipient (armored format)
func encryptWithRecipient(data []byte, recipient age.Recipient) ([]byte, error) {
	var encryptedBuf bytes.Buffer

	// Use armored output for consistency
	armorWriter := armor.NewWriter(&encryptedBuf)

	ageWriter, err := age.Encrypt(armorWriter, recipient)
	if err != nil {
		return nil, fmt.Errorf("failed to create age writer: %w", err)
	}

	if _, err := ageWriter.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	if err := ageWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close age writer: %w", err)
	}

	if err := armorWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close armor writer: %w", err)
	}

	return encryptedBuf.Bytes(), nil
}

// createBackup creates a backup copy of a file
func createBackup(originalPath, backupPath string) error {
	// Check if original file exists
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		// Original doesn't exist, nothing to backup
		return nil
	}

	data, err := os.ReadFile(originalPath)
	if err != nil {
		return fmt.Errorf("failed to read original file: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// writeFileAtomically writes data to a file atomically by writing to a temp file first
func writeFileAtomically(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create temp file in the same directory
	tempFile, err := os.CreateTemp(dir, ".tmp_"+filepath.Base(filePath)+"_*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	tempPath := tempFile.Name()
	defer func() {
		tempFile.Close()
		os.Remove(tempPath) // Clean up temp file if something goes wrong
	}()

	// Write data to temp file
	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Sync to disk
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
