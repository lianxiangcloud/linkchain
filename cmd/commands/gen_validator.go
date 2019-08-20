package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
)

type keys struct {
	Pubkey string `json:"pubkey"`
	Prikey string `json:"prikey"`
}

// GenValidatorCmd allows the generation of a keypair for a
// validator.
var GenValidatorCmd = &cobra.Command{
	Use:   "gen_validator",
	Short: "Generate new validator keypair",
	Run:   genValidator,
}

func genValidator(cmd *cobra.Command, args []string) {
	pv := types.GenFilePV("")
	key := keys{
		Pubkey: common.ToHex(pv.PubKey.Bytes()),
		Prikey: common.ToHex(pv.PrivKey.Bytes()),
	}

	data, err := ser.MarshalJSONIndent(key, "", "  ")
	if err != nil {
		panic(err)
	}

	fmt.Printf("%v", string(data))
}
