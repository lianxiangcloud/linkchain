package wallet

import (
	"math/big"
	"testing"

	. "github.com/bouk/monkey"
	"github.com/lianxiangcloud/linkchain/wallet/daemon"

	// . "github.com/prashantv/gostub"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGetBlockUTXOsByNumber(t *testing.T) {
	Convey("test GetBlockUTXOsByNumber", t, func() {

		Convey("for GetBlockUTXOsByNumber fail", func() {
			// curl -X POST --data '{"jsonrpc":"2.0","method":"eth_getBlockUTXOsByNumber","params":["0x0",true],"id":0}'  https://c32024s1.lianxiangcloud.com:10443/getBlockUTXOsByNumber
			Patch(daemon.CallJSONRPC, func(method string, params interface{}) ([]byte, error) {
				return []byte(`{"jsonrpc":"2.0","id":0,"error":{"code":-32000,"message":"LoadBlock fail"}}`), nil
			})
			defer UnpatchAll()

			blockExpect := big.NewInt(1)

			block, err := GetBlockUTXOsByNumber(blockExpect)
			So(err, ShouldNotBeNil)
			So(block, ShouldBeNil)
		})
		Convey("for GetBlockUTXOsByNumber succ", func() {
			// curl -X POST --data '{"jsonrpc":"2.0","method":"eth_getBlockUTXOsByNumber","params":["0xff4c6",true],"id":0}'  https://c32024s1.lianxiangcloud.com:10443/getBlockUTXOsByNumber
			Patch(daemon.CallJSONRPC, func(method string, params interface{}) ([]byte, error) {
				return []byte(`{"jsonrpc":"2.0","id":0,"result":{"number":"0xff4c6","hash":"0x7f32c6d2407dfefb34b72787009f56ec8d4a1c97590f94dfbeedc5eb4b42d2ba","miner":"0x00000000000000000000466f756e646174696f6e","timestamp":"0x5d7782c5","parentHash":"0x81ddd8c44168ed018890bc7aab7469100d9978bbd88b3eadca5e0528ba4c5ab4","transactionsRoot":"0x38e1ff837f8b2eb083634b6893c70324a51caa0d3fbcc0f9188284b255c0ba08","stateRoot":"0xe2a4642a75f120915fb9eed7b337906d7e470c2ded5cffaa5bd118c34556846e","receiptsRoot":"0xa7187b9495c4dae16d785413bf89dabd79fe2f1fe20d4304179e6a9d8977a029","gasLimit":"0x12a05f200","gasUsed":"0x4c4b40","logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","transactions":[{"type":"rpctx","value":{"txType":"utx","txHash":"0x38e1ff837f8b2eb083634b6893c70324a51caa0d3fbcc0f9188284b255c0ba08","from":"0xa73810e519e1075010678d706533486d8ecc8000","tx":{"type":"utx","value":{"inputs":[{"type":"AccountInput","value":{"nonce":"0","amount":100500000000000000000,"cf":"TRYEPcTiuLEtAsaObFkvDiqVNjzb2NJYMCvG1nlavAE=","commit":"mnkhbcKnBc4BAQNdUHf58iGJohVhI56h5xdfzqZFuik="}}],"outputs":[{"type":"UTXOOutput","value":{"otaddr":"axDZrTuapP+LnUDL6TL37UWlLdZhLdUpQA96JxPPC70=","amount":0,"remark":"aMMCvuEn1iVfTPH8OBAODUwEiJChvvolyHyk6OIaC/E="}}],"token_id":"0x0000000000000000000000000000000000000000","r_key":"C5PKo3YJ/NaFPSHtzOGw1TqI8FghNhaNwGCsE3wduzs=","add_keys":["tZZNmpXBaJ5+p5Ip9m8q3WUIcWLN2sRytdVJrfrUK5s="],"fee":500000000000000000,"extra":null,"signature":{"v":58344,"r":56322138348917690189454387989114684365943480771812337665258431555425659143207,"s":118435920303938730491225226847964653980441608217120382623072372002762842389},"rct_signature":{"RctSigBase":{"Type":0,"PseudoOuts":[],"EcdhInfo":[{"Mask":"qbIwk/OFwiwG2j6PU66pqAvt3FyZF9ifSryI3K4JeQI=","Amount":"58tKvSIwg3txHr1TCDApRlXqA8UOXcYOcN1CahnWhwQ=","SenderPK":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}],"OutPk":[{"Dest":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","Mask":"M06ur3wkmVsoxkAT2JVpxLzB6g3pPHXjDBKIYLxTmcQ="}],"TxnFee":"0"},"P":{"RangeSigs":[],"Bulletproofs":[{"A":"8eANrm+Xry+RXzrXnFLcXnJk4G2mE7c7PnO2QY/rPsE=","S":"pis8ID3s81j1jpr0+5fMgkkRM2G+Qn4f2JaB/+Vg9kc=","T1":"Bkp2Ghkh7NWGcsKrDilLlvz1us1NR155SIp3ed/m1L8=","T2":"bznXwOT/mJFdLCXCZzJbdN4cEMXT+gu0DtxMnebg+ao=","Taux":"hctFJQjyxHCjwv87aDibVbW0YE2aLP8GsLuFLi6hpgg=","Mu":"P+pXo9Byb2WvNL1PxejwV15IUmvjXEpI/JFJ1FxzAQY=","L":["q2Zk5EoroeM37gMTd2VXN2ySyfhZWimZY1+gBYRZXWY=","ZZZeYugN1mqViTb3agLiwyv8Ei24C6zMqhzhXk/gg3c=","lp56DeAXSQvu/y/gVitEl79I8kvuKj+IvD4idxS8VSY=","45zLygxl2K9u0gIsNfMRuXf9/al2kiClmb/wg9kH+bc=","rAVSel6AZbvfnExgdiwBrW77RPd9dGQua6WZY9JicOw=","h31NpvHAMypRCNt8Y88LjuMUmZfoU9EpQb+lXaO+4Ck="],"R":["G+4SqJaibOjmFY6SX1xWeUNFp57ar9AVaOzeSQnnDlc=","XLKnXP20m6tezGwf/mKITCTiVnwHyHPV+hAKfdj5Ijg=","fnKyxK/ZTzeJrjxAPXS9LVbbFcDgsC1Lt27BomZBR2g=","iKcOgaPUgTGN+j38lhRo2WTxsMt40+yCCjAZR58RpV8=","8To7g7BRrv1/39XiXQAoOjhjg4Hipo2kheoIDkCOiD8=","v3kpy6jGk7NDQtR3nlMmKlD47PhbdFR/X05Oaca/QCg="],"Aa":"LiRL42iFDQF1eV00Ahxkia6h5w2tUjxyz/TUwMLR0gg=","B":"HHMMAsO9ctjm4cAmRgfj0WqRWQiClJmJdqHeOcjiNQM=","T":"EAV+XTdWC9wUqjMWg1cuShMTdkw2ySgFK9N4ppUSNAk="}],"MGs":[],"PseudoOuts":[],"Ss":[]}}}}}}],"token_output_seqs":{"0x0000000000000000000000000000000000000000":-1}}}`), nil
			})
			defer UnpatchAll()

			blockExpect := big.NewInt(0xff4c6)

			block, err := GetBlockUTXOsByNumber(blockExpect)
			So(err, ShouldBeNil)
			So(block.Height.ToInt().String(), ShouldEqual, blockExpect.String())
		})
	})
}

