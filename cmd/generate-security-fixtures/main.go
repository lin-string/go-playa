// Command generate-security-fixtures creates deterministic modern Standard
// Security PDFs used by the security and recovery corpus tests.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var (
	password = []byte("secret")
	fileKey  = []byte("01234567890123456789012345678901")
	fileID   = []byte("modern-security-fixture")
)

func main() {
	check := flag.Bool("check", false, "check generated fixtures without modifying them")
	flag.Parse()
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	outDir := filepath.Join(root, "testdata", "files")
	for revision, name := range map[int]string{5: "security_r5_aes256.pdf", 6: "security_r6_aes256.pdf"} {
		path := filepath.Join(outDir, name)
		data := modernPDF(revision)
		if *check {
			current, err := os.ReadFile(path)
			if err != nil {
				fatal(fmt.Errorf("read %s: %w", path, err))
			}
			if string(current) != string(data) {
				fatal(fmt.Errorf("security fixture is out of date: %s", path))
			}
			continue
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fatal(fmt.Errorf("write %s: %w", path, err))
		}
	}
}

func modernPDF(revision int) []byte {
	uValidationSalt := []byte(fmt.Sprintf("uvsalt0%d", revision))
	uKeySalt := []byte(fmt.Sprintf("uksalt0%d", revision))
	oValidationSalt := []byte(fmt.Sprintf("ovsalt0%d", revision))
	oKeySalt := []byte(fmt.Sprintf("oksalt0%d", revision))
	user := append(append(append([]byte(nil), passwordHash(password, uValidationSalt, nil, revision)...), uValidationSalt...), uKeySalt...)
	owner := append(append(append([]byte(nil), passwordHash(password, oValidationSalt, user, revision)...), oValidationSalt...), oKeySalt...)
	userKey := passwordHash(password, uKeySalt, nil, revision)
	ownerKey := passwordHash(password, oKeySalt, user, revision)
	ue := aesCBCNoPadding(userKey, make([]byte, aes.BlockSize), fileKey)
	oe := aesCBCNoPadding(ownerKey, make([]byte, aes.BlockSize), fileKey)
	permissions := int32(-4)
	permsPlain := make([]byte, 16)
	binary.LittleEndian.PutUint32(permsPlain[:4], uint32(permissions))
	for i := 4; i < 8; i++ {
		permsPlain[i] = 0xff
	}
	permsPlain[8] = 'F'
	copy(permsPlain[9:12], []byte("adb"))
	permsPlain[12] = 'T'
	perms := aesCBCNoPadding(fileKey, make([]byte, aes.BlockSize), permsPlain)

	text := fmt.Sprintf("Modern R%d encrypted report", revision)
	iv := make([]byte, aes.BlockSize)
	for i := range iv {
		iv[i] = byte(i + 1)
	}
	content := aesPKCS7(fileKey, iv, []byte("BT /F1 12 Tf 72 740 Td ("+text+") Tj ET"))
	content = append(append([]byte(nil), iv...), content...)

	objects := map[int][]byte{
		1: []byte("<< /Type /Catalog /Pages 2 0 R /Metadata 6 0 R >>"),
		2: []byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		3: []byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>"),
		4: []byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"),
		5: stream(content),
		6: streamWithDict([]byte("/Type /Metadata /Subtype /XML"), []byte("<x:xmpmeta><dc:title>Modern encrypted report</dc:title></x:xmpmeta>")),
		9: []byte(fmt.Sprintf("<< /Filter /Standard /V 5 /R %d /Length 256 /O <%s> /U <%s> /OE <%s> /UE <%s> /P %d /EncryptMetadata false /Perms <%s> /CF << /StdCF << /Type /CryptFilter /CFM /AESV3 /Length 32 >> >> >>", revision, hex.EncodeToString(owner), hex.EncodeToString(user), hex.EncodeToString(oe), hex.EncodeToString(ue), permissions, hex.EncodeToString(perms))),
	}
	return writePDF(objects)
}

func passwordHash(password, salt, vector []byte, revision int) []byte {
	input := make([]byte, 0, len(password)+len(salt)+len(vector))
	input = append(input, password...)
	input = append(input, salt...)
	input = append(input, vector...)
	if revision == 5 {
		hash := sha256.Sum256(input)
		return append([]byte(nil), hash[:]...)
	}
	k := sha256.Sum256(input)
	state := append([]byte(nil), k[:]...)
	for count := 1; ; count++ {
		block := make([]byte, 0, len(password)+len(state)+len(vector))
		block = append(block, password...)
		block = append(block, state...)
		block = append(block, vector...)
		input = make([]byte, 0, len(block)*64)
		for i := 0; i < 64; i++ {
			input = append(input, block...)
		}
		e := aesCBCWithIV(state[:16], state[16:32], input)
		var next []byte
		switch byteSum(e[:16]) % 3 {
		case 0:
			hash := sha256.Sum256(e)
			next = hash[:]
		case 1:
			hash := sha512.Sum384(e)
			next = hash[:]
		default:
			hash := sha512.Sum512(e)
			next = hash[:]
		}
		state = append(state[:0], next...)
		if count >= 64 && int(e[len(e)-1]) <= count-32 {
			return append([]byte(nil), state[:32]...)
		}
	}
}

func byteSum(value []byte) int {
	total := 0
	for _, item := range value {
		total += int(item)
	}
	return total
}

func aesPKCS7(key, iv, plain []byte) []byte {
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	padded := append(append([]byte(nil), plain...), make([]byte, padding)...)
	for i := len(plain); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	return aesCBCWithIV(key, iv, padded)
}

func aesCBCNoPadding(key, iv, plain []byte) []byte {
	return aesCBCWithIV(key, iv, plain)
}

func aesCBCWithIV(key, iv, plain []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		fatal(err)
	}
	if len(plain)%aes.BlockSize != 0 {
		fatal(fmt.Errorf("AES plaintext is not block aligned: %d", len(plain)))
	}
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, plain)
	return out
}

func stream(data []byte) []byte {
	return streamWithDict([]byte{}, data)
}

func streamWithDict(dict, data []byte) []byte {
	out := make([]byte, 0, len(dict)+len(data)+64)
	out = append(out, []byte(fmt.Sprintf("<< %s /Length %d >>\nstream\n", dict, len(data)))...)
	out = append(out, data...)
	out = append(out, []byte("\nendstream")...)
	return out
}

func writePDF(objects map[int][]byte) []byte {
	data := []byte("%PDF-1.7\n")
	offsets := make(map[int]int, len(objects))
	maxObject := 0
	for number := range objects {
		if number > maxObject {
			maxObject = number
		}
	}
	for number := 1; number <= maxObject; number++ {
		object, ok := objects[number]
		if !ok {
			continue
		}
		offsets[number] = len(data)
		data = append(data, []byte(fmt.Sprintf("%d 0 obj\n", number))...)
		data = append(data, object...)
		data = append(data, []byte("\nendobj\n")...)
	}
	xref := len(data)
	data = append(data, []byte(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", maxObject+1))...)
	for number := 1; number <= maxObject; number++ {
		if offset, ok := offsets[number]; ok {
			data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offset))...)
		} else {
			data = append(data, []byte("0000000000 00000 f \n")...)
		}
	}
	data = append(data, []byte(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R /Encrypt 9 0 R /ID [<%s> <%s>] >>\nstartxref\n%d\n%%%%EOF\n", maxObject+1, hex.EncodeToString(fileID), hex.EncodeToString(fileID), xref))...)
	return data
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
