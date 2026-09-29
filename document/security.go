package document

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rc4"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/text/secure/precis"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/bidi"
	"golang.org/x/text/unicode/norm"
)

var (
	ErrPasswordRequired      = fmt.Errorf("playa: encrypted PDF password is required")
	ErrInvalidPassword       = fmt.Errorf("playa: encrypted PDF password is invalid")
	ErrUnsupportedEncryption = fmt.Errorf("playa: unsupported PDF encryption")
)

var passwordPadding = []byte{
	0x28, 0xbf, 0x4e, 0x5e, 0x4e, 0x75, 0x8a, 0x41,
	0x64, 0x00, 0x4e, 0x56, 0xff, 0xfa, 0x01, 0x08,
	0x2e, 0x2e, 0x00, 0xb6, 0xd0, 0x68, 0x3e, 0x80,
	0x2f, 0x0c, 0xa9, 0xfe, 0x64, 0x53, 0x69, 0x7a,
}

func (d *Document) configureSecurity(password string) error {
	return d.configureSecurityWithPresence(password, password != "")
}

func (d *Document) configureSecurityWithPresence(password string, provided bool) error {
	if !provided {
		return ErrPasswordRequired
	}
	encValue, _ := d.resolveIndirectChain(d.trailer[Name("Encrypt")])
	enc, ok := encValue.(Dict)
	if !ok {
		return ErrUnsupportedEncryption
	}
	filterValue, _ := d.resolveIndirectChain(enc[Name("Filter")])
	filter, _ := filterValue.(Name)
	versionValue, _ := d.resolveIndirectChain(enc[Name("V")])
	version, _ := IntValue(versionValue)
	revisionValue, _ := d.resolveIndirectChain(enc[Name("R")])
	revision, _ := IntValue(revisionValue)
	if filter != Name("Standard") || (version == 1 && revision != 2) || (version == 2 && revision != 3) || (version == 4 && revision != 4) || (version == 5 && revision != 5 && revision != 6) || (version != 1 && version != 2 && version != 4 && version != 5) {
		return ErrUnsupportedEncryption
	}
	if version == 5 {
		d.encryptionMethod = "AESV3"
		d.streamEncryptionMethod = "AESV3"
		d.stringEncryptionMethod = "AESV3"
		return d.configureSecurityRevisionFivePlus(password, enc, revision)
	}
	keyLength := 5
	if revision == 3 {
		length := 40
		valueObject, _ := d.resolveIndirectChain(enc[Name("Length")])
		if value, ok := IntValue(valueObject); ok {
			length = value
		}
		if !validLegacyKeyLength(length) {
			return ErrUnsupportedEncryption
		}
		keyLength = length / 8
	}
	if revision == 4 {
		length := 40
		valueObject, _ := d.resolveIndirectChain(enc[Name("Length")])
		if value, ok := IntValue(valueObject); ok {
			length = value
		}
		if !validLegacyKeyLength(length) {
			return ErrUnsupportedEncryption
		}
		streamMethod, stringMethod, ok := standardR4CryptFilters(d, enc)
		if !ok {
			return ErrUnsupportedEncryption
		}
		if (streamMethod == "AESV2" || stringMethod == "AESV2") && length != 128 {
			return ErrUnsupportedEncryption
		}
		d.streamEncryptionMethod = streamMethod
		d.stringEncryptionMethod = stringMethod
		if streamMethod != "Identity" {
			d.encryptionMethod = streamMethod
		} else {
			d.encryptionMethod = stringMethod
		}
		keyLength = length / 8
	}
	ownerValue, _ := d.resolveIndirectChain(enc[Name("O")])
	userValue, _ := d.resolveIndirectChain(enc[Name("U")])
	o, okO := ownerValue.(String)
	u, okU := userValue.(String)
	id := trailerFileID(d)
	permissions, okP := securityPermissions(d, enc)
	if !okO || !okU || len(o) == 0 || len(u) < 32 || len(id) == 0 || !okP {
		return ErrUnsupportedEncryption
	}
	metadata, okMetadata := encryptionMetadataFlag(d, enc)
	if !okMetadata {
		return ErrUnsupportedEncryption
	}
	d.metadataExcluded = !metadata
	d.recordMetadataRef()
	key := encryptionKeyRevisionWithMetadata([]byte(password), []byte(o), permissions, id, keyLength, revision, metadata)
	check, err := userEntryRevision(key, id, revision)
	if err != nil || subtle.ConstantTimeCompare(check, u[:32]) != 1 {
		return ErrInvalidPassword
	}
	d.encrypted = true
	d.printable, d.modifiable, d.extractable = permissionsFor(permissions)
	d.encryptionKey = key
	d.encryptionRevision = revision
	return nil
}

