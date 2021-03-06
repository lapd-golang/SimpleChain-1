package crypto

import (
	"blockchain/btcsuite/btcd/btcec"
	"fmt"
	"blockchain/btcsuite/btcutil/base58"
	"encoding/json"
	"strings"
	"bytes"
	"golang.org/x/crypto/ripemd160"
)

const PublicKeyPrefix = "PUB_"
const PublicKeyPrefixCompat = "TFE"

type PublicKey struct {
	Content []byte
}

func NewPublicKey(pubKey string) (out PublicKey, err error) {
	if len(pubKey) < 8 {
		return out, fmt.Errorf("invalid format")
	}

	var pubKeyMaterial string
	if strings.HasPrefix(pubKey, PublicKeyPrefix) {
		pubKeyMaterial = pubKey[len(PublicKeyPrefix):] // strip "PUB_"
		pubKeyMaterial = pubKeyMaterial[3:] // strip "K1_"

	} else if strings.HasPrefix(pubKey, PublicKeyPrefixCompat) { // "TFE"
		pubKeyMaterial = pubKey[len(PublicKeyPrefixCompat):] // strip "TFE"

	} else {
		return out, fmt.Errorf("public key should start with %q (or the old %q)", PublicKeyPrefix, PublicKeyPrefixCompat)
	}

	pubDecoded, err := checkDecode(pubKeyMaterial)
	if err != nil {
		return out, fmt.Errorf("checkDecode: %s", err)
	}

	return PublicKey{Content: pubDecoded}, nil
}

// CheckDecode decodes a string that was encoded with CheckEncode and verifies the checksum.
func checkDecode(input string) (result []byte, err error) {
	decoded := base58.Decode(input)
	if len(decoded) < 5 {
		return nil, fmt.Errorf("invalid format")
	}
	var cksum [4]byte
	copy(cksum[:], decoded[len(decoded)-4:])
	///// WARN: ok the ripemd160checksum should include the prefix in CERTAIN situations,
	// like when we imported the PubKey without a prefix ?! tied to the string representation
	// or something ? weird.. checksum shouldn't change based on the string reprsentation.
	if bytes.Compare(ripemd160checksum(decoded[:len(decoded)-4]), cksum[:]) != 0 {
		return nil, fmt.Errorf("invalid checksum")
	}
	payload := decoded[:len(decoded)-4]
	result = append(result, payload...)
	return
}

func ripemd160checksum(in []byte) []byte {
	h := ripemd160.New()
	_, _ = h.Write(in) // this implementation has no error path
	sum := h.Sum(nil)
	return sum[:4]
}

func (p PublicKey) Key() (*btcec.PublicKey, error) {
	key, err := btcec.ParsePubKey(p.Content, btcec.S256())
	if err != nil {
		return nil, fmt.Errorf("parsePubKey: %s", err)
	}

	return key, nil
}

func (p PublicKey) String() string {
	hash := ripemd160checksum(p.Content)
	rawkey := append(p.Content, hash[:4]...)
	return PublicKeyPrefixCompat + base58.Encode(rawkey)
}

func (p PublicKey) MarshalJSON() ([]byte, error) {
	s := p.String()
	return json.Marshal(s)
}

func (p *PublicKey) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	newKey, err := NewPublicKey(s)
	if err != nil {
		return err
	}

	*p = newKey

	return nil
}