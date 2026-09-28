package impl

import (
	"cmp"
	"crypto/rsa"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
)

//go:embed assets/happ/crypt5-keys.json
var happCrypt5KeysJSON []byte

const (
	happCrypt5NonceSize  = 12
	happCrypt5SaltOffset = happCrypt5NonceSize + 2
	happCrypt5SaltSize   = 8
	happCrypt5KeySize    = 32
)

var (
	ErrCrypt5PayloadTooShort    = errors.New("crypt5 payload is too short")
	ErrCrypt5UnknownMarker      = errors.New("crypt5 payload names a marker with no bundled key")
	ErrCrypt5BodyTooShort       = errors.New("crypt5 body is too short")
	ErrCrypt5SaltedBodyTooShort = errors.New("crypt5 salted body is too short")
	ErrCrypt5SegmentLength      = errors.New("crypt5 segment length is missing")
	ErrCrypt5SegmentTruncated   = errors.New("crypt5 segment is truncated")
	ErrCrypt5KeySize            = errors.New("crypt5 recovered key is not 32 bytes")
	ErrCrypt5Authentication     = errors.New("crypt5 authentication failed")
)

var happCrypt5KeyTable = sync.OnceValues(func() (map[string]string, error) {
	var table map[string]string
	if err := json.Unmarshal(happCrypt5KeysJSON, &table); err != nil {
		return nil, fmt.Errorf("happ: crypt5 key table: %w", err)
	}
	if len(table) == 0 {
		return nil, errors.New("happ: crypt5 key table is empty")
	}
	return table, nil
})

type happCrypt5KeyStore struct {
	mu     sync.Mutex
	parsed map[string]*rsa.PrivateKey
}

var happCrypt5Keys = &happCrypt5KeyStore{parsed: make(map[string]*rsa.PrivateKey)}

func (s *happCrypt5KeyStore) privateKey(marker string) (*rsa.PrivateKey, error) {
	table, err := happCrypt5KeyTable()
	if err != nil {
		return nil, err
	}

	encoded, ok := table[marker]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrCrypt5UnknownMarker, marker)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if key, ok := s.parsed[marker]; ok {
		return key, nil
	}

	der, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("happ: crypt5 key %q: %w", marker, err)
	}

	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("happ: crypt5 key %q: %w", marker, err)
	}

	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("happ: crypt5 key %q: %w", marker, ErrPrivateKeyNotRSA)
	}

	s.parsed[marker] = key
	return key, nil
}

func decryptCrypt5(payload string) (string, error) {
	shuffled := happSwapBlockHalves([]byte(payload))
	if len(shuffled) < 8 {
		return "", ErrCrypt5PayloadTooShort
	}

	marker := string(shuffled[:4]) + string(shuffled[len(shuffled)-4:])
	key, err := happCrypt5Keys.privateKey(marker)
	if err != nil {
		return "", err
	}

	body := shuffled[4 : len(shuffled)-4]
	salted := len(body) > happCrypt5NonceSize && !happIsASCIIDigit(body[happCrypt5NonceSize])

	var firstErr error
	for _, attempt := range [2]bool{salted, !salted} {
		url, err := happCrypt5Body(body, key, attempt)
		if err == nil {
			return url, nil
		}
		firstErr = cmp.Or(firstErr, err)
	}
	return "", firstErr
}

func happCrypt5Body(body []byte, key *rsa.PrivateKey, salted bool) (string, error) {
	if len(body) < happCrypt5NonceSize+1 {
		return "", ErrCrypt5BodyTooShort
	}

	nonce := body[:happCrypt5NonceSize]
	lengthStart := happCrypt5NonceSize

	var salt []byte
	if salted {
		if len(body) < happCrypt5SaltOffset+happCrypt5SaltSize+1 {
			return "", ErrCrypt5SaltedBodyTooShort
		}
		salt = body[happCrypt5SaltOffset : happCrypt5SaltOffset+happCrypt5SaltSize]
		lengthStart = happCrypt5SaltOffset + happCrypt5SaltSize
	}

	lengthEnd := lengthStart
	for lengthEnd < len(body) && happIsASCIIDigit(body[lengthEnd]) {
		lengthEnd++
	}
	if lengthEnd == lengthStart {
		return "", ErrCrypt5SegmentLength
	}

	segmentLength, err := strconv.Atoi(string(body[lengthStart:lengthEnd]))
	if err != nil {
		return "", fmt.Errorf("happ: crypt5 segment length: %w", err)
	}

	packed := body[lengthEnd:]
	if len(packed) == 0 || segmentLength > len(packed)-1 {
		return "", ErrCrypt5SegmentTruncated
	}

	sealedURL := string(packed[1 : segmentLength+1])
	rsaBlock := string(packed[segmentLength+1:])

	rsaCiphertext, err := b64DecodeUrlSafe(rsaBlock)
	if err != nil {
		return "", fmt.Errorf("happ: crypt5 RSA block: %w", err)
	}

	rsaPlaintext, err := rsa.DecryptPKCS1v15(nil, key, rsaCiphertext)
	if err != nil {
		return "", fmt.Errorf("happ: crypt5 RSA: %w", err)
	}

	chachaKey, err := b64DecodeUrlSafe(string(happSwapAdjacent(rsaPlaintext)))
	if err != nil {
		return "", fmt.Errorf("happ: crypt5 key: %w", err)
	}
	if len(chachaKey) != happCrypt5KeySize {
		return "", fmt.Errorf("%w: got %d bytes", ErrCrypt5KeySize, len(chachaKey))
	}

	if len(salt) > 0 {
		for i := range chachaKey {
			chachaKey[i] ^= salt[i%len(salt)]
		}
	}

	ciphertext, err := b64DecodeUrlSafe(sealedURL)
	if err != nil {
		return "", fmt.Errorf("happ: crypt5 ciphertext: %w", err)
	}

	aead, err := chacha20poly1305.New(chachaKey)
	if err != nil {
		return "", fmt.Errorf("happ: crypt5 ChaCha20-Poly1305: %w", err)
	}

	intermediate, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrCrypt5Authentication
	}

	plaintext, err := b64DecodeUrlSafe(string(happSwapAdjacent(intermediate)))
	if err != nil {
		return "", fmt.Errorf("happ: crypt5 plaintext: %w", err)
	}

	return string(plaintext), nil
}

func happIsASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func happSwapAdjacent(src []byte) []byte {
	out := slices.Clone(src)
	for i := 0; i+1 < len(out); i += 2 {
		out[i], out[i+1] = out[i+1], out[i]
	}
	return out
}

func happSwapBlockHalves(src []byte) []byte {
	out := slices.Clone(src)
	for i := 0; i+3 < len(out); i += 4 {
		out[i], out[i+2] = out[i+2], out[i]
		out[i+1], out[i+3] = out[i+3], out[i+1]
	}
	return out
}