func (d *Document) configureSecurityRevisionFivePlus(password string, enc Dict, revision int) error {
	// AESV3 uses a fixed 256-bit key. PDF 2.0 explicitly says that Length is
	// ignored for this crypt method, so both an omitted entry and a legacy
	// producer's explicit 256-bit value are valid.
	ownerValue, _ := d.resolveIndirectChain(enc[Name("O")])
	userValue, _ := d.resolveIndirectChain(enc[Name("U")])
	ownerEntryValue, _ := d.resolveIndirectChain(enc[Name("OE")])
	userEntryValue, _ := d.resolveIndirectChain(enc[Name("UE")])
	o, okO := ownerValue.(String)
	u, okU := userValue.(String)
	oe, okOE := ownerEntryValue.(String)
	ue, okUE := userEntryValue.(String)
	_, okP := securityPermissions(d, enc)
	if !okO || !okU || !okOE || !okUE || !okP || len(o) != 48 || len(u) != 48 || len(oe) != 32 || len(ue) != 32 {
		return ErrUnsupportedEncryption
	}
	if !standardAESV3(d, enc) {
		return ErrUnsupportedEncryption
	}
	metadata, okMetadata := encryptionMetadataFlag(d, enc)
	if !okMetadata {
		return ErrUnsupportedEncryption
	}
	passwordBytes := []byte(password)
	if revision == 6 {
		var err error
		passwordBytes, err = revisionSixPassword(password)
		if err != nil {
			return ErrInvalidPassword
		}
	}
	passwordBytes = truncatePassword(passwordBytes)
	fileKey, ok := revisionFiveFileKey(passwordBytes, []byte(o), []byte(u), []byte(oe), []byte(ue), revision)
	if !ok {
		return ErrInvalidPassword
	}
	if !revisionFivePermissions(fileKey, enc, d, metadata) {
		return ErrInvalidPassword
	}
	d.encrypted = true
	permissions, _ := securityPermissions(d, enc)
	d.printable, d.modifiable, d.extractable = permissionsFor(permissions)
	d.encryptionKey = fileKey
	d.encryptionRevision = revision
	d.metadataExcluded = !metadata
	d.recordMetadataRef()
	return nil
}

// revisionSixPassword applies the PDF 2.0 password preprocessing rule before
// the R6 hash. PRECIS supplies the Unicode validity checks; the explicit
// mappings and bidi validation below implement the PDF/RFC 4013 profile.
func revisionSixPassword(password string) ([]byte, error) {
	profile := precis.NewFreeform(
		precis.Norm(norm.NFKC),
	)
	var mapped strings.Builder
	for _, r := range password {
		if legacy, ok := unicode32NormalizationCorrection(r); ok {
			mapped.WriteRune(legacy)
			continue
		}
		switch {
		case r == '\u00a0' || r == '\u1680' || (r >= '\u2000' && r <= '\u200a') || r == '\u202f' || r == '\u205f' || r == '\u3000':
			mapped.WriteRune(' ')
		case r == '\u00ad' || r == '\u034f' || r == '\u1806' || (r >= '\u180b' && r <= '\u180d') || (r >= '\u200b' && r <= '\u200d') || r == '\u2060' || (r >= '\ufe00' && r <= '\ufe0f') || r == '\ufeff':
		case r >= '\u0000' && r <= '\u001f' || r == '\u007f' || r >= '\u0080' && r <= '\u009f':
			mapped.WriteRune(r)
		default:
			mapped.WriteRune(r)
		}
	}
	prepared, _, err := transform.String(profile.NewTransformer(), mapped.String())
	if err != nil {
		return nil, err
	}
	if !revisionSixBidiValid(prepared) {
		return nil, fmt.Errorf("playa: invalid R6 password bidi sequence")
	}
	return []byte(prepared), nil
}

