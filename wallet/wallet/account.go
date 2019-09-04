package wallet

import (
	"encoding/json"
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
	EthAddress        common.Address
}

func (a *AccountBase) String() string {
	return a.EthAddress.String()
}

type keyStroe struct {
	Address string `json:"address"`
}

// NewAccount return AccountBase ,constructed from config
func NewAccount(config *cfg.Config) *AccountBase {
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
	addr := GetSubaddr(priKey, idx)
	kSub, err := StrToAddress(addr)
	if err != nil {
		return "", 0, err
	}
	subAccount := lkctypes.AccountKey{Addr: *kSub, SubIdx: idx, Address: addr}
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

// zeroKey zeroes a private key in memory.
func (a *AccountBase) ZeroKey() {
	mainAccount := a.GetKeys()
	if mainAccount != nil {
		for i := range mainAccount.SpendSKey {
			mainAccount.SpendSKey[i] = byte(0)
		}
		for i := range mainAccount.ViewSKey {
			mainAccount.ViewSKey[i] = byte(0)
		}
	}
}
