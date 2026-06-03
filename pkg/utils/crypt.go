package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
)

func encryptForSnowflakeRaw(plaintext string, passphrase string) (string, error) {
	// Erzeugt aus JEDER Passphrasen-Länge exakt einen 32-Byte-Key (AES-256)
	hasher := sha256.New()
	hasher.Write([]byte(passphrase))
	key := hasher.Sum(nil) // Genau 32 Bytes lang

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 12 Bytes Nonce für GCM
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Nonce vor den Chiffretext hängen
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)

	// Als Hex-String ausgeben
	return hex.EncodeToString(ciphertext), nil
}