// unicode32NormalizationCorrection restores the five canonical decompositions
// corrected in Unicode 4.0. PDF revision 6 password preprocessing is pinned to
// Unicode 3.2 and therefore requires their earlier mappings.
func unicode32NormalizationCorrection(r rune) (rune, bool) {
	switch r {
	case '\U0002f868':
		return '\U0002136a', true
	case '\U0002f874':
		return '\u5f33', true
	case '\U0002f91f':
		return '\u43ab', true
	case '\U0002f95f':
		return '\u7aae', true
	case '\U0002f9bf':
		return '\u4d57', true
	default:
		return r, false
	}
}

func revisionSixBidiValid(password string) bool {
	var first, last bidi.Class
	hasFirst := false
	hasRandAL, hasL := false, false
	for _, r := range password {
		class, _ := bidi.LookupRune(r)
		c := class.Class()
		if !hasFirst {
			first = c
			hasFirst = true
		}
		last = c
		switch c {
		case bidi.R, bidi.AL:
			hasRandAL = true
		case bidi.L:
			hasL = true
		}
	}
	if !hasRandAL {
		return true
	}
	return !hasL && (first == bidi.R || first == bidi.AL) && (last == bidi.R || last == bidi.AL)
}

// encryptionMetadataFlag applies the PDF default while rejecting malformed
// explicit values. EncryptMetadata is optional, but when present it must be a
// boolean; treating a number or name as the default can decrypt the wrong
// object bytes and hide a malformed encryption dictionary.
func encryptionMetadataFlag(d *Document, enc Dict) (bool, bool) {
	raw, present := enc[Name("EncryptMetadata")]
	if !present {
		return true, true
	}
	resolved, _ := d.resolveIndirectChain(raw)
	value, ok := resolved.(Bool)
	return bool(value), ok
}

func permissionsFor(value int) (printable, modifiable, extractable bool) {
	permissions := uint32(int32(value))
	return permissions&4 != 0, permissions&8 != 0, permissions&16 != 0
}

func securityPermissions(d *Document, enc Dict) (int, bool) {
	resolved, _ := d.resolveIndirectChain(enc[Name("P")])
	value, ok := finiteNumberValue(resolved)
	if !ok || value != float64(int64(value)) || value < -2147483648 || value > 2147483647 {
		return 0, false
	}
	return int(value), true
}

func validLegacyKeyLength(length int) bool {
	return length >= 40 && length <= 128 && length%8 == 0
}

func (d *Document) recordMetadataRef() {
	if d.metadataRefs == nil {
		d.metadataRefs = map[Ref]bool{}
	}
	root, ok := d.trailer[Name("Root")]
	if !ok {
		return
	}
	catalogValue, ok := d.resolveIndirectChain(root)
	if !ok {
		return
	}
	catalog, ok := catalogValue.(Dict)
	if !ok {
		return
	}
	value := catalog[Name("Metadata")]
	seen := map[Ref]bool{}
	for {
		ref, ok := value.(Ref)
		if !ok || seen[ref] {
			return
		}
		seen[ref] = true
		d.metadataRefs[ref] = true
		resolved, err := d.resolveRefRaw(ref)
		if err != nil {
			return
		}
		value = resolved
	}
}

// resolveIndirectChain follows a bounded-by-cycle indirect reference chain
// without copying the resolved object. Security metadata can legally be
// wrapped in multiple indirect objects, and malformed files may contain a
// reference cycle; callers need both behaviors to be explicit.
func (d *Document) resolveIndirectChain(value Object) (Object, bool) {
	seen := map[Ref]bool{}
	for {
		ref, ok := value.(Ref)
		if !ok {
			return value, true
		}
		if seen[ref] {
			return nil, false
		}
		seen[ref] = true
		resolved, err := d.resolveRefRaw(ref)
		if err != nil {
			return nil, false
		}
		value = resolved
	}
}

// resolveContentIndirectChain follows references used by a page content root
// or entry. Playa skips genuinely missing references in /Contents, but errors
// while parsing a referenced object (including an object stream) remain
// observable. The general resolver intentionally retains its historical bool
// contract, so content traversal uses this error-preserving variant.
func (d *Document) resolveContentIndirectChain(value Object) (Object, bool, error) {
	seen := map[int]bool{}
	for {
		ref, ok := value.(Ref)
		if !ok {
			return value, true, nil
		}
		if seen[ref.Object] {
			return nil, false, nil
		}
		seen[ref.Object] = true
		_, resolved, err := d.lookupRawWithError(ref.Object)
		if err != nil && err != ErrObjectNotFound && contentReferenceMayRecover(err) {
			recovered, ok, recoverErr := d.recoverContentObjectNumber(ref.Object)
			if recoverErr != nil {
				return nil, false, recoverErr
			}
			if ok {
				value = recovered
				continue
			}
		}
		if contentReferenceErrorIsMissing(err) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		value = resolved
	}
}

