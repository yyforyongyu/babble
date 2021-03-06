package dh

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/btcsuite/btcd/btcec"
)

// dhlenBitcoin defines the DHLEN for secp256k1.
const dhlenBitcoin = 32

// lenBitcoin defines the public key length(compressed).
const lenBitcoinPubKey = 33

// publicKeyBitcoin implements the PublicKey interface.
type publicKeyBitcoin struct {
	// btcecPub mounts a btcec.PublicKey
	*btcec.PublicKey
}

// Bytes turns the underlying bytes array into a slice.
func (pk *publicKeyBitcoin) Bytes() []byte {
	return pk.SerializeCompressed()
}

// Hex returns the public key in hexstring.
func (pk *publicKeyBitcoin) Hex() string {
	return hex.EncodeToString(pk.Bytes())
}

// loadBytes takes the input data and copies it into a dhlenBitcoin-byte array.
func (pk *publicKeyBitcoin) loadBytes(data []byte) error {
	if len(data) != lenBitcoinPubKey {
		return errMismatchedKey("public", lenBitcoinPubKey, len(data))
	}

	pub, err := btcec.ParsePubKey(data, btcec.S256())
	if err != nil {
		return err
	}
	// move the pointer to point a new struct
	*pk = publicKeyBitcoin{pub}
	return nil
}

// privateKeyBitcoin implements the PrivateKey interface.
type privateKeyBitcoin struct {
	// btcecPriv mounts a btcec.PrivateKey
	*btcec.PrivateKey
	pub *publicKeyBitcoin
}

// Bytes turns the underlying bytes array into a slice.
func (pk *privateKeyBitcoin) Bytes() []byte {
	return pk.Serialize()
}

// DH performs a Diffie-Hellman calculation between the private key in the
// key pair and the public key.
func (pk *privateKeyBitcoin) DH(pub []byte) ([]byte, error) {
	var pubKey publicKeyBitcoin
	// validate public key
	if err := pubKey.loadBytes(pub); err != nil {
		return nil, err
	}

	var shared [dhlenBitcoin]byte

	newPoint := &btcec.PublicKey{}
	x, y := btcec.S256().ScalarMult(
		pubKey.X, pubKey.Y, pk.D.Bytes())
	newPoint.X = x
	newPoint.Y = y

	shared = sha256.Sum256(newPoint.SerializeCompressed())
	return shared[:], nil
}

// PubKey returns the corresponding public key.
func (pk *privateKeyBitcoin) PubKey() PublicKey {
	return pk.pub
}

// update writes secret to the private key.
func (pk *privateKeyBitcoin) update(data []byte) {
	// construct the key pairs
	priv, pub := btcec.PrivKeyFromBytes(btcec.S256(), data)
	// assign the values
	pk.PrivateKey = priv
	pk.pub = &publicKeyBitcoin{pub}
}

// curveBitcoin implements the DH interface(aka "secp256k1").
type curveBitcoin struct {
	DHLEN int
}

// GenerateKeyPair creates a key pair from entropy. If the entropy is not
// supplied, it will use rand.Read to generate a new private key.
func (dh *curveBitcoin) GenerateKeyPair(entropy []byte) (PrivateKey, error) {
	secret := make([]byte, dhlenBitcoin)

	if entropy != nil {
		// entropy is given, use it to create the private key.
		copy(secret, entropy[:dhlenBitcoin])
	} else {
		// no entropy given, use the default rand.Read.
		if _, err := rand.Read(secret); err != nil {
			return nil, err
		}
	}

	pk := &privateKeyBitcoin{pub: &publicKeyBitcoin{}}
	pk.update(secret)

	return pk, nil
}

// LoadPrivateKey uses the data provided to create a new private key.
func (dh *curveBitcoin) LoadPrivateKey(data []byte) (PrivateKey, error) {
	p := &privateKeyBitcoin{pub: &publicKeyBitcoin{}}
	if len(data) != dhlenBitcoin {
		return nil, errMismatchedKey("private", dhlenBitcoin, len(data))
	}
	p.update(data)
	return p, nil
}

// LoadPublicKey uses the data provided to create a new public key.
func (dh *curveBitcoin) LoadPublicKey(data []byte) (PublicKey, error) {
	p := &publicKeyBitcoin{}
	if err := p.loadBytes(data); err != nil {
		return nil, err
	}
	return p, nil
}

// Size returns the DHLEN.
//
// Note that for secp256k1, the private key size is 32-byte, while the public
// key size is 33-byte. Because the Size param is used for specifying the public
// size for the babble package, we return the lenBitcoinPubKey instead of dhlen.
func (dh *curveBitcoin) Size() int {
	// return dh.DHLEN
	return lenBitcoinPubKey
}

func (dh *curveBitcoin) String() string {
	return "secp256k1"
}

func newCurveBitcoin() Curve {
	return &curveBitcoin{DHLEN: dhlenBitcoin}
}

func init() {
	Register("secp256k1", newCurveBitcoin)
}
