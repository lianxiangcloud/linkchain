package wallet

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/lianxiangcloud/linkchain/libs/common"
	lkctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/log"
	cfg "github.com/lianxiangcloud/linkchain/wallet/config"
)

// AccountBase UTXO account base
type AccountBase struct {
	Keys              []*lkctypes.AccountKey
	KeyIndex          map[lkctypes.PublicKey]uint64
	CreationTimestamp int64
	CurrIdx           uint64
	MainSecKey        lkctypes.SecretKey
	EthAddress        common.Address
}

func (a *AccountBase) String() string {
	// str := "{Keys:{"
	// for i := 0; i < len(a.Keys); i++ {
	// 	str = fmt.Sprintf("%s %d:[SpendSKey:%x,ViewSKey:%x,Address:%s,SubIdx:%d],\t", str, i, a.Keys[i].SpendSKey, a.Keys[i].ViewSKey, a.Keys[i].Address, a.Keys[i].SubIdx)
	// }
	// str += "},KeyIndex:{"
	// for k, v := range a.KeyIndex {
	// 	str = fmt.Sprintf("%s %x=>%d\t", str, k, v)
	// }
	// str = fmt.Sprintf("%s},CurrIdx:%d}", str, a.CurrIdx)

	return ""
}

type keyStroe struct {
	Address string `json:"address"`
}

// NewAccount return AccountBase ,constructed from config
func NewAccount(config *cfg.Config) *AccountBase {
	// if len(config.KeyWordsFile) > 0 {
	// 	log.Info("NewAccount from config.KeyWordsFile", "config.KeyWordsFile", config.KeyWordsFile)
	// 	keyswordsStr, err := ioutil.ReadFile(config.KeyWordsFile)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	keyswords := strings.Split(strings.TrimSpace(string(keyswordsStr)), "\n")

	// 	log.Warn("NewAccount", "keyswords", keyswords[0])

	// 	k, err := WordsToAccount(keyswords[0])
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	// FOR test

	// 	log.Warn("NewAccount success", "UTXO Address", k.GetKeys().Address)
	// 	return k
	// }

	if len(config.KeystoreFile) > 0 {
		log.Info("NewAccount from config.KeystoreFile", "config.KeystoreFile", config.KeystoreFile)
		keyjson, err := ioutil.ReadFile(config.KeystoreFile)
		if err != nil {
			panic(err)
		}
		var keyAddr keyStroe
		if err = json.Unmarshal(keyjson, &keyAddr); err != nil {
			panic(err)
		}

		pwd := config.Password
		if len(config.PasswordFile) > 0 {
			p, err := ioutil.ReadFile(config.KeystoreFile)
			if err != nil {
				panic(err)
			}

			pwd = string(p)
		}

		log.Debug("NewAccount from config.WalletFile", "password", pwd)

		rk, err := KeyFromAccount(keyjson, pwd)
		if err != nil {
			log.Error("NewAccount KeyFromAccount", "err", err)
			panic(err)
		}
		k, err := RecoveryKeyToAccount(rk)
		if err != nil {
			panic(err)
		}

		k.EthAddress = common.HexToAddress(keyAddr.Address)
		log.Warn("NewAccount success", "UTXO Address", k.GetKeys().Address, "EthAddress", k.EthAddress)
		return k
	}
	panic("not surport init account ways!!!")
}

// NewUTXOAccount return AccountBase ,constructed from config
func NewUTXOAccount(keystoreFile string, password string) *AccountBase {
	if len(keystoreFile) > 0 {
		log.Info("NewUTXOAccount from KeystoreFile", "keystoreFile", keystoreFile)
		keyjson, err := ioutil.ReadFile(keystoreFile)
		if err != nil {
			panic(err)
		}
		var keyAddr keyStroe
		if err = json.Unmarshal(keyjson, &keyAddr); err != nil {
			panic(err)
		}

		// log.Debug("NewUTXOAccount from config.WalletFile", "password", pwd)

		rk, err := KeyFromAccount(keyjson, password)
		if err != nil {
			log.Error("NewUTXOAccount KeyFromAccount", "err", err)
			panic(err)
		}
		k, err := RecoveryKeyToAccount(rk)
		if err != nil {
			panic(err)
		}

		k.EthAddress = common.HexToAddress(keyAddr.Address)
		log.Warn("NewUTXOAccount success", "UTXO Address", k.GetKeys().Address, "EthAddress", k.EthAddress)
		return k
	}
	panic("not surport init account ways!!!")
}

// GetKeys return CurrIdx key，default is main key
func (a *AccountBase) GetKeys() *lkctypes.AccountKey {
	return a.Keys[a.CurrIdx]
}

// GetCreatetime return accountBase create time
func (a *AccountBase) GetCreatetime() int64 {
	return a.CreationTimestamp
}

// CreateSubAccount create a sub address,return account address and sub index
func (a *AccountBase) CreateSubAccount() (string, uint64, error) {
	idx := uint64(len(a.Keys))
	priKey := a.Keys[0]
	log.Debug("CreateSubAccount", "priKey", priKey)
	addr := GetSubaddr(priKey, idx)
	kSub, err := StrToAddress(addr)
	if err != nil {
		return "", 0, err
	}
	log.Debug("CreateSubAccount", "priKey", priKey, "subAddr", addr, "MainSecKey", fmt.Sprintf("%x", a.MainSecKey))
	subAccount := lkctypes.AccountKey{Addr: *kSub, SubIdx: idx, Address: addr}

	//create sec key
	// subSecKey := xcrypto.GetSubaddressSecretKey(a.MainSecKey, uint32(idx))
	// spendSK, spendPK := GenerateKeys(subSecKey)
	// hash := crypto.Keccak256(spendSK[:])
	// var rk2 lktypes.SecretKey
	// copy(rk2[:], hash)
	// viewSK, viewPK := GenerateKeys(rk2)

	// subAccount.SpendSKey = spendSK
	// subAccount.ViewSKey = viewSK

	// if spendPK != kSub.SpendPublicKey {
	// 	log.Error("CreateSubAccount spendPK != kSub.SpendPublicKey", "spendPK", spendPK, "kSub.SpendPublicKey", kSub.SpendPublicKey)
	// 	return "", 0, fmt.Errorf("CreateSubAccount spendPK != kSub.SpendPublicKey,spendPK:%x,kSub.SpendPublicKey:%x", spendPK, kSub.SpendPublicKey)
	// }
	// if viewPK != kSub.ViewPublicKey {
	// 	log.Error("CreateSubAccount viewPK != kSub.ViewPublicKey", "viewPK", viewPK, "kSub.ViewPublicKey", kSub.ViewPublicKey)
	// 	return "", 0, fmt.Errorf("CreateSubAccount viewPK != kSub.ViewPublicKey,viewPK:%x,kSub.ViewPublicKey:%x", viewPK, kSub.ViewPublicKey)
	// }

	a.Keys = append(a.Keys, &subAccount)
	a.KeyIndex[kSub.SpendPublicKey] = idx
	return addr, idx, nil
}

// CreateSubAccountN create cnt subAddress
func (a *AccountBase) CreateSubAccountN(cnt int) error {
	for index := 1; index < cnt; index++ {
		_, _, err := a.CreateSubAccount()
		if err != nil {
			return err
		}
	}
	return nil
}