func contentReferenceErrorIsMissing(err error) bool {
	if err == nil || err == ErrObjectNotFound {
		return err != nil
	}
	message := err.Error()
	return strings.Contains(message, "not found") ||
		strings.Contains(message, "invalid offset") ||
		strings.Contains(message, "body not found") ||
		strings.Contains(message, "header not found") ||
		strings.Contains(message, "header does not match xref")
}

func contentReferenceMayRecover(err error) bool {
	if err == nil {
		return false
	}
	for cause := err; cause != nil; cause = errors.Unwrap(cause) {
		if parseErr, ok := cause.(*ParseError); ok {
			return !strings.HasPrefix(parseErr.Operation(), "filter ")
		}
	}
	message := err.Error()
	return strings.Contains(message, "invalid offset") ||
		strings.Contains(message, "body not found") ||
		strings.Contains(message, "header not found") ||
		strings.Contains(message, "header does not match xref")
}

func standardAESV3(d *Document, enc Dict) bool {
	cfValue, _ := d.resolveIndirectChain(enc[Name("CF")])
	cf, ok := cfValue.(Dict)
	if !ok {
		return false
	}
	stdValue, _ := d.resolveIndirectChain(cf[Name("StdCF")])
	std, ok := stdValue.(Dict)
	if !ok {
		return false
	}
	methodValue, _ := d.resolveIndirectChain(std[Name("CFM")])
	method, _ := methodValue.(Name)
	stream, ok := optionalEncryptionName(d, enc, Name("StmF"), Name("StdCF"))
	if !ok {
		return false
	}
	stringFilter, ok := optionalEncryptionName(d, enc, Name("StrF"), Name("StdCF"))
	if !ok {
		return false
	}
	return method == Name("AESV3") && stream == Name("StdCF") && stringFilter == Name("StdCF")
}

func optionalEncryptionName(d *Document, dict Dict, key, defaultValue Name) (Name, bool) {
	raw, present := dict[key]
	if !present {
		return defaultValue, true
	}
	resolved, _ := d.resolveIndirectChain(raw)
	value, ok := resolved.(Name)
	return value, ok
}

func revisionFiveFileKey(password, owner, user, oe, ue []byte, revision int) ([]byte, bool) {
	ownerHash := revisionFiveHash(password, owner[40:48], user, revision)
	ownerValidation := revisionFiveHash(password, owner[32:40], user, revision)
	userValidation := revisionFiveHash(password, user[32:40], nil, revision)
	ownerValid := subtle.ConstantTimeCompare(ownerValidation, owner[:32]) == 1
	userValid := subtle.ConstantTimeCompare(userValidation, user[:32]) == 1
	if !ownerValid && !userValid {
		return nil, false
	}
	var key []byte
	if userValid {
		key = revisionFiveHash(password, user[40:48], nil, revision)
	} else {
		key = ownerHash[:]
	}
	ciphertext := ue
	if ownerValid && !userValid {
		ciphertext = oe
	}
	plain, err := aesCBCNoPadding(key, ciphertext)
	if err != nil || len(plain) != 32 {
		return nil, false
	}
	return plain, true
}

func revisionFiveHash(password, salt, user []byte, revision int) []byte {
	input := make([]byte, 0, len(password)+len(salt)+len(user))
	input = append(input, password...)
	input = append(input, salt...)
	input = append(input, user...)
	if revision == 5 {
		hash := sha256.Sum256(input)
		return append([]byte(nil), hash[:]...)
	}
	return revisionSixHash(password, salt, user)
}

