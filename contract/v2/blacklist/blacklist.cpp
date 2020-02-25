#include "tctpl.hpp"
#include <string>

#define ContractCommitteeAddr "0x0000000000000000000000436f6d6d6974746565"
#define EmptyAddress          "0x0000000000000000000000000000000000000000"
#define RKEY                  "right"
#define AddressLength         42
#define MintAdminKey          "blacklist"
#define MintWhitelistKey      "blacklist"

const tc::Address GetRightAccount(const std::string& right) {
	tc::Address addr;
    StorMap<Key<std::string>, tc::Address> rights(RKEY);

    const tlv::BufferWriter key = Key<std::string>::keyStr(right);
    uint8_t* tmp = rights.getKeyBytes(key);
    uint8_t* value = TC_ContractStoragePureGet(ContractCommitteeAddr, tmp, std::string(RKEY).length() + key.length());
    free(tmp);

    tc::tlv::BufferReader buffer(value);
    unpack(buffer, addr);

	return addr;
}

bool CheckAddrRight(const tc::Address& addr, const std::string& right) {
	return addr == GetRightAccount(right);
}

bool IsAddressIllegal(const tc::Address& sender, const tc::Address& blackAddress) {
    tc::Address liankeAddress = tc::Address(EmptyAddress);
    return (sender == blackAddress) || (blackAddress == liankeAddress);
}

class Blacklist : public TCBaseContract {
public:
    /*
    * Note: CheckAddBlackAddress Used to verify that the newly added blacklist address is valid.
    *      If it is legal, it returns a fixed prefix and a new blacklist. Return an empty string if it is not legal.
    * Arg: Hexadecimal address length is 42 bytes
    *      The length of the parameter must be a multiple of 42.
    *      For example: 0x00000000000000000000000000000000000000000x0000000000000000000000000000000000000001
    */
    std::string CheckAddBlackAddress(std::string strBlackAddress) {
        const tc::Address& sender = tc::App::getInstance()->sender();
        TC_RequireWithMsg(CheckAddrRight(sender, "blacklist"), "Address does not have permission");
        TC_RequireWithMsg((strBlackAddress.length()%AddressLength == 0), "balck address arg illegal");
        std::string addStr = "addBlackAddress";
        for (int index = 0; index < strBlackAddress.length(); index = index+AddressLength) {
            std::string strAddr = strBlackAddress.substr(index, AddressLength);
            TC_RequireWithMsg(TC_IsHexAddress(strAddr.c_str()), "address illegal(must HexAddress)");
            tc::Address addr    = tc::Address(strAddr.c_str());
            TC_RequireWithMsg(!IsAddressIllegal(sender, addr), "address illegal");
        }
        return addStr+strBlackAddress;
    }

    /*
    * Note: CheckDelBlackAddress Used to verify that the deleted blacklist address is valid.
    *      If it is legal, it returns a fixed prefix and a new blacklist. Return an empty string if it is not legal.
    * Arg: Hexadecimal address length is 42 bytes
    *      The length of the parameter must be a multiple of 42.
    *      For example: 0x00000000000000000000000000000000000000000x0000000000000000000000000000000000000001
    */
    std::string CheckDelBlackAddress(std::string strBlackAddress) {
        const tc::Address& sender = tc::App::getInstance()->sender();
        TC_RequireWithMsg(CheckAddrRight(sender, "blacklist"), "Address does not have permission");
        TC_RequireWithMsg((strBlackAddress.length()%AddressLength == 0), "balck address arg illegal");
        std::string delString = "delBlackAddress";
        for (int index = 0; index < strBlackAddress.length(); index = index+AddressLength) {
            std::string strAddr = strBlackAddress.substr(index, AddressLength);
            TC_RequireWithMsg(TC_IsHexAddress(strAddr.c_str()), "address illegal(must HexAddress)");
            tc::Address addr    = tc::Address(strAddr.c_str());
            TC_RequireWithMsg(!IsAddressIllegal(sender, addr), "address illegal");
        }
        return delString+strBlackAddress;
    }

