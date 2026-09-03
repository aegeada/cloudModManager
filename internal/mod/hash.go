package mod

import (
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// ComputeSHA512 calculates the SHA-512 checksum of a file.
func ComputeSHA512(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha512.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ComputeSHA1 calculates the SHA-1 checksum of a file.
func ComputeSHA1(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// VerifyFileSHA512 verifies that the file at path matches the expected SHA-512 checksum.
func VerifyFileSHA512(path string, expected string) (bool, error) {
	if expected == "" {
		return true, nil
	}
	hash, err := ComputeSHA512(path)
	if err != nil {
		return false, err
	}
	if !strings.EqualFold(hash, expected) {
		return false, fmt.Errorf("SHA-512 checksum mismatch: expected %s, got %s", expected, hash)
	}
	return true, nil
}
