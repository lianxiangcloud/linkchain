package crypto

import (
	"fmt"
	"strings"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
)

func HexToPubkey(pub string) (PubKey, error) {
	key, err := hexutil.Decode(pub)
	if err != nil {
		return nil, err
	}
	pubKey, err := PubKeyFromBytes(key)
	return pubKey, err
}

func HexToPrivkey(priv string) (PrivKey, error) {
	key, err := hexutil.Decode(priv)
	if err != nil {
		return nil, err
	}
	privKey, err := PrivKeyFromBytes(key)
	return privKey, err
}

func CheckPriKey(pubKey, priKey string) (bool, error) {
	privKey, err := HexToPrivkey(priKey)
	if err != nil {
		return false, fmt.Errorf("CheckPriKey decode error : %v", err)
	}
	pubKeyStr := hexutil.Encode(privKey.PubKey().Bytes())
	if strings.ToLower(pubKeyStr) != strings.ToLower(pubKey) {
		return false, fmt.Errorf("pubkey and prikey mismatch : want=%s, get=%s", pubKey, pubKeyStr)
	}
	return true, nil
}
