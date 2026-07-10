package pdf

// AES-128 standard security handler (AESV2): /V 4, /R 4, /Length 128, with a
// /CF crypt filter using /CFM /AESV2. Key derivation (/O, /U, file key) is
// identical to the RC4 revision-3 handler; only the per-object cipher differs.
// Strings and streams are AES-128-CBC encrypted with a random 16-byte IV
// prepended, PKCS#7 padded. Enabled via Options.EncryptAES with a password.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
)

// setupAES derives the encryption keys (same as revision 3), adds the AESV2
// /Encrypt dictionary, and installs an AES encryptor.
func (d *Document) setupAES() {
	p := d.opts.Permissions
	if p == 0 {
		p = permAllowAll
	}
	o := computeO(d.opts.OwnerPassword, d.opts.UserPassword)
	fileKey := computeFileKey(d.opts.UserPassword, o, p, d.w.ID[0])
	u := computeU(fileKey, d.w.ID[0])

	stdCF := Dict{
		Name("CFM"):       Name("AESV2"),
		Name("AuthEvent"): Name("DocOpen"),
		Name("Length"):    Integer(rc4KeyLen * 8), // key length in bits (128)
	}
	d.w.Encrypt = d.w.Add(Dict{
		Name("Filter"): Name("Standard"),
		Name("V"):      Integer(4),
		Name("R"):      Integer(4),
		Name("Length"): Integer(rc4KeyLen * 8),
		Name("P"):      Integer(p),
		Name("O"):      LiteralString(o),
		Name("U"):      LiteralString(u),
		Name("CF"):     Dict{Name("StdCF"): stdCF},
		Name("StmF"):   Name("StdCF"),
		Name("StrF"):   Name("StdCF"),
	})
	d.w.encrypt = &encryptor{fileKey: fileKey, aes: true}
}

// aesEncrypt encrypts data with AES-128-CBC under key, prepending a random IV
// and applying PKCS#7 padding (PDF 32000-1 §7.6.2).
func aesEncrypt(key, data []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err) // key is always 16 bytes here
	}
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		panic(err)
	}
	padded := pkcs7Pad(data, aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return append(iv, out...)
}

// pkcs7Pad appends 1..blockSize padding bytes so len is a multiple of blockSize;
// a full extra block is added when the input is already aligned.
func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+pad)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}
