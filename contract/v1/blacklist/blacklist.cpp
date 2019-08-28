#include "tctpl.hpp"
#include <set>
#include <string>

#define ContractCommitteeAddr "0x0000000000000000000000436f6d6d6974746565"
#define RKEY "right"
#define BLACKLIST "blacklist"

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
    void Init() {
        tc::StorValue<std::set<tc::Address>> blacklist(BLACKLIST);
        auto addrs = blacklist.get();
        blacklist.set(addrs);
    }

    void AddBlackAddress(tc::Address blackAddress) {
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "blacklist"), "Address does not have permission");
        TC_RequireWithMsg(IsSenderNotBlackAddr(tc::App::getInstance()->sender(), blackAddress), "blackAddress can not be sender");
        tc::StorValue<std::set<tc::Address>> blacklist(BLACKLIST);
        auto addrs = blacklist.get();
        addrs.insert(blackAddress);
        blacklist.set(addrs);
    }

    void DelBlackAddress(tc::Address blackAddress) {
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "blacklist"), "Address does not have permission");
        TC_RequireWithMsg(IsSenderNotBlackAddr(tc::App::getInstance()->sender(), blackAddress), "blackAddress can not be sender");
        tc::StorValue<std::set<tc::Address>> blacklist(BLACKLIST);
        auto addrs = blacklist.get();
        addrs.erase(blackAddress);
        blacklist.set(addrs);
    }
};

TC_ABI(Blacklist, (AddBlackAddress)(DelBlackAddress))
