// Package backup creates and restores encrypted backup archives.
// A backup contains the SQLite database, blueprint YAML files,
// optional app volume tarballs, and a JSON manifest.
package backup

import (
	"archive/tar"
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
	AppVolumes   []string  `json:"app_volumes,omitempty"`
}

// AppVolume represents a single app container volume to include in the backup.
type AppVolume struct {
	AppID         string
	ContainerName string
	ContainerPath string // path inside the container to archive
}

// VolumeReader is a function that returns a tar stream for a given container path.
// Injected by the caller so this package doesn't depend on Docker directly.
type VolumeReader func(containerName, containerPath string) (io.ReadCloser, error)

const (
	manifestFile = "manifest.json"
	dbFile       = "cloud-core.db"
	bpDir        = "blueprints"
	volumesDir   = "volumes"
	keyLen       = 32 // AES-256
	saltLen      = 32
	iterations   = 100_000
	archiveExt   = ".pcg-backup"
)

// Create builds a backup archive at destPath.
// volumes and readVolume may be nil — if so, volume backup is skipped.
// passphrase may be empty — if so the archive is written unencrypted.
func Create(dbPath, blueprintsDir, destPath, passphrase string, volumes []AppVolume, readVolume VolumeReader) error {
	tmpFile, err := os.CreateTemp("", "pcg-backup-*.zip")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if err := writeZip(tmpFile, dbPath, blueprintsDir, volumes, readVolume, passphrase != ""); err != nil {
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

func writeZip(w io.Writer, dbPath, blueprintsDir string, volumes []AppVolume, readVolume VolumeReader, willEncrypt bool) error {
	zw := zip.NewWriter(w)

	// Manifest
	var volNames []string
	for _, v := range volumes {
		volNames = append(volNames, v.AppID)
	}
	manifest := Manifest{
		Version:      "2",
		CreatedAt:    time.Now().UTC(),
		DBPath:       dbFile,
		BlueprintDir: bpDir,
		Encrypted:    willEncrypt,
		AppVolumes:   volNames,
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
		dst := filepath.Join(bpDir, e.Name())
		if err := addFile(zw, src, dst); err != nil {
			return fmt.Errorf("add blueprint %s: %w", e.Name(), err)
		}
	}

	// App volumes — each stored as volumes/{app-id}/{path-base}.tar
	if readVolume != nil {
		for _, vol := range volumes {
			rc, err := readVolume(vol.ContainerName, vol.ContainerPath)
			if err != nil {
				// Non-fatal — log and continue
				continue
			}
			dst := filepath.Join(volumesDir, vol.AppID, filepath.Base(vol.ContainerPath)+".tar")
			w2, err := zw.Create(dst)
			if err != nil {
				rc.Close()
				continue
			}
			_, _ = io.Copy(w2, rc)
			rc.Close()
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

// Restore extracts a backup archive and restores the DB and blueprints.
// It does NOT restart the server — the caller must do that.
func Restore(srcPath, passphrase, dbDest, blueprintsDest string) error {
	var reader io.Reader

	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()

	if passphrase != "" {
		reader, err = decryptStream(f, passphrase)
		if err != nil {
			return fmt.Errorf("decrypt: %w", err)
		}
	} else {
		reader = f
	}

	// Read all into memory to get the zip
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	zr, err := zip.NewReader(newBytesReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			continue
		}

		switch {
		case file.Name == dbFile:
			if err := writeToPath(rc, dbDest); err != nil {
				rc.Close()
				return fmt.Errorf("restore db: %w", err)
			}
		case filepath.Dir(file.Name) == bpDir:
			dst := filepath.Join(blueprintsDest, filepath.Base(file.Name))
			_ = writeToPath(rc, dst) // best-effort blueprint restore
		}
		rc.Close()
	}
	return nil
}

// RestoreVolume extracts the volume tar from a backup into a destination directory.
// Returns a tar reader so the caller can copy files directly into a container.
func ExtractVolumeTar(srcPath, passphrase, appID, pathBase string) (*tar.Reader, func(), error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, nil, err
	}

	var reader io.Reader = f
	if passphrase != "" {
		dec, err := decryptStream(f, passphrase)
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		reader = dec
	}

	data, err := io.ReadAll(reader)
	f.Close()
	if err != nil {
		return nil, nil, err
	}

	zr, err := zip.NewReader(newBytesReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, err
	}

	targetName := filepath.Join(volumesDir, appID, pathBase+".tar")
	for _, file := range zr.File {
		if file.Name == targetName {
			rc, err := file.Open()
			if err != nil {
				return nil, nil, err
			}
			return tar.NewReader(rc), func() { rc.Close() }, nil
		}
	}
	return nil, nil, fmt.Errorf("volume %s not found in backup", targetName)
}

func writeToPath(r io.Reader, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

// encryptStream encrypts src using AES-256-GCM + PBKDF2.
// Format: [32-byte salt][12-byte nonce][ciphertext+GCM tag]
func encryptStream(src io.Reader, dst io.Writer, passphrase string) error {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}
	key := pbkdf2.Key([]byte(passphrase), salt, iterations, keyLen, sha256.New)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	plaintext, err := io.ReadAll(src)
	if err != nil {
		return err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	if _, err := dst.Write(salt); err != nil {
		return err
	}
	_, err = dst.Write(ciphertext)
	return err
}

// decryptStream decrypts an AES-256-GCM stream.
func decryptStream(src io.Reader, passphrase string) (io.Reader, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(src, salt); err != nil {
		return nil, fmt.Errorf("read salt: %w", err)
	}

	key := pbkdf2.Key([]byte(passphrase), salt, iterations, keyLen, sha256.New)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)

	ciphertext, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	plaintext, err := gcm.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed — wrong passphrase?")
	}

	return newBytesReader(plaintext), nil
}

// newBytesReader wraps a byte slice as an io.Reader.
type bytesReader struct{ b []byte; pos int }
func newBytesReader(b []byte) *bytesReader { return &bytesReader{b: b} }
func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.b) { return 0, io.EOF }
	n := copy(p, r.b[r.pos:])
	r.pos += n
	return n, nil
}
func (r *bytesReader) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r.b)) { return 0, io.EOF }
	n := copy(p, r.b[off:])
	if n < len(p) { return n, io.EOF }
	return n, nil
}
func (r *bytesReader) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:   newPos = offset
	case io.SeekCurrent: newPos = int64(r.pos) + offset
	case io.SeekEnd:     newPos = int64(len(r.b)) + offset
	}
	if newPos < 0 { return 0, fmt.Errorf("negative seek") }
	r.pos = int(newPos)
	return newPos, nil
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
		if e.IsDir() || filepath.Ext(e.Name()) != archiveExt {
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
