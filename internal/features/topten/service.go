package topten

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"

	"prospero/assets"
)

type Service struct {
	collection TopTenCollection
}

func NewService(ctx context.Context) (*Service, error) {
	encryptedData := assets.GetEmbeddedTopTenData()

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

	// Read the decrypted JSON data
	decryptedData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted data: %w", err)
	}

	var collection TopTenCollection
	if err := json.Unmarshal(decryptedData, &collection); err != nil {
		return nil, fmt.Errorf("failed to unmarshal top ten data: %w", err)
	}

	if len(collection.Lists) == 0 {
		return nil, fmt.Errorf("no top ten lists found in data")
	}

	return &Service{
		collection: collection,
	}, nil
}

func (s *Service) GetRandomList() (*TopTenList, error) {
	if len(s.collection.Lists) == 0 {
		return nil, fmt.Errorf("no lists available")
	}

	max := big.NewInt(int64(len(s.collection.Lists)))
	randomIndex, err := rand.Int(rand.Reader, max)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random number: %w", err)
	}

	list := s.collection.Lists[randomIndex.Int64()]
	return &list, nil
}

func (s *Service) GetListCount() int {
	return len(s.collection.Lists)
}

// DecryptSSHHostKey decrypts the embedded AGE-encrypted SSH host key
func DecryptSSHHostKey(ctx context.Context) ([]byte, error) {
	encryptedKey := assets.GetEmbeddedSSHKey()

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
	if bytes.HasPrefix(encryptedKey, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		// It's armored, decode it first
		armorReader := armor.NewReader(bytes.NewReader(encryptedKey))
		ageReader = armorReader
	} else {
		// It's binary format
		ageReader = bytes.NewReader(encryptedKey)
	}

	// Decrypt the age data
	reader, err := age.Decrypt(ageReader, identity)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt SSH host key: %w", err)
	}

	// Read the decrypted SSH key data
	decryptedKey, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted SSH key: %w", err)
	}

	// Validate it looks like an SSH private key
	keyStr := string(decryptedKey)
	if !strings.HasPrefix(keyStr, "-----BEGIN OPENSSH PRIVATE KEY-----") {
		return nil, fmt.Errorf("decrypted data does not appear to be a valid OpenSSH private key")
	}
	if !strings.HasSuffix(strings.TrimSpace(keyStr), "-----END OPENSSH PRIVATE KEY-----") {
		return nil, fmt.Errorf("decrypted data does not appear to be a complete OpenSSH private key")
	}

	return decryptedKey, nil
}

// ValidatePassword validates that the AGE_ENCRYPTION_PASSWORD can decrypt all encrypted assets
func ValidatePassword(ctx context.Context) error {
	// Check if password is set
	password := os.Getenv("AGE_ENCRYPTION_PASSWORD")
	if password == "" {
		return fmt.Errorf("AGE_ENCRYPTION_PASSWORD environment variable is not set")
	}

	// Create age identity for decryption
	identity, err := age.NewScryptIdentity(password)
	if err != nil {
		return fmt.Errorf("failed to create age identity: %w", err)
	}

	// Test 1: Decrypt topten.json.age
	encryptedTopTen := assets.GetEmbeddedTopTenData()
	var ageReader io.Reader
	if bytes.HasPrefix(encryptedTopTen, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		armorReader := armor.NewReader(bytes.NewReader(encryptedTopTen))
		ageReader = armorReader
	} else {
		ageReader = bytes.NewReader(encryptedTopTen)
	}

	reader, err := age.Decrypt(ageReader, identity)
	if err != nil {
		return fmt.Errorf("failed to decrypt topten data with provided password: %w", err)
	}

	// Read a few bytes to ensure decryption actually works
	testBuf := make([]byte, 10)
	_, err = io.ReadAtLeast(reader, testBuf, 1)
	if err != nil {
		return fmt.Errorf("failed to read decrypted topten data: %w", err)
	}

	// Test 2: Decrypt hostkey.age
	encryptedKey := assets.GetEmbeddedSSHKey()
	if bytes.HasPrefix(encryptedKey, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		armorReader := armor.NewReader(bytes.NewReader(encryptedKey))
		ageReader = armorReader
	} else {
		ageReader = bytes.NewReader(encryptedKey)
	}

	reader, err = age.Decrypt(ageReader, identity)
	if err != nil {
		return fmt.Errorf("failed to decrypt SSH host key with provided password: %w", err)
	}

	// Read a few bytes to ensure decryption actually works
	_, err = io.ReadAtLeast(reader, testBuf, 1)
	if err != nil {
		return fmt.Errorf("failed to read decrypted SSH host key: %w", err)
	}

	return nil
}
