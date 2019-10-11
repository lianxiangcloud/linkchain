package rpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/ringct"
	lkctypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/xcrypto"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
	wtypes "github.com/lianxiangcloud/linkchain/wallet/types"
	"github.com/lianxiangcloud/linkchain/wallet/wallet"
)

// PublicTransactionPoolAPI exposes methods for the RPC interface
type PublicTransactionPoolAPI struct {
	b         Backend
	nonceLock *AddrLocker
	wallet    Wallet
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewPublicTransactionPoolAPI(b Backend, nonceLock *AddrLocker) *PublicTransactionPoolAPI {
	return &PublicTransactionPoolAPI{b, nonceLock, b.GetWallet()}
}

func (s *PublicTransactionPoolAPI) signUTXOTransaction(ctx context.Context, args wtypes.SendUTXOTxArgs) (*wtypes.SignUTXOTransactionResult, error) {
	args.SetDefaults()

	log.Debug("signTx", "input", args)
	destsCnt := len(args.Dests)
	if destsCnt == 0 {
		return nil, wtypes.ErrArgsInvalid
	}

	dests := make([]types.DestEntry, 0)
	hasOneAccountOutput := false
	utxoDestsCnt := 0
	for i := 0; i < destsCnt; i++ {
		toAddress := args.Dests[i].Addr
		if len(toAddress) == wtypes.UTXO_ADDR_STR_LEN {
			if utxoDestsCnt >= wtypes.UTXO_DESTS_MAX_NUM {
				return nil, wtypes.ErrUTXODestsOverLimit
			}
			// utxo address
			addr, err := wallet.StrToAddress(args.Dests[i].Addr)
			if err != nil {
				return nil, err
			}

			var remark [32]byte
			copy(remark[:], args.Dests[i].Remark[:])
			log.Debug("signUTXOTransaction", "Remark", args.Dests[i].Remark, "len", len(args.Dests[i].Remark), "remark", remark)
			isSubaddr, err := wallet.IsSubaddress(args.Dests[i].Addr)
			if err != nil {
				return nil, err
			}
			dests = append(dests, &types.UTXODestEntry{Addr: *addr, Amount: args.Dests[i].Amount.ToInt(), IsSubaddress: isSubaddr, Remark: remark})
			utxoDestsCnt++
		} else {
			if !common.IsHexAddress(toAddress) {
				return nil, wtypes.ErrArgsInvalid
			}
			if hasOneAccountOutput {
				// can not sign more than one account output
				return nil, wtypes.ErrAccDestsOverLimit
			}
			addr := common.HexToAddress(toAddress)
			dests = append(dests, &types.AccountDestEntry{To: addr, Amount: args.Dests[i].Amount.ToInt(), Data: args.Dests[i].Data})
			hasOneAccountOutput = true
		}

	}
	if args.From != common.EmptyAddress && hasOneAccountOutput {
		return nil, wtypes.ErrTxTypeNotSupport
	}

	txs, err := s.wallet.CreateUTXOTransaction(args.From, uint64(*args.Nonce), args.SubAddrs, dests, *args.TokenID, args.From, nil)
	if err != nil {
		return nil, err
	}

	var signedtxs []wtypes.SignUTXORet
	for _, tx := range txs {
		bz, err := ser.EncodeToBytes(tx)
		if err != nil {
			return nil, wtypes.ErrInnerServer
		}

		keys := tx.GetInputKeyImages()
		for i := 0; i < len(keys); i++ {
			log.Debug("signUTXOTransaction", "args.SubAddrs", args.SubAddrs, "keyimage", keys[i])
		}
		gas := hexutil.Uint64(tx.Gas())

		if args.From == common.EmptyAddress {
			addInfo, err := s.wallet.GetUTXOAddInfo(tx.Hash())
			if err != nil {
				return nil, err
			}
			signedtxs = append(signedtxs, wtypes.SignUTXORet{
				Raw:       fmt.Sprintf("0x%s", hex.EncodeToString(bz)),
				Hash:      tx.Hash(),
				Gas:       gas,
				Subaddrs:  addInfo.Subaddrs,
				OutAmount: addInfo.OutAmount,
			})
		} else {
			signedtxs = append(signedtxs, wtypes.SignUTXORet{Raw: fmt.Sprintf("0x%s", hex.EncodeToString(bz)), Hash: tx.Hash(), Gas: gas})
		}
	}
	return &wtypes.SignUTXOTransactionResult{Txs: signedtxs}, nil
}

// SignUTXOTransaction will sign the given transaction with the from account.
// The node needs to have the private key of the account corresponding with
// the given from address and it needs to be unlocked.
func (s *PublicTransactionPoolAPI) SignUTXOTransaction(ctx context.Context, args wtypes.SendUTXOTxArgs) (*wtypes.SignUTXOTransactionResult, error) {
	return s.signUTXOTransaction(ctx, args)
}

// SendUTXOTransaction send utxo tx
func (s *PublicTransactionPoolAPI) SendUTXOTransaction(ctx context.Context, args wtypes.SendUTXOTxArgs) (*wtypes.SendUTXOTransactionResult, error) {
	signRet, err := s.signUTXOTransaction(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(signRet.Txs) > 1 {
		return nil, wtypes.ErrTransNeedSplit
	}

	ret := s.wallet.Transfer([]string{signRet.Txs[0].Raw})
	ret[0].Gas = signRet.Txs[0].Gas

	return &wtypes.SendUTXOTransactionResult{Txs: ret}, nil
}

// SendUTXOTransactionSplit send utxo tx
func (s *PublicTransactionPoolAPI) SendUTXOTransactionSplit(ctx context.Context, args wtypes.SendUTXOTxArgs) (*wtypes.SendUTXOTransactionResult, error) {
	signRet, err := s.signUTXOTransaction(ctx, args)
	if err != nil {
		return nil, err
	}
	var signedRaw []string
	for i := 0; i < len(signRet.Txs); i++ {
		signedRaw = append(signedRaw, signRet.Txs[i].Raw)
	}
	ret := s.wallet.Transfer(signedRaw)
	for index := 0; index < len(signRet.Txs); index++ {
		ret[index].Gas = signRet.Txs[index].Gas
	}
	return &wtypes.SendUTXOTransactionResult{Txs: ret}, nil
}

// BlockHeight get block height
func (s *PublicTransactionPoolAPI) BlockHeight(ctx context.Context, addr *common.Address) (*wtypes.BlockHeightResult, error) {
	localHeight, remoteHeight := s.wallet.GetHeight(addr)

	return &wtypes.BlockHeightResult{LocalHeight: (*hexutil.Big)(localHeight), RemoteHeight: (*hexutil.Big)(remoteHeight)}, nil
}

// Balance get account Balance
func (s *PublicTransactionPoolAPI) Balance(ctx context.Context, args wtypes.BalanceArgs) (*wtypes.BalanceResult, error) {
	if args.TokenID == nil {
		args.TokenID = &common.EmptyAddress
	}

	address, err := s.wallet.GetAddress(uint64(args.AccountIndex), args.Addr)
	if err != nil {
		return nil, err
	}
	balance, err := s.wallet.GetBalance(uint64(args.AccountIndex), args.TokenID, args.Addr)
	if err != nil {
		return nil, err
	}

	return &wtypes.BalanceResult{Balance: (*hexutil.Big)(balance), Address: address, TokenID: args.TokenID}, err
}

// CreateSubAccount create sub account to max sub index
func (s *PublicTransactionPoolAPI) CreateSubAccount(ctx context.Context, maxSub hexutil.Uint64, addr *common.Address) (bool, error) {
	err := s.wallet.CreateSubAccount(uint64(maxSub), addr)
	if err != nil {
		return false, err
	}
	return true, nil
}

// Balance get account Balance
func (s *PublicTransactionPoolAPI) AutoRefreshBlockchain(ctx context.Context, autoRefresh bool, addr *common.Address) (bool, error) {
	err := s.wallet.AutoRefreshBlockchain(autoRefresh, addr)
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetAccountInfo get all sub accounts Balance
func (s *PublicTransactionPoolAPI) GetAccountInfo(ctx context.Context, tokenID *common.Address, addr *common.Address) (*wtypes.GetAccountInfoResult, error) {
	if tokenID == nil {
		tokenID = &common.EmptyAddress
	}
	ret, err := s.wallet.GetAccountInfo(tokenID, addr)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// RescanBlockchain reset wallet block and transfer info
func (s *PublicTransactionPoolAPI) RescanBlockchain(ctx context.Context, addr *common.Address) (bool, error) {
	err := s.wallet.RescanBlockchain(addr)
	if err != nil {
		return false, err
	}
	return true, nil
}

// Status return wallet status
func (s *PublicTransactionPoolAPI) Status(ctx context.Context, addr *common.Address) (*wtypes.StatusResult, error) {
	status := s.wallet.Status(addr)
	return status, nil
}

// GetTxKey return tx key
func (s *PublicTransactionPoolAPI) GetTxKey(ctx context.Context, hash common.Hash, addr *common.Address) (*lkctypes.Key, error) {
	return s.wallet.GetTxKey(&hash, addr)
}

// CheckTxKey Check a transaction in the blockchain with its secret key.
// func (s *PublicTransactionPoolAPI) CheckTxKey(ctx context.Context, args wtypes.CheckTxKeyArgs) (*wtypes.CheckTxKeyResult, error) {
// 	block, amount, err := s.wallet.CheckTxKey(&args.TxHash, &args.TxKey, args.DestAddr)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &wtypes.CheckTxKeyResult{BlockID: *block, Amount: amount}, err
// }

// GetMaxOutput return max output
func (s *PublicTransactionPoolAPI) GetMaxOutput(ctx context.Context, tokenID common.Address, addr *common.Address) (*hexutil.Uint64, error) {
	return s.wallet.GetMaxOutput(tokenID, addr)
}

// GetProofKey return proof key
func (s *PublicTransactionPoolAPI) GetProofKey(ctx context.Context, args wtypes.ProofKeyArgs) (*wtypes.ProofKeyRet, error) {
	if len(args.Addr) != wtypes.UTXO_ADDR_STR_LEN && len(args.Addr) != common.AddressLength*2 && len(args.Addr) != common.AddressLength*2+2 {
		return nil, wtypes.ErrArgsInvalid
	}
	tx, err := s.wallet.GetUTXOTx(args.Hash, args.EthAddr)
	if err != nil {
		return nil, err
	}
	if (tx.UTXOKind() & types.Ain) != types.IllKind {
		return nil, wtypes.ErrNoNeedToProof
	}
	if len(args.Addr) == wtypes.UTXO_ADDR_STR_LEN {
		addr, err := wallet.StrToAddress(args.Addr)
		if err != nil {
			return nil, err
		}
		txKey, err := s.wallet.GetTxKey(&args.Hash, args.EthAddr)
		if err != nil {
			return nil, err
		}
		derivationKey, err := xcrypto.GenerateKeyDerivation(addr.ViewPublicKey, lkctypes.SecretKey(*txKey))
		if err != nil {
			return nil, wtypes.ErrInnerServer
		}
		outIdx := 0
		for _, output := range tx.Outputs {
			if utxoOutput, ok := output.(*types.UTXOOutput); ok {
				otAddr, err := xcrypto.DerivePublicKey(derivationKey, outIdx, addr.SpendPublicKey)
				if err != nil {
					return nil, wtypes.ErrInnerServer
				}
				if bytes.Equal(otAddr[:], utxoOutput.OTAddr[:]) {
					return &wtypes.ProofKeyRet{
						ProofKey: fmt.Sprintf("%x", derivationKey[:]),
					}, nil
				}
				outIdx++
			}
		}
		return nil, wtypes.ErrNoTransInTx
	}

	if !common.IsHexAddress(args.Addr) {
		return nil, wtypes.ErrArgsInvalid
	}
	addr := common.HexToAddress(args.Addr)
	txKey, err := s.wallet.GetTxKey(&args.Hash, args.EthAddr)
	if err != nil {
		return nil, err
	}
	for i, output := range tx.Outputs {
		if accOutput, ok := output.(*types.AccountOutput); ok {
			if bytes.Equal(addr[:], accOutput.To[:]) {
				proofKey, err := xcrypto.DerivationToScalar(lkctypes.KeyDerivation(*txKey), i)
				if err != nil {
					return nil, wtypes.ErrInnerServer
				}
				return &wtypes.ProofKeyRet{
					ProofKey: fmt.Sprintf("%x", proofKey[:]),
				}, nil
			}
		}
	}
	return nil, wtypes.ErrNoTransInTx
}

// CheckProofKey verify proof key
func (s *PublicTransactionPoolAPI) CheckProofKey(ctx context.Context, args wtypes.VerifyProofKeyArgs) (*wtypes.VerifyProofKeyRet, error) {
	if len(args.Addr) != wtypes.UTXO_ADDR_STR_LEN && len(args.Addr) != common.AddressLength*2 && len(args.Addr) != common.AddressLength*2+2 {
		return nil, wtypes.ErrArgsInvalid
	}
	tx, err := s.wallet.GetUTXOTx(args.Hash, args.EthAddr)
	if err != nil {
		return nil, err
	}
	if (tx.UTXOKind() & types.Ain) != types.IllKind {
		return nil, wtypes.ErrNoNeedToProof
	}
	k, err := hex.DecodeString(args.Key)
	if err != nil || len(k) != lkctypes.COMMONLEN {
		return nil, wtypes.ErrArgsInvalid
	}
	var key lkctypes.Key
	copy(key[:], k[:])
	ret := &wtypes.VerifyProofKeyRet{
		Records: make([]*wtypes.VerifyProofKey, 0),
	}
	if len(args.Addr) == wtypes.UTXO_ADDR_STR_LEN {
		addr, err := wallet.StrToAddress(args.Addr)
		if err != nil {
			return nil, err
		}
		outIdx := 0
		for _, output := range tx.Outputs {
			if utxoOutput, ok := output.(*types.UTXOOutput); ok {
				otAddr, err := xcrypto.DerivePublicKey(lkctypes.KeyDerivation(key), outIdx, addr.SpendPublicKey)
				if err != nil {
					return nil, wtypes.ErrInnerServer
				}
				if bytes.Equal(otAddr[:], utxoOutput.OTAddr[:]) {
					ecdh := &lkctypes.EcdhTuple{
						Amount: tx.RCTSig.RctSigBase.EcdhInfo[outIdx].Amount,
					}
					scalar, err := xcrypto.DerivationToScalar(lkctypes.KeyDerivation(key), outIdx)
					if err != nil {
						return nil, wtypes.ErrInnerServer
					}
					ok := xcrypto.EcdhDecode(ecdh, lkctypes.Key(scalar), false)
					if !ok {
						return nil, wtypes.ErrInnerServer
					}
					//check encode amount is valid
					outAmountKeys := []lkctypes.Key{ecdh.Amount}
					outMKeys := lkctypes.KeyV{lkctypes.Key(scalar)}
					_, tCommits, _, err := ringct.ProveRangeBulletproof(outAmountKeys, outMKeys)
					if err != nil || len(tCommits) != 1 {
						return nil, wtypes.ErrTransInvalid
					}
					tMask, _ := ringct.Scalarmult8(tCommits[0])
					if !bytes.Equal(tx.RCTSig.OutPk[outIdx].Mask[:], tMask[:]) {
						return nil, wtypes.ErrTransInvalid
					}
					ret.Records = append(ret.Records, &wtypes.VerifyProofKey{
						Hash:   args.Hash,
						Addr:   args.Addr,
						Amount: (*hexutil.Big)(big.NewInt(0).Mul(types.Hash2BigInt(ecdh.Amount), big.NewInt(types.GetUtxoCommitmentChangeRate(tx.TokenID)))),
					})
				}
				outIdx++
			}
		}
		return ret, nil
	}

	if !common.IsHexAddress(args.Addr) {
		return nil, wtypes.ErrArgsInvalid
	}
	addr := common.HexToAddress(args.Addr)
	for _, output := range tx.Outputs {
		if accOutput, ok := output.(*types.AccountOutput); ok {
			if bytes.Equal(addr[:], accOutput.To[:]) {
				data := make([]byte, lkctypes.COMMONLEN+common.AddressLength)
				copy(data[0:], key[:])
				copy(data[len(data):], addr[:])
				pkey := crypto.Sha256(data)
				for _, addKey := range tx.AddKeys {
					if bytes.Equal(pkey[:], addKey[:]) {
						ret.Records = append(ret.Records, &wtypes.VerifyProofKey{
							Hash:   args.Hash,
							Addr:   args.Addr,
							Amount: (*hexutil.Big)(accOutput.Amount),
						})
						break
					}
				}
			}
		}
	}
	return ret, nil
}

// SelectAddress set wallet curr account
func (s *PublicTransactionPoolAPI) SelectAddress(ctx context.Context, addr common.Address) (bool, error) {
	err := s.wallet.SelectAddress(addr)
	return err == nil, err
}

// SetRefreshBlockInterval set wallet curr account
func (s *PublicTransactionPoolAPI) SetRefreshBlockInterval(ctx context.Context, interval time.Duration, addr *common.Address) (bool, error) {
	if interval <= time.Duration(0) {
		//return false, fmt.Errorf("interval must be greater than 0")
		return false, wtypes.ErrArgsInvalid
	}
	sec := interval * time.Second
	err := s.wallet.SetRefreshBlockInterval(sec, addr)
	return err == nil, err
}

// GetLocalUTXOTxsByHeight return
func (s *PublicTransactionPoolAPI) GetLocalUTXOTxsByHeight(ctx context.Context, height *hexutil.Big, addr *common.Address) (*wtypes.UTXOBlock, error) {
	return s.wallet.GetLocalUTXOTxsByHeight((*big.Int)(height), addr)
}

// GetLocalOutputs return
func (s *PublicTransactionPoolAPI) GetLocalOutputs(ctx context.Context, args wtypes.LocalOutputsArgs) ([]wtypes.UTXOOutputDetail, error) {
	return s.wallet.GetLocalOutputs(args.IDs, args.Addr)
}
