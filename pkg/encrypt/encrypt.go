package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

func makeIV() []byte {
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		panic(err)
	}
	return iv
}

func Encrypt(key, plaintext string) []byte {
	block, err := newBlockCipher(key)
	if err != nil {
		panic(err)
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		panic(err)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(plaintext))

	//fmt.Printf("IV: %x\n", iv)

	return ciphertext
}

func Decrypt(key string, ciphertext []byte) string {
   if len(ciphertext) < aes.BlockSize {
		panic("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ctext := ciphertext[aes.BlockSize:]
   result := make([]byte, len(ctext))

	stream, _ := decryptStream(key, iv)

	stream.XORKeyStream(result, ctext)
	return string(result)
}

func newBlockCipher(key string) (cipher.Block, error) {
	hasher := md5.New()
	fmt.Fprint(hasher, key)
	cipherKey := hasher.Sum(nil)
	return aes.NewCipher(cipherKey)
}

// encryptStream expec iv and doesn't create it itself because it will be written to
// the beginning of the ciphertext and therefore is needed outside of this function
func encryptStream(key string, iv []byte) (cipher.Stream, error){
	block, err := newBlockCipher(key)
	if err != nil {
		return nil, err
	}

	return cipher.NewCFBEncrypter(block, iv), nil
}

func EncryptWriter(w io.Writer, key string) (*cipher.StreamWriter, error){
	// Create IV
	iv := makeIV()

	// Create Stream
	stream, err := encryptStream(key, iv)
	if err != nil {
		panic(err)
	}

	// Write IV to the beginning of the file
	n, e := w.Write(iv)
	if n != aes.BlockSize || e != nil {
		return nil, errors.New("Unable to write IV to the writer\n")
	}

	return &cipher.StreamWriter{S: stream, W: w}, nil
}

func decryptStream(key string, iv[]byte) (cipher.Stream, error) {
	block, err := newBlockCipher(key)
	if err != nil {
		return nil, err
	}

	return cipher.NewCFBDecrypter(block, iv), nil
}

func DecryptReader(r io.Reader, key string) (*cipher.StreamReader, error) {
	// Read IV
	iv := make([]byte, aes.BlockSize)
	n, err := r.Read(iv)
	if n < aes.BlockSize || err != nil {
		return nil, errors.New("Unable to read from reader\n")
	}

	// Create Stream
	stream, err := decryptStream(key, iv)
	if err != nil {
		return nil, err
	}

	return &cipher.StreamReader{S: stream, R: r}, nil
}