    void SetMintAdminAddr(const tc::BInt addr) {
        TC_Payable(false);
        TC_RequireWithMsg(addr != EmptyAddress, "admin address can not be empty address");
        const tc::Address& sender = tc::App::getInstance()->sender();
        TC_RequireWithMsg(CheckAddrRight(sender, MintAdminKey), "Address does not have permission");
        m_mintAdminAddr.set(addr);
    }

    const std::string MintAdminAddr() {
        TC_Payable(false);
        if (m_mintAdminAddr.get() == EmptyAddress) {
            return GetRightAccount(MintAdminKey).toString();
        }
        return m_mintAdminAddr.get().toString();
    }

    void InsertMintWhitelist(const std::string coinName, const tc::Address tokenId) {
        TC_Payable(false);
        TC_RequireWithMsg(coinName.length() > 0, "coin name param not set");
        TC_RequireWithMsg(tokenId != EmptyAddress, "tokenId param illegal");
        const tc::Address& sender = tc::App::getInstance()->sender();
        TC_RequireWithMsg(CheckAddrRight(sender, MintWhitelistKey), "Address does not have permission");
        tc::Address checkAddr = m_mapMintWhitelist.get(coinName);
        TC_RequireWithMsg(checkAddr == EmptyAddress, "this mintage contarct has been set");
        m_mapMintWhitelist.set(tokenId, coinName);
    }

    void DeleteMintWhitelist(const std::string coinName) {
        TC_Payable(false);
        TC_RequireWithMsg(coinName.length() > 0, "coin name param not set");
        const tc::Address& sender = tc::App::getInstance()->sender();
        TC_RequireWithMsg(CheckAddrRight(sender, MintWhitelistKey), "Address does not have permission");
        tc::Address checkAddr = m_mapMintWhitelist.get(coinName);
        TC_RequireWithMsg(checkAddr != EmptyAddress, "this mintage contarct not exist");
        tc::Address emptyTokenId = EmptyAddress;
        m_mapMintWhitelist.set(emptyTokenId, coinName);
    }

    void UpdateMintWhitelist(const std::string coinName, const tc::Address tokenId) {
        TC_Payable(false);
        TC_RequireWithMsg(coinName.length() > 0, "coin name param not set");
        TC_RequireWithMsg(tokenId != EmptyAddress, "tokenId param illegal");
        const tc::Address& sender = tc::App::getInstance()->sender();
        TC_RequireWithMsg(CheckAddrRight(sender, MintWhitelistKey), "Address does not have permission");
        tc::Address checkAddr = m_mapMintWhitelist.get(coinName);
        TC_RequireWithMsg(checkAddr != EmptyAddress, "this mintage contarct not set");
        m_mapMintWhitelist.set(tokenId, coinName);
    }

    std::string SelectMintWhitelist(const std::string coinName) {
        TC_Payable(false);
        TC_RequireWithMsg(coinName.length() > 0, "coin name param not set");
        tc::Address tokenId = m_mapMintWhitelist.get(coinName);
        TC_RequireWithMsg(tokenId != EmptyAddress, "this mintage contarct not exist");
        return tokenId.toString();
    }

private:
    tc::StorValue<tc::BInt> m_mintAdminAddr{"m_mintAdminAddr"};
    StorMap<Key<std::string>, tc::Address> m_mapMintWhitelist{"m_mapMintWhitelist"};
};

TC_ABI(Blacklist, 
    (CheckAddBlackAddress)\
    (CheckDelBlackAddress)\
    (SetMintAdminAddr)\
    (MintAdminAddr)\
    (InsertMintWhitelist)\
    (DeleteMintWhitelist)\
    (UpdateMintWhitelist)\
    (SelectMintWhitelist)\
)