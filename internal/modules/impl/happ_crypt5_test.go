package impl

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"strconv"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

const happCrypt5ExamplePayload = "fzvdO4bMOfTWNaB3taWRhRaF64soexaE9dm3ZlLK0Rke9Rz3BG1f9gmj4tSDpjRSWWdX5G6oaaQK9Gs+fdJPWNXIF08BsaeifWtfTlCvC/nSWDv0ZrofgJXkQ8MlUk63CoJkt7RvXAfablYXb/cYWEZJQkDMyMaE5/1KegAbWVWVI60MPqDylUyYoLtOeOOX9amvELecOZ4kKz1QVqgE9uCBz3py+3Ghr1iVGKOhFwb98OFP+j0tGvDo/3d609DVq3RwBGXu1ogZ7PTc3/A5IlaA3Hff5IlVujozQ3ywmQBsTGd+l3AHJAX1oDbPkRSSwg7Y7hl3AKXKpZsEhMzPbJY8UxZ7GmVsxeROLopVx85ACqakzg+ZZwdZslfKgdRzUmL9Mv895HDOHE3tbh6qnDhE9Ew/Epx1iBCb2HjorOLDBluH8ztdL9mdUX+turjC4GLN0YR55P3H23A0W5zl0di5YfrI2nUBKxh30lUVG9NbqYKlwxgmhxAUZrQ6cnFGF2VuZ9VJLQ3sQ8rqXTtfau8ySbu770Hd6vVEun8aSJm4W4M0DagKbORL4A4M6Cuf1v/jj7EWhA9yhhcSuxkc6WUWMGYRraWBM9vSrSbjeT1U69a3n9T56M/TOJWf4z8fXrHRnCR1tfzwrjHiOJfZGiKvbAd4k6f+VADYpLdCq+ornEElv7V0sByfwPTgep+Q33Qkl67ArHpmbZDcCyWkGz0BoUzGJFe58YiS/oNFVdufbuDnFd1ArAVMztJJJbxlo4Is48+Iof=ff"

func TestHappDecryptCrypt5SaltedPayload(t *testing.T) {
	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := engine.Decrypt("happ://crypt5/" + happCrypt5ExamplePayload)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if want := "https://example.com/sub"; got.DecryptedData != want {
		t.Fatalf("DecryptedData = %q, want %q", got.DecryptedData, want)
	}
	if got.Version != VersionCrypt5 {
		t.Fatalf("Version = %q, want %q", got.Version, VersionCrypt5)
	}
	if got.UsedKey != VersionCrypt5 {
		t.Fatalf("UsedKey = %q, want %q", got.UsedKey, VersionCrypt5)
	}
}

func TestHappDecryptCrypt5TamperedPayload(t *testing.T) {
	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tampered := []byte(happCrypt5ExamplePayload)
	tampered[200] = byte('A')
	if happCrypt5ExamplePayload[200] == 'A' {
		tampered[200] = byte('B')
	}

	if _, err := engine.Decrypt("happ://crypt5/" + string(tampered)); err == nil {
		t.Fatal("a tampered crypt5 payload decrypted successfully")
	}
}

func TestHappDecryptCrypt5UnknownMarker(t *testing.T) {
	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = engine.Decrypt("happ://crypt5/aaaabbbb")
	if !errors.Is(err, ErrCrypt5UnknownMarker) {
		t.Fatalf("err = %v, want %v", err, ErrCrypt5UnknownMarker)
	}
}

func TestHappDecryptCrypt5LegacyPayload(t *testing.T) {
	const want = "https://example.com/legacy"

	payload := happCrypt5LegacyPayload(t, "asajzqxt", want)

	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := engine.Decrypt("happ://crypt5/" + payload)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got.DecryptedData != want {
		t.Fatalf("DecryptedData = %q, want %q", got.DecryptedData, want)
	}
}

func happCrypt5LegacyPayload(t *testing.T, marker, url string) string {
	t.Helper()

	key, err := happCrypt5Keys.privateKey(marker)
	if err != nil {
		t.Fatalf("privateKey(%q): %v", marker, err)
	}

	chachaKey := make([]byte, happCrypt5KeySize)
	nonce := make([]byte, happCrypt5NonceSize)
	if _, err := rand.Read(chachaKey); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	aead, err := chacha20poly1305.New(chachaKey)
	if err != nil {
		t.Fatalf("chacha20poly1305.New: %v", err)
	}
	sealed := aead.Seal(nil, nonce, happSwapAdjacent([]byte(base64.StdEncoding.EncodeToString([]byte(url)))), nil)
	sealedURL := base64.StdEncoding.EncodeToString(sealed)

	block, err := rsa.EncryptPKCS1v15(rand.Reader, &key.PublicKey, happSwapAdjacent([]byte(base64.StdEncoding.EncodeToString(chachaKey))))
	if err != nil {
		t.Fatalf("EncryptPKCS1v15: %v", err)
	}

	body := string(nonce) + strconv.Itoa(len(sealedURL)) + "|" + sealedURL + base64.StdEncoding.EncodeToString(block)
	return string(happSwapBlockHalves([]byte(marker[:4] + body + marker[4:])))
}
