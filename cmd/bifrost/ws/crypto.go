package ws

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
)

var cipherKey []byte

func init() {
	hexKey := "904F4BD34C303D2A2F5C609D3DEF710FEEBDE497DF4840380927D744D343D4CD" 
	cipherKey, _ = hex.DecodeString(hexKey)
	// fmt.Printf("cipher key: %v\n", cipherKey)
}

// refer: https://golang.org/pkg/crypto/cipher/#example_NewCFBDecrypter
func Encrypt(key, input []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(input))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], input)
	return ciphertext, nil
}

func Decrypt(key, input []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	if len(input) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := input[:aes.BlockSize]
	input = input[aes.BlockSize:]
	// XORKeyStream can work input-place if the two arguments are the same.
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(input, input)
	return input, nil
}