#include "tctpl.hpp"
#include <string>

#define ContractCommitteeAddr "0x0000000000000000000000436f6d6d6974746565"
#define RKEY                  "right"
#define AddressLength         42

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

bool IsSenderNotBlackAddr(const tc::Address& sender, const tc::Address& blackAddress) {
    return sender != blackAddress;
}

class Blacklist : public TCBaseContract {
public:
    std::string CheckAddBlackAddress(std::string strBlackAddress) {
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "blacklist"), "Address does not have permission");
        TC_RequireWithMsg((strBlackAddress.length()%AddressLength == 0), "balck address arg illegal");
        std::string addStr = "addBlackAddress";
        for (int index = 0; index < strBlackAddress.length(); index = index+AddressLength) {
            std::string strAddr = strBlackAddress.substr(index, index+AddressLength);
            tc::Address addr    = tc::Address(strAddr.c_str());
            if (!IsSenderNotBlackAddr(tc::App::getInstance()->sender(), addr)) {
                return "";
            }
        }
        return addStr+strBlackAddress;
    }

    std::string CheckDelBlackAddress(std::string strBlackAddress) {
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "blacklist"), "Address does not have permission");
        TC_RequireWithMsg((strBlackAddress.length()%AddressLength == 0), "balck address arg illegal");
        std::string delString = "delBlackAddress";
        for (int index = 0; index < strBlackAddress.length(); index = index+AddressLength) {
            std::string strAddr = strBlackAddress.substr(index, index+AddressLength);
            tc::Address addr    = tc::Address(strAddr.c_str());
            if (!IsSenderNotBlackAddr(tc::App::getInstance()->sender(), addr)) {
                return "";
            }
        }
        return delString+strBlackAddress;
    }
};

TC_ABI(Blacklist, (CheckAddBlackAddress)(CheckDelBlackAddress))
