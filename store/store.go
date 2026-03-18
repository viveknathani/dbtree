package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// Connection represents a saved database connection.
type Connection struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Driver   string `json:"driver"`
	LastUsed int64  `json:"last_used"`
}

// encryptedFile is the JSON structure stored on disk.
type encryptedFile struct {
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// Store manages encrypted connection persistence.
type Store struct {
	mu       sync.Mutex
	filePath string
	key      []byte
}

var (
	ErrWrongPassword = errors.New("wrong password or corrupted data")
)

// NewStore creates a Store with a key derived from the given password.
// If the store file doesn't exist yet, it creates an empty one.
func NewStore(password string) (*Store, error) {
	dir, err := storeDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(dir, "connections.json")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// First run: create directory and empty store
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}

		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}

		key := deriveKey(password, salt)
		s := &Store{filePath: filePath, key: key}

		// Write empty connections list
		if err := s.saveWithSalt([]Connection{}, salt); err != nil {
			return nil, err
		}

		return s, nil
	}

	// Existing file: read salt, derive key, verify by decrypting
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read store file: %w", err)
	}

	var ef encryptedFile
	if err := json.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("failed to parse store file: %w", err)
	}

	key := deriveKey(password, ef.Salt)
	s := &Store{filePath: filePath, key: key}

	// Verify password by decrypting the already-read data
	if _, err := s.decryptConnections(ef); err != nil {
		return nil, ErrWrongPassword
	}

	return s, nil
}

// Load decrypts and returns all saved connections.
func (s *Store) Load() ([]Connection, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read store file: %w", err)
	}

	var ef encryptedFile
	if err := json.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("failed to parse store file: %w", err)
	}

	return s.decryptConnections(ef)
}

// decryptConnections decrypts an encryptedFile and returns the connections.
func (s *Store) decryptConnections(ef encryptedFile) ([]Connection, error) {
	plaintext, err := decrypt(s.key, ef.Nonce, ef.Ciphertext)
	if err != nil {
		return nil, ErrWrongPassword
	}

	var connections []Connection
	if err := json.Unmarshal(plaintext, &connections); err != nil {
		return nil, fmt.Errorf("failed to parse connections: %w", err)
	}

	return connections, nil
}

// Save encrypts and writes the connections to disk.
func (s *Store) Save(connections []Connection) error {
	// Read existing salt
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read store file: %w", err)
	}

	var ef encryptedFile
	if err := json.Unmarshal(data, &ef); err != nil {
		return fmt.Errorf("failed to parse store file: %w", err)
	}

	return s.saveWithSalt(connections, ef.Salt)
}

// Add appends a new connection and saves. Returns an error if a connection
// with the same name already exists.
func (s *Store) Add(conn Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	connections, err := s.Load()
	if err != nil {
		return err
	}

	for _, existing := range connections {
		if existing.Name == conn.Name {
			return fmt.Errorf("connection name %q already exists", conn.Name)
		}
	}

	conn.LastUsed = time.Now().Unix()
	connections = append(connections, conn)
	return s.Save(connections)
}

// Remove deletes a connection by name and saves.
func (s *Store) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	connections, err := s.Load()
	if err != nil {
		return err
	}

	filtered := make([]Connection, 0, len(connections))
	for _, c := range connections {
		if c.Name != name {
			filtered = append(filtered, c)
		}
	}

	return s.Save(filtered)
}

// TouchLastUsed updates the LastUsed timestamp for the named connection.
func (s *Store) TouchLastUsed(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	connections, err := s.Load()
	if err != nil {
		return err
	}

	for i := range connections {
		if connections[i].Name == name {
			connections[i].LastUsed = time.Now().Unix()
			break
		}
	}

	return s.Save(connections)
}

func (s *Store) saveWithSalt(connections []Connection, salt []byte) error {
	plaintext, err := json.Marshal(connections)
	if err != nil {
		return fmt.Errorf("failed to marshal connections: %w", err)
	}

	nonce, ciphertext, err := encrypt(s.key, plaintext)
	if err != nil {
		return err
	}

	ef := encryptedFile{
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}

	data, err := json.Marshal(ef)
	if err != nil {
		return fmt.Errorf("failed to marshal encrypted file: %w", err)
	}

	// Atomic write: write to temp file, fsync, then rename into place.
	tmpPath := s.filePath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Argon2id parameters. OWASP recommends time=2, memory=19456 as a baseline.
// Higher values are more secure but slower to unlock.
const (
	argon2Time    = 2
	argon2Memory  = 19 * 1024 // 19 MiB
	argon2Threads = 4
	argon2KeyLen  = 32
)

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
}

func encrypt(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

func decrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func storeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".dbtree"), nil
}