func revisionSixHash(password, salt, user []byte) []byte {
	input := make([]byte, 0, len(password)+len(salt)+len(user))
	input = append(input, password...)
	input = append(input, salt...)
	input = append(input, user...)
	initial := sha256.Sum256(input)
	k := append([]byte(nil), initial[:]...)
	for count := 1; ; count++ {
		// R6 repeats password + K + user data, not the original hash input.
		blockInput := make([]byte, 0, len(password)+len(k)+len(user))
		blockInput = append(blockInput, password...)
		blockInput = append(blockInput, k...)
		blockInput = append(blockInput, user...)
		e, err := aesCBCEncrypt(k[:16], k[16:32], bytesRepeat(blockInput, 64))
		if err != nil {
			return nil
		}
		switch byteSum(e[:16]) % 3 {
		case 0:
			hash := sha256.Sum256(e)
			k = append(k[:0], hash[:]...)
		case 1:
			hash := sha512.Sum384(e)
			k = append(k[:0], hash[:]...)
		case 2:
			hash := sha512.Sum512(e)
			k = append(k[:0], hash[:]...)
		}
		if count >= 64 && int(e[len(e)-1]) <= count-32 {
			return append([]byte(nil), k[:32]...)
		}
	}
}

func byteSum(value []byte) int {
	total := 0
	for _, b := range value {
		total += int(b)
	}
	return total
}

func bytesRepeat(value []byte, count int) []byte {
	out := make([]byte, 0, len(value)*count)
	for i := 0; i < count; i++ {
		out = append(out, value...)
	}
	return out
}

func revisionFivePermissions(fileKey []byte, enc Dict, d *Document, metadata bool) bool {
	permsValue, _ := d.resolveIndirectChain(enc[Name("Perms")])
	perms, ok := permsValue.(String)
	if !ok || len(perms) != 16 {
		return false
	}
	plain, err := aesCBCNoPadding(fileKey, []byte(perms))
	if err != nil || len(plain) != 16 {
		return false
	}
	permissions, ok := securityPermissions(d, enc)
	if !ok || binary.LittleEndian.Uint32(plain[:4]) != uint32(int32(permissions)) {
		return false
	}
	for _, value := range plain[4:8] {
		if value != 0xff {
			return false
		}
	}
	wantMetadata := byte('T')
	if !metadata {
		wantMetadata = 'F'
	}
	if plain[8] != wantMetadata || string(plain[9:12]) != "adb" {
		return false
	}
	return true
}

func truncatePassword(password []byte) []byte {
	if len(password) > 127 {
		return password[:127]
	}
	return password
}

