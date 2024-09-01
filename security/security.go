package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"emlserver/config"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	MAX_PASS_LENGTH = 72
)

func InitSecurity() {
}

func GenerateID() string {
	return strconv.FormatInt(time.Now().UnixMilli(), 10)
}

func GenerateUUID() string {
	token := uuid.New().String()
	return token
}

func PassHash(password string) string {
	res, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(res)
}

func PasswordsMatch(hashedPassword string, password string) bool {
	buffer, _ := hex.DecodeString(hashedPassword)
	return bcrypt.CompareHashAndPassword(buffer, []byte(password)) == nil
}

func Hash(toHash string) string {
	hash := sha256.Sum256([]byte(toHash))
	stringed := fmt.Sprintf("%x", hash)
	return stringed
}

func Encrypt(toEncrypt string) string {
	key := []byte(config.LoadedConfig["ENCRYPT_KEY"])
	c, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println(err)
	}
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		fmt.Println(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		fmt.Println(err)
	}

	encrypted := base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(toEncrypt), nil))
	return encrypted
}

func Decrypt(toDecrypt string) string {
	crypt, err := aes.NewCipher([]byte(config.LoadedConfig["ENCRYPT_KEY"]))
	if err != nil {
		fmt.Println(err)
	}

	gcm, err := cipher.NewGCM(crypt)
	if err != nil {
		fmt.Println(err)
	}

	decoded, _ := base64.StdEncoding.DecodeString(toDecrypt)

	nonceSize := gcm.NonceSize()
	if len(decoded) < nonceSize {
		fmt.Println(err)
	}

	nonce, ciphertext := decoded[:nonceSize], decoded[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		fmt.Println(err)
	}
	return string(plaintext)
}

func CompareHashToString(rawString string, hash string) bool {
	hashed := Hash(rawString)
	return hash == hashed
}
