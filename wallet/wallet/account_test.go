package wallet

import "testing"

func TestCreateSubAccount(t *testing.T) {
	mockWallet.currAccount.account.CreateSubAccount()
}
