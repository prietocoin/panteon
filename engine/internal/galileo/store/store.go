package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"elpanteon/engine/internal/galileo/contract"
)

// Store persiste los casos de ingeniería como artefactos JSON.
type Store struct {
	Dir string
}

// Save escribe el caso de forma atómica y devuelve la ruta del artefacto.
func (s Store) Save(engineeringCase contract.EngineeringCase) (string, error) {
	dir := s.Dir
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creando directorio %s: %w", dir, err)
	}

	body, err := json.MarshalIndent(engineeringCase, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serializando caso: %w", err)
	}
	body = append(body, '\n')

	path := filepath.Join(dir, FileName(engineeringCase.ID))
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, body, 0o644); err != nil {
		return "", fmt.Errorf("escribiendo %s: %w", temporary, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return "", fmt.Errorf("publicando %s: %w", path, err)
	}
	return path, nil
}

// FileName convierte un ID con prefijo sha256: en un nombre válido.
func FileName(id string) string {
	name := strings.ReplaceAll(id, ":", "-")
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, name)
	name = strings.Trim(name, "-.")
	if name == "" {
		name = "case"
	}
	return name + ".json"
}

// CacheKey calcula la clave de idempotencia de una ejecución.
func CacheKey(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		hash.Write([]byte(part))
		hash.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
