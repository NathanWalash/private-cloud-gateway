// Package backup creates and restores encrypted backup archives.
// A backup contains the SQLite database, all blueprint YAML files,
// and a manifest describing the contents.
package backup

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// Manifest describes what is inside a backup archive.
type Manifest struct {
	Version      string    `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	DBPath       string    `json:"db_path"`
	BlueprintDir string    `json:"blueprint_dir"`
	Encrypted    bool      `json:"encrypted"`
}

const (
	manifestFile = "manifest.json"
	dbFile       = "cloud-core.db"
	blueprintDir = "blueprints"
	keyLen       = 32 // AES-256
	saltLen      = 32
	iterations   = 100_000
	archiveExt   = ".pcg-backup"
)

// Create builds an encrypted backup archive at destPath.
// passphrase may be empty — if so the archive is written unencrypted.
func Create(dbPath, blueprintsDir, destPath, passphrase string) error {
	// Build a plain zip in memory first, then encrypt.
	tmpFile, err := os.CreateTemp("", "pcg-backup-*.zip")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if err := writeZip(tmpFile, dbPath, blueprintsDir, passphrase != ""); err != nil {
		return fmt.Errorf("write zip: %w", err)
	}
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return err
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer out.Close()

	if passphrase == "" {
		_, err = io.Copy(out, tmpFile)
		return err
	}
	return encryptStream(tmpFile, out, passphrase)
}

// writeZip writes a zip archive containing the DB, blueprints, and manifest.
func writeZip(w io.Writer, dbPath, blueprintsDir string, willEncrypt bool) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	// Manifest
	manifest := Manifest{
		Version:      "1",
		CreatedAt:    time.Now().UTC(),
		DBPath:       dbFile,
		BlueprintDir: blueprintDir,
		Encrypted:    willEncrypt,
	}
	mf, err := zw.Create(manifestFile)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(mf).Encode(manifest); err != nil {
		return err
	}

	// SQLite database
	if err := addFile(zw, dbPath, dbFile); err != nil {
		return fmt.Errorf("add db: %w", err)
	}

	// Blueprint YAML files
	entries, _ := os.ReadDir(blueprintsDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(blueprintsDir, e.Name())
		dst := filepath.Join(blueprintDir, e.Name())
		if err := addFile(zw, src, dst); err != nil {
			return fmt.Errorf("add blueprint %s: %w", e.Name(), err)
		}
	}

	return zw.Close()
}

func addFile(zw *zip.Writer, src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	w, err := zw.Create(dst)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

// encryptStream encrypts src into dst using AES-256-GCM with PBKDF2 key derivation.
// Format: [32-byte salt][12-byte nonce][ciphertext+tag]
func encryptStream(src io.Reader, dst io.Writer, passphrase string) error {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}

	key := pbkdf2.Key([]byte(passphrase), salt, iterations, keyLen, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	plaintext, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("read plaintext: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	if _, err := dst.Write(salt); err != nil {
		return err
	}
	_, err = dst.Write(ciphertext)
	return err
}

// FileName returns the backup filename for a given time.
func FileName(t time.Time) string {
	return fmt.Sprintf("pcg-backup-%s%s", t.UTC().Format("20060102-150405"), archiveExt)
}

// ListBackups returns all backup files in dir, newest first.
func ListBackups(dir string) ([]BackupInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []BackupInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != archiveExt {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, BackupInfo{
			Name:      e.Name(),
			Size:      info.Size(),
			CreatedAt: info.ModTime().UTC(),
		})
	}
	// Reverse for newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

// BackupInfo describes a backup file on disk.
type BackupInfo struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}