func TestGetBlockUTXO(t *testing.T) {
	Convey("test GetBlockUTXO", t, func() {
		Convey("for GetBlockUTXO block null", func() {
			Patch(daemon.CallJSONRPC, func(method string, params interface{}) ([]byte, error) {
				return []byte(`{"id":1,"jsonrpc":"2.0","result":{"block":null,"max_height":"0x119e9d","next_height":"0xff4c6"}}`), nil
			})
			defer UnpatchAll()

			block, err := GetBlockUTXO(big.NewInt(0))

			So(err, ShouldBeNil)
			So(block.MaxHeight.String(), ShouldEqual, "0x119e9d")
			So(block.NextHeight.String(), ShouldEqual, "0xff4c6")
		})
		// Convey("for GetBlockUTXO block not null", func() {
		// 	Patch(daemon.CallJSONRPC, func(method string, params interface{}) ([]byte, error) {
		// 		return []byte(`{"id":1,"jsonrpc":"2.0","result":{"block":{"gasLimit":"0x12a05f200","gasUsed":"0x4c4b40","hash":"0x7f32c6d2407dfefb34b72787009f56ec8d4a1c97590f94dfbeedc5eb4b42d2ba","logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","miner":"0x00000000000000000000466f756e646174696f6e","number":"0xff4c6","parentHash":"0x81ddd8c44168ed018890bc7aab7469100d9978bbd88b3eadca5e0528ba4c5ab4","receiptsRoot":"0xa7187b9495c4dae16d785413bf89dabd79fe2f1fe20d4304179e6a9d8977a029","stateRoot":"0xe2a4642a75f120915fb9eed7b337906d7e470c2ded5cffaa5bd118c34556846e","timestamp":"0x5d7782c5","token_output_seqs":{"0x0000000000000000000000000000000000000000":-1},"transactions":[{"type":"rpctx","value":{"from":"0xa73810e519e1075010678d706533486d8ecc8000","tx":{"type":"utx","value":{"add_keys":["tZZNmpXBaJ5+p5Ip9m8q3WUIcWLN2sRytdVJrfrUK5s="],"extra":null,"fee":500000000000000000,"inputs":[{"type":"AccountInput","value":{"amount":100500000000000000000,"cf":"TRYEPcTiuLEtAsaObFkvDiqVNjzb2NJYMCvG1nlavAE=","commit":"mnkhbcKnBc4BAQNdUHf58iGJohVhI56h5xdfzqZFuik=","nonce":"0"}}],"outputs":[{"type":"UTXOOutput","value":{"amount":0,"otaddr":"axDZrTuapP+LnUDL6TL37UWlLdZhLdUpQA96JxPPC70=","remark":"aMMCvuEn1iVfTPH8OBAODUwEiJChvvolyHyk6OIaC/E="}}],"r_key":"C5PKo3YJ/NaFPSHtzOGw1TqI8FghNhaNwGCsE3wduzs=","rct_signature":{"P":{"Bulletproofs":[{"A":"8eANrm+Xry+RXzrXnFLcXnJk4G2mE7c7PnO2QY/rPsE=","Aa":"LiRL42iFDQF1eV00Ahxkia6h5w2tUjxyz/TUwMLR0gg=","B":"HHMMAsO9ctjm4cAmRgfj0WqRWQiClJmJdqHeOcjiNQM=","L":["q2Zk5EoroeM37gMTd2VXN2ySyfhZWimZY1+gBYRZXWY=","ZZZeYugN1mqViTb3agLiwyv8Ei24C6zMqhzhXk/gg3c=","lp56DeAXSQvu/y/gVitEl79I8kvuKj+IvD4idxS8VSY=","45zLygxl2K9u0gIsNfMRuXf9/al2kiClmb/wg9kH+bc=","rAVSel6AZbvfnExgdiwBrW77RPd9dGQua6WZY9JicOw=","h31NpvHAMypRCNt8Y88LjuMUmZfoU9EpQb+lXaO+4Ck="],"Mu":"P+pXo9Byb2WvNL1PxejwV15IUmvjXEpI/JFJ1FxzAQY=","R":["G+4SqJaibOjmFY6SX1xWeUNFp57ar9AVaOzeSQnnDlc=","XLKnXP20m6tezGwf/mKITCTiVnwHyHPV+hAKfdj5Ijg=","fnKyxK/ZTzeJrjxAPXS9LVbbFcDgsC1Lt27BomZBR2g=","iKcOgaPUgTGN+j38lhRo2WTxsMt40+yCCjAZR58RpV8=","8To7g7BRrv1/39XiXQAoOjhjg4Hipo2kheoIDkCOiD8=","v3kpy6jGk7NDQtR3nlMmKlD47PhbdFR/X05Oaca/QCg="],"S":"pis8ID3s81j1jpr0+5fMgkkRM2G+Qn4f2JaB/+Vg9kc=","T":"EAV+XTdWC9wUqjMWg1cuShMTdkw2ySgFK9N4ppUSNAk=","T1":"Bkp2Ghkh7NWGcsKrDilLlvz1us1NR155SIp3ed/m1L8=","T2":"bznXwOT/mJFdLCXCZzJbdN4cEMXT+gu0DtxMnebg+ao=","Taux":"hctFJQjyxHCjwv87aDibVbW0YE2aLP8GsLuFLi6hpgg="}],"MGs":null,"PseudoOuts":null,"RangeSigs":null,"Ss":null},"RctSigBase":{"EcdhInfo":[{"Amount":"58tKvSIwg3txHr1TCDApRlXqA8UOXcYOcN1CahnWhwQ=","Mask":"qbIwk/OFwiwG2j6PU66pqAvt3FyZF9ifSryI3K4JeQI=","SenderPK":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}],"OutPk":[{"Dest":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","Mask":"M06ur3wkmVsoxkAT2JVpxLzB6g3pPHXjDBKIYLxTmcQ="}],"PseudoOuts":null,"TxnFee":"0","Type":0}},"signature":{"r":5.632213834891769e+76,"s":1.1843592030393873e+74,"v":58344},"token_id":"0x0000000000000000000000000000000000000000"}},"txHash":"0x38e1ff837f8b2eb083634b6893c70324a51caa0d3fbcc0f9188284b255c0ba08","txType":"utx"}}],"transactionsRoot":"0x38e1ff837f8b2eb083634b6893c70324a51caa0d3fbcc0f9188284b255c0ba08"},"max_height":"0x119e9d","next_height":"0xff4c7"}}`), nil
		// 	})
		// 	defer UnpatchAll()

		// 	block, err := GetBlockUTXO(big.NewInt(0))

		// 	So(err, ShouldBeNil)
		// 	So(len(block.Block.Txs), ShouldEqual, 1)
		// 	So(block.MaxHeight.String(), ShouldEqual, "0x119e9d")
		// 	So(block.NextHeight.String(), ShouldEqual, "0xff4c7")
		// })

	})

}