func aesCBCNoPadding(key, data []byte) ([]byte, error) {
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("playa: invalid AES block length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(out, data)
	return out, nil
}

func aesCBCEncrypt(key, iv, data []byte) ([]byte, error) {
	if len(data) == 0 || len(data)%aes.BlockSize != 0 || len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("playa: invalid AES block length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, data)
	return out, nil
}

func standardR4CryptFilters(d *Document, enc Dict) (string, string, bool) {
	cfValue, _ := d.resolveIndirectChain(enc[Name("CF")])
	cf, _ := cfValue.(Dict)
	stream, streamOK := optionalEncryptionName(d, enc, Name("StmF"), Name("Identity"))
	if !streamOK {
		return "", "", false
	}
	stringFilter, stringOK := optionalEncryptionName(d, enc, Name("StrF"), Name("Identity"))
	if !stringOK {
		return "", "", false
	}
	method := func(filter Name) (string, bool) {
		if filter == Name("Identity") {
			return "Identity", true
		}
		entryValue, _ := d.resolveIndirectChain(cf[filter])
		entry, ok := entryValue.(Dict)
		if !ok {
			return "", false
		}
		valueObject, _ := d.resolveIndirectChain(entry[Name("CFM")])
		value, _ := valueObject.(Name)
		name := string(value)
		if name == "None" {
			return "Identity", true
		}
		return name, name == "AESV2" || name == "V2"
	}
	streamMethod, streamMethodOK := method(stream)
	stringMethod, stringMethodOK := method(stringFilter)
	return streamMethod, stringMethod, streamMethodOK && stringMethodOK
}

func trailerFileID(d *Document) []byte {
	idsValue, _ := d.resolveIndirectChain(d.trailer[Name("ID")])
	ids, _ := idsValue.(Array)
	if len(ids) == 0 {
		return nil
	}
	idValue, _ := d.resolveIndirectChain(ids[0])
	id, _ := idValue.(String)
	return []byte(id)
}

func encryptionKey(password, owner []byte, permissions int, id []byte) []byte {
	return encryptionKeyRevision(password, owner, permissions, id, 5, 2)
}

func encryptionKeyRevision(password, owner []byte, permissions int, id []byte, length, revision int) []byte {
	return encryptionKeyRevisionWithMetadata(password, owner, permissions, id, length, revision, true)
}

func encryptionKeyRevisionWithMetadata(password, owner []byte, permissions int, id []byte, length, revision int, metadata bool) []byte {
	padded := padPassword(password)
	h := md5.New()
	h.Write(padded)
	h.Write(owner)
	var p [4]byte
	binary.LittleEndian.PutUint32(p[:], uint32(int32(permissions)))
	h.Write(p[:])
	h.Write(id)
	if revision >= 4 && !metadata {
		h.Write([]byte{0xff, 0xff, 0xff, 0xff})
	}
	if revision >= 3 {
		for i := 1; i < 50; i++ {
			sum := h.Sum(nil)
			h.Reset()
			h.Write(sum[:length])
		}
	}
	sum := h.Sum(nil)
	return append([]byte(nil), sum[:length]...)
}

func userEntryRevision(key, id []byte, revision int) ([]byte, error) {
	if revision == 2 {
		return rc4Bytes(key, passwordPadding)
	}
	h := md5.New()
	h.Write(passwordPadding)
	h.Write(id)
	value := h.Sum(nil)
	value, err := rc4Bytes(key, value)
	if err != nil {
		return nil, err
	}
	for i := 1; i <= 19; i++ {
		xorKey := append([]byte(nil), key...)
		for j := range xorKey {
			xorKey[j] ^= byte(i)
		}
		value, err = rc4Bytes(xorKey, value)
		if err != nil {
			return nil, err
		}
	}
	return append(value, make([]byte, 16)...), nil
}

func padPassword(password []byte) []byte {
	out := make([]byte, 32)
	n := len(password)
	if n > len(out) {
		n = len(out)
	}
	copy(out, password[:n])
	copy(out[n:], passwordPadding[:len(out)-n])
	return out
}

func (d *Document) objectKey(ref Ref) []byte {
	return d.objectKeyForMethod(ref, d.encryptionMethod)
}

func (d *Document) objectKeyForMethod(ref Ref, method string) []byte {
	if method == "AESV3" {
		return d.encryptionKey
	}
	if d.encryptionRevision >= 5 {
		h := sha256.New()
		h.Write(d.encryptionKey)
		var b [5]byte
		b[0] = byte(ref.Object)
		b[1] = byte(ref.Object >> 8)
		b[2] = byte(ref.Object >> 16)
		b[3] = byte(ref.Generation)
		b[4] = byte(ref.Generation >> 8)
		h.Write(b[:])
		h.Write([]byte("sAlT"))
		return h.Sum(nil)
	}
	h := md5.New()
	h.Write(d.encryptionKey)
	var b [5]byte
	b[0] = byte(ref.Object)
	b[1] = byte(ref.Object >> 8)
	b[2] = byte(ref.Object >> 16)
	b[3] = byte(ref.Generation)
	b[4] = byte(ref.Generation >> 8)
	h.Write(b[:])
	if method != "V2" && method != "Identity" && d.encryptionRevision >= 4 {
		h.Write([]byte("sAlT"))
	}
	sum := h.Sum(nil)
	n := len(d.encryptionKey) + 5
	if n > 16 {
		n = 16
	}
	return sum[:n]
}

func rc4Bytes(key, data []byte) ([]byte, error) {
	cipher, err := rc4.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	cipher.XORKeyStream(out, data)
	return out, nil
}

func (d *Document) decryptObject(ref Ref, object Object) Object {
	if !d.encrypted {
		return object
	}
	decrypted, err := d.decryptObjectWithMetadataContextWithError(ref, object, d.isCatalogRef(ref))
	if err != nil {
		return object
	}
	return decrypted
}

func (d *Document) isCatalogRef(ref Ref) bool {
	root, ok := d.trailer[Name("Root")].(Ref)
	if !ok {
		return false
	}
	seen := map[Ref]bool{}
	for {
		if root == ref {
			return true
		}
		if seen[root] {
			return false
		}
		seen[root] = true
		value, err := d.resolveRefRaw(root)
		if err != nil {
			return false
		}
		next, ok := value.(Ref)
		if !ok {
			return false
		}
		root = next
	}
}

func (d *Document) isEncryptionObject(ref Ref, object Object) bool {
	if raw, ok := d.trailer[Name("Encrypt")].(Ref); ok && raw == ref {
		return true
	}
	value, ok := object.(Dict)
	if !ok {
		return false
	}
	filter, filterOK := value[Name("Filter")].(Name)
	_, ownerOK := value[Name("O")]
	_, userOK := value[Name("U")]
	return filterOK && filter == Name("Standard") && ownerOK && userOK
}

func (d *Document) decryptObjectWithMetadataContextWithError(ref Ref, object Object, catalog bool) (Object, error) {
	switch value := object.(type) {
	case String:
		data, err := d.decryptBytesWithError(ref, []byte(value), d.stringEncryptionMethod)
		if err != nil {
			return nil, fmt.Errorf("playa: decrypt string object %s: %w", ref.String(), err)
		}
		return String(data), nil
	case Array:
		out := make(Array, len(value))
		for i, item := range value {
			decrypted, err := d.decryptObjectWithMetadataContextWithError(ref, item, false)
			if err != nil {
				return nil, err
			}
			out[i] = decrypted
		}
		return out, nil
	case Dict:
		out := Dict{}
		for key, item := range value {
			if d.metadataExcluded && catalog && key == Name("Metadata") {
				if metadata, ok := item.(Stream); ok {
					metadataDict := metadata.DictBorrowed()
					// EncryptMetadata excludes only the metadata stream bytes;
					// its dictionary is still an ordinary encrypted object.
					decrypted, err := d.decryptObjectWithMetadataContextWithError(ref, metadataDict, false)
					if err != nil {
						return nil, err
					}
					out[key] = newStream(decrypted.(Dict), metadata.DataBorrowed())
					continue
				}
			}
			decrypted, err := d.decryptObjectWithMetadataContextWithError(ref, item, false)
			if err != nil {
				return nil, err
			}
			out[key] = decrypted
		}
		return out, nil
	case Stream:
		valueDict := value.DictBorrowed()
		if d.metadataExcluded && (d.metadataRefs[ref] || d.isMetadataStream(value)) {
			// EncryptMetadata excludes the stream bytes, not the stream
			// dictionary. Strings in the dictionary still use the ordinary
			// object key and must be decrypted.
			decrypted, err := d.decryptObjectWithMetadataContextWithError(ref, valueDict, false)
			if err != nil {
				return nil, err
			}
			return newStream(decrypted.(Dict), value.DataBorrowed()), nil
		}
		decryptedDict, err := d.decryptObjectWithMetadataContextWithError(ref, valueDict, false)
		if err != nil {
			return nil, err
		}
		data, err := d.decryptBytesWithError(ref, value.DataBorrowed(), d.streamEncryptionMethod)
		if err != nil {
			return nil, fmt.Errorf("playa: decrypt stream object %s: %w", ref.String(), err)
		}
		return newStream(decryptedDict.(Dict), data), nil
	default:
		return object, nil
	}
}

func (d *Document) decryptBytesWithError(ref Ref, data []byte, method string) ([]byte, error) {
	if method == "" {
		method = d.encryptionMethod
	}
	if method == "Identity" {
		return data, nil
	}
	var out []byte
	var err error
	if method != "V2" && d.encryptionRevision >= 4 {
		out, err = aesBytes(d.objectKeyForMethod(ref, method), data)
	} else {
		out, err = rc4Bytes(d.objectKeyForMethod(ref, method), data)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (d *Document) isMetadataStream(stream Stream) bool {
	streamDict := stream.DictBorrowed()
	typValue, _ := d.resolveIndirectChain(streamDict[Name("Type")])
	subtypeValue, _ := d.resolveIndirectChain(streamDict[Name("Subtype")])
	typ, _ := typValue.(Name)
	subtype, _ := subtypeValue.(Name)
	return typ == Name("Metadata") && subtype == Name("XML")
}

func aesBytes(key, data []byte) ([]byte, error) {
	if len(data) < aes.BlockSize || (len(data)-aes.BlockSize)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("playa: invalid AES stream length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data)-aes.BlockSize)
	cipher.NewCBCDecrypter(block, data[:aes.BlockSize]).CryptBlocks(out, data[aes.BlockSize:])
	if len(out) == 0 {
		return nil, fmt.Errorf("playa: empty AES plaintext")
	}
	padding := int(out[len(out)-1])
	if padding == 0 || padding > aes.BlockSize || padding > len(out) {
		return nil, fmt.Errorf("playa: invalid AES padding")
	}
	for _, value := range out[len(out)-padding:] {
		if int(value) != padding {
			return nil, fmt.Errorf("playa: invalid AES padding")
		}
	}
	return out[:len(out)-padding], nil
}
