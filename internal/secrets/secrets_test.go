package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustBox(t *testing.T) (*Box, []byte) {
	t.Helper()
	key, err := NewRandomKey()
	if err != nil {
		t.Fatalf("NewRandomKey: %v", err)
	}
	box, err := NewBox(key)
	if err != nil {
		t.Fatalf("NewBox: %v", err)
	}
	return box, key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	box, _ := mustBox(t)
	for _, plaintext := range []string{"", "s3cret", "multi\nline\tvalue", strings.Repeat("x", 4096)} {
		ct, err := box.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plaintext, err)
		}
		if !strings.HasPrefix(ct, "v1:") {
			t.Fatalf("ciphertext missing v1: prefix: %q", ct)
		}
		if strings.Contains(ct, plaintext) && plaintext != "" {
			t.Fatalf("ciphertext contains plaintext: %q", ct)
		}
		got, err := box.Decrypt(ct)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if got != plaintext {
			t.Fatalf("round trip mismatch: got %q want %q", got, plaintext)
		}
	}
}

func TestEncrypt_NoncesDiffer(t *testing.T) {
	box, _ := mustBox(t)
	a, _ := box.Encrypt("same")
	b, _ := box.Encrypt("same")
	if a == b {
		t.Fatal("two encryptions of the same plaintext produced identical ciphertext")
	}
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	box1, _ := mustBox(t)
	box2, _ := mustBox(t)
	ct, err := box1.Encrypt("s3cret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := box2.Decrypt(ct); err == nil {
		t.Fatal("decrypt with a different key succeeded")
	}
}

func TestDecrypt_TamperedFails(t *testing.T) {
	box, _ := mustBox(t)
	ct, err := box.Encrypt("s3cret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	tampered := ct[:len(ct)-2] + "AA"
	if tampered == ct {
		tampered = ct[:len(ct)-2] + "BB"
	}
	if _, err := box.Decrypt(tampered); err == nil {
		t.Fatal("decrypt of tampered ciphertext succeeded")
	}
	if _, err := box.Decrypt("plaintext-no-prefix"); err == nil {
		t.Fatal("decrypt of unprefixed value succeeded")
	}
}

func TestNewBox_RejectsBadKeySize(t *testing.T) {
	if _, err := NewBox(make([]byte, 16)); err == nil {
		t.Fatal("NewBox accepted a 16-byte key")
	}
}

func TestLoadOrCreateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.key")

	key1, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatalf("LoadOrCreateKey (create): %v", err)
	}
	if len(key1) != KeySize {
		t.Fatalf("key length = %d, want %d", len(key1), KeySize)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("key file mode = %o, want 600", perm)
	}

	key2, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatalf("LoadOrCreateKey (reload): %v", err)
	}
	if string(key1) != string(key2) {
		t.Fatal("reloaded key differs from created key")
	}
}

func TestLoadOrCreateKey_RejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.key")
	if err := os.WriteFile(path, []byte("not-hex"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateKey(path); err == nil {
		t.Fatal("LoadOrCreateKey accepted a corrupt key file")
	}
}
