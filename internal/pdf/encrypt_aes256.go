package pdf

// AES-256 standard security handler (AESV3): /V 5, /R 6, /Length 256 (PDF 2.0 /
// ISO 32000-2, originally Adobe extension level 3). Unlike the RC4/AESV2
// handlers, the file encryption key is a random 32-byte key wrapped by password-
// derived keys (/UE, /OE); strings and streams are AES-256-CBC encrypted with
// the file key directly. Key derivation uses the revision-6 hash (Algorithm 2.B)
// over SHA-256/384/512. Enabled via Options.EncryptAES256 with a password.

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
)

// setupAES256 generates the random file key, wraps it under the user and owner
// passwords, builds the /Perms block, and installs the AESV3 encryptor.
func (d *Document) setupAES256() {
	p := d.opts.Permissions
	if p == 0 {
		p = permAllowAll
	}
	userPW := d.opts.UserPassword
	ownerPW := d.opts.OwnerPassword
	if ownerPW == "" {
		ownerPW = userPW
	}

	fileKey := randBytes(32)
	uValSalt, uKeySalt := randBytes(8), randBytes(8)
	oValSalt, oKeySalt := randBytes(8), randBytes(8)

	// /U = hash(userPW+valSalt) ++ valSalt ++ keySalt.
	u := append(append(hash2B([]byte(userPW), uValSalt, nil), uValSalt...), uKeySalt...)
	// /UE = AES-256-CBC-noPad(fileKey) under hash(userPW+keySalt), IV=0.
	ue := aesNoPad(hash2B([]byte(userPW), uKeySalt, nil), fileKey)

	// /O = hash(ownerPW+valSalt+U) ++ valSalt ++ keySalt; /OE likewise keyed by U.
	o := append(append(hash2B([]byte(ownerPW), oValSalt, u), oValSalt...), oKeySalt...)
	oe := aesNoPad(hash2B([]byte(ownerPW), oKeySalt, u), fileKey)

	perms := encodePerms(p, fileKey, true)

	stdCF := Dict{
		Name("CFM"):       Name("AESV3"),
		Name("AuthEvent"): Name("DocOpen"),
		Name("Length"):    Integer(32),
	}
	d.w.Encrypt = d.w.Add(Dict{
		Name("Filter"):          Name("Standard"),
		Name("V"):               Integer(5),
		Name("R"):               Integer(6),
		Name("Length"):          Integer(256),
		Name("P"):               Integer(p),
		Name("O"):               LiteralString(o),
		Name("U"):               LiteralString(u),
		Name("OE"):              LiteralString(oe),
		Name("UE"):              LiteralString(ue),
		Name("Perms"):           LiteralString(perms),
		Name("EncryptMetadata"): Boolean(true),
		Name("CF"):              Dict{Name("StdCF"): stdCF},
		Name("StmF"):            Name("StdCF"),
		Name("StrF"):            Name("StdCF"),
	})
	d.w.encrypt = &encryptor{fileKey: fileKey, aes: true, directKey: true}
}

// hash2B implements the revision-6 password hash (ISO 32000-2 Algorithm 2.B).
// udata is the 48-byte /U value when hashing an owner password, else empty.
func hash2B(pw, salt, udata []byte) []byte {
	sum := sha256.Sum256(concat(pw, salt, udata))
	k := sum[:]
	for round := 1; ; round++ {
		block := concat(pw, k, udata)
		k1 := bytes.Repeat(block, 64)

		blk, _ := aes.NewCipher(k[:16])
		e := make([]byte, len(k1))
		cipher.NewCBCEncrypter(blk, k[16:32]).CryptBlocks(e, k1)

		mod := 0
		for _, b := range e[:16] {
			mod += int(b)
		}
		switch mod % 3 {
		case 0:
			s := sha256.Sum256(e)
			k = s[:]
		case 1:
			s := sha512.Sum384(e)
			k = s[:]
		case 2:
			s := sha512.Sum512(e)
			k = s[:]
		}
		if round >= 64 && int(e[len(e)-1]) <= round-32 {
			break
		}
	}
	return k[:32]
}

// aesNoPad encrypts exactly len(data) bytes (a multiple of 16) with AES-CBC and
// a zero IV, no padding — used to wrap the file key into /UE and /OE.
func aesNoPad(key, data []byte) []byte {
	blk, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(blk, make([]byte, aes.BlockSize)).CryptBlocks(out, data)
	return out
}

// encodePerms builds the 16-byte /Perms block and encrypts it with the file key
// (AES-256, single ECB block, no IV).
func encodePerms(p int32, fileKey []byte, encryptMetadata bool) []byte {
	buf := make([]byte, 16)
	buf[0] = byte(p)
	buf[1] = byte(p >> 8)
	buf[2] = byte(p >> 16)
	buf[3] = byte(p >> 24)
	buf[4], buf[5], buf[6], buf[7] = 0xFF, 0xFF, 0xFF, 0xFF
	if encryptMetadata {
		buf[8] = 'T'
	} else {
		buf[8] = 'F'
	}
	buf[9], buf[10], buf[11] = 'a', 'd', 'b'
	copy(buf[12:], randBytes(4))
	blk, _ := aes.NewCipher(fileKey)
	out := make([]byte, 16)
	blk.Encrypt(out, buf) // single-block ECB
	return out
}

func concat(parts ...[]byte) []byte {
	var b []byte
	for _, p := range parts {
		b = append(b, p...)
	}
	return b
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
