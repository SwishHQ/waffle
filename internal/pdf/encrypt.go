package pdf

// Standard security handler encryption (PDF 32000-1 §7.6.3) with RC4 and a
// 128-bit file encryption key: /Filter /Standard, /V 2, /R 3, /Length 128.
// This covers PDF 1.4/1.5 viewers. Encryption is enabled through
// Options.UserPassword / Options.OwnerPassword; when either is non-empty the
// document gains an /Encrypt dictionary and every string and stream in every
// indirect object is RC4-encrypted with a per-object key. The /Encrypt
// dictionary itself and the trailer /ID entries stay in the clear, as required
// by the spec.

import (
	"crypto/md5"
	"crypto/rc4"
)

// passwordPad is the 32-byte padding string of Algorithm 2 step (a); passwords
// are padded or truncated to exactly 32 bytes with it.
var passwordPad = [32]byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41,
	0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80,
	0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

const (
	// rc4KeyLen is the file encryption key length in bytes (/Length 128 bits).
	rc4KeyLen = 16

	// permAllowAll is the /P value granting every permission: all flag bits
	// set except bits 1–2, which are reserved and shall be 0 (PDF 32000-1
	// Table 22). As a signed 32-bit integer 0xFFFFFFFC is -4.
	permAllowAll int32 = -4
)

// setupEncryption derives the encryption keys from the document passwords,
// adds the /Encrypt dictionary as an indirect object, and installs the
// writer's encryption hook. The file identifier must already be set on the
// writer: its first half feeds the file encryption key (Algorithm 2).
func (d *Document) setupEncryption() {
	p := d.opts.Permissions
	if p == 0 {
		p = permAllowAll
	}
	o := computeO(d.opts.OwnerPassword, d.opts.UserPassword)
	fileKey := computeFileKey(d.opts.UserPassword, o, p, d.w.ID[0])
	u := computeU(fileKey, d.w.ID[0])

	d.w.Encrypt = d.w.Add(Dict{
		Name("Filter"): Name("Standard"),
		Name("V"):      Integer(2),
		Name("R"):      Integer(3),
		Name("Length"): Integer(rc4KeyLen * 8),
		Name("P"):      Integer(p),
		Name("O"):      LiteralString(o),
		Name("U"):      LiteralString(u),
	})
	d.w.encrypt = &encryptor{fileKey: fileKey}
}

// padPassword pads or truncates pw to exactly 32 bytes using the standard
// padding string (Algorithm 2 step a).
func padPassword(pw string) []byte {
	b := make([]byte, 32)
	n := copy(b, pw)
	copy(b[n:], passwordPad[:])
	return b
}

// rc4Apply returns data transformed by RC4 under key using a fresh cipher
// (encryption and decryption are the same operation).
func rc4Apply(key, data []byte) []byte {
	c, err := rc4.NewCipher(key)
	if err != nil {
		// Key lengths here are always 10..16 bytes, well within RC4's 1..256.
		panic(err)
	}
	out := make([]byte, len(data))
	c.XORKeyStream(out, data)
	return out
}

// computeO computes the /O entry (Algorithm 3, revision 3). When the owner
// password is empty the user password is used in its place.
func computeO(ownerPW, userPW string) []byte {
	pw := ownerPW
	if pw == "" {
		pw = userPW
	}
	digest := md5.Sum(padPassword(pw))
	for i := 0; i < 50; i++ { // revision 3: 50 extra MD5 rounds
		digest = md5.Sum(digest[:])
	}
	key := digest[:rc4KeyLen]
	o := rc4Apply(key, padPassword(userPW))
	xorKey := make([]byte, len(key))
	for i := 1; i <= 19; i++ { // revision 3: 19 RC4 rounds with XORed keys
		for j := range key {
			xorKey[j] = key[j] ^ byte(i)
		}
		o = rc4Apply(xorKey, o)
	}
	return o
}

