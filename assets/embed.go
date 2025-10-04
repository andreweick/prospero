package assets

import (
	"embed"
)

//go:embed data/topten.json.age
var topTenData []byte

//go:embed data/hostkey.age
var sshHostKey []byte

//go:embed data/shakespert.db
var shakespertDB []byte

//go:embed prompts
var promptFiles embed.FS

// GetEmbeddedTopTenData returns the embedded encrypted Top Ten data
func GetEmbeddedTopTenData() []byte {
	return topTenData
}

// GetEmbeddedSSHKey returns the embedded encrypted SSH host key
func GetEmbeddedSSHKey() []byte {
	return sshHostKey
}

// GetEmbeddedShakespertDB returns the embedded Shakespeare database
func GetEmbeddedShakespertDB() []byte {
	return shakespertDB
}

// GetEmbeddedPrompts returns the embedded prompts filesystem
func GetEmbeddedPrompts() embed.FS {
	return promptFiles
}