// computeFileKey computes the 128-bit file encryption key (Algorithm 2,
// revision 3): MD5 over the padded user password, the /O value, /P as four
// little-endian bytes, and the first file identifier half, then 50 re-hash
// rounds of the first rc4KeyLen bytes.
func computeFileKey(userPW string, o []byte, p int32, id0 []byte) []byte {
	h := md5.New()
	h.Write(padPassword(userPW))
	h.Write(o)
	h.Write([]byte{byte(p), byte(p >> 8), byte(p >> 16), byte(p >> 24)})
	h.Write(id0)
	digest := h.Sum(nil)
	for i := 0; i < 50; i++ { // revision 3: 50 extra MD5 rounds
		sum := md5.Sum(digest[:rc4KeyLen])
		digest = sum[:]
	}
	return digest[:rc4KeyLen]
}

// computeU computes the /U entry (Algorithm 5, revision 3): MD5 of the padding
// string plus the first file identifier half, RC4-encrypted with the file key
// and then 19 more times with XORed keys, padded to 32 bytes. The final 16
// padding bytes are arbitrary per the spec; zeros keep output deterministic.
func computeU(fileKey, id0 []byte) []byte {
	h := md5.New()
	h.Write(passwordPad[:])
	h.Write(id0)
	u := rc4Apply(fileKey, h.Sum(nil))
	xorKey := make([]byte, len(fileKey))
	for i := 1; i <= 19; i++ {
		for j := range fileKey {
			xorKey[j] = fileKey[j] ^ byte(i)
		}
		u = rc4Apply(xorKey, u)
	}
	return append(u, make([]byte, 16)...)
}

// encryptor rewrites indirect objects at serialization time so that their
// strings and stream data are encrypted. It uses RC4 by default, AES-128-CBC
// (AESV2) when aes is set, or AES-256-CBC (AESV3) when aes+directKey are set —
// the latter encrypts with the file key directly, without per-object keys.
type encryptor struct {
	fileKey   []byte
	aes       bool
	directKey bool
}

// aesSalt is appended to the object-key digest input for AESV2 (PDF 32000-1
// §7.6.2, "the bytes 0x73 0x41 0x6C 0x54 (sAlT)").
var aesSalt = []byte{0x73, 0x41, 0x6C, 0x54}

// objectKey derives the encryption key for one indirect object (Algorithm 1
// step b): MD5 of the file key followed by the low-order 3 bytes of the object
// number and 2 bytes of the generation number (little-endian) — plus the sAlT
// suffix for AES — truncated to min(len(fileKey)+5, 16) bytes.
func (e *encryptor) objectKey(num, gen int) []byte {
	h := md5.New()
	h.Write(e.fileKey)
	h.Write([]byte{byte(num), byte(num >> 8), byte(num >> 16), byte(gen), byte(gen >> 8)})
	if e.aes {
		h.Write(aesSalt)
	}
	n := len(e.fileKey) + 5
	if n > 16 {
		n = 16
	}
	return h.Sum(nil)[:n]
}

// apply encrypts data with the per-object key using the configured cipher.
func (e *encryptor) apply(key, data []byte) []byte {
	if e.aes {
		return aesEncrypt(key, data)
	}
	return rc4Apply(key, data)
}

// transform returns a deep copy of obj in which every string and every
// stream's data has been encrypted for indirect object (num, gen). The stored
// object is never mutated, so serialization stays repeatable.
func (e *encryptor) transform(obj Object, num, gen int) Object {
	key := e.fileKey
	if !e.directKey {
		key = e.objectKey(num, gen) // AESV3 (R6) uses the file key directly
	}
	return e.encryptValue(obj, key)
}

// encryptValue walks an object tree, encrypting LiteralString/HexString values
// and stream data with key. Each string and stream is encrypted independently.
// Non-string scalar objects (names, numbers, booleans, null, references) pass
// through unchanged.
func (e *encryptor) encryptValue(obj Object, key []byte) Object {
	switch v := obj.(type) {
	case LiteralString:
		return LiteralString(e.apply(key, []byte(v)))
	case HexString:
		return HexString(e.apply(key, v))
	case Array:
		out := make(Array, len(v))
		for i, o := range v {
			out[i] = e.encryptValue(o, key)
		}
		return out
	case Dict:
		out := make(Dict, len(v))
		for k, o := range v {
			out[k] = e.encryptValue(o, key)
		}
		return out
	case *Stream:
		return &Stream{
			Dict: e.encryptValue(v.Dict, key).(Dict),
			Data: e.apply(key, v.Data),
		}
	default:
		return obj
	}
}
