#include "tctpl.hpp"
#include "ctype.h"

#include<string>
#include<set>
#include<vector>

#define RKEY  "right"

#define ContractCommitteeAddr "0x0000000000000000000000436f6d6d6974746565"
#define ContractPledgeAddr "0x0000000000000000000000000000506c65646765"


struct Candidate {
	std::string pub_key;
	uint64 voting_power;
	tc::Address coinbase;
	uint64 score;
    uint64 PunishHeight;
};

TC_STRUCT(Candidate,
    TC_FIELD_NAME(pub_key, "pub_key"),
    TC_FIELD_NAME(voting_power, "voting_power"),
	TC_FIELD_NAME(coinbase, "coinbase"),
	TC_FIELD_NAME(score, "score"),
    TC_FIELD_NAME(PunishHeight, "punish_height")
	);

const tc::Address GetRightAccount(const std::string& right){
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

bool CheckAddrRight(const tc::Address& addr, const std::string& right){
	return addr == GetRightAccount(right);
}

bool IsWinOutAccount(const tc::Address& addr){
    std::set<tc::Address> winOutAddr;
    uint8_t* buf = TC_ContractStorageGet(ContractPledgeAddr, "winaddr");
    TC_RequireWithMsg(*buf != 0,  "WinOut Addr is empty");
    tc::tlv::BufferReader buffer((uint8_t*)buf);
    unpack(buffer, winOutAddr);
    return winOutAddr.find(addr) != winOutAddr.end();
}

class Candidates : public TCBaseContract {

private:

    bool isPubKeyHex(const std::string& s) {
        int size = s.size();
        if (size != 82) {
            return false;
        } 
        if (s[0]!='0'||(s[1]!='x'&&s[1]!='X')){
            return false;
        }
        for(int i = 2; i < size; i++){
            if(!isxdigit(s[i]))
                return false;
        }
        return true;
    }

    void checkCandidate(const Candidate& c) {
        TC_RequireWithMsg(c.coinbase.isHex(),  "illegal coinbase");
		TC_RequireWithMsg(isPubKeyHex(c.pub_key), "illegal PubKey");
		TC_RequireWithMsg(IsWinOutAccount(c.coinbase), "Coinbase is not Winout Account");
    }

public:
    tc::StorMap<Key<std::string>, std::string> cand{"cand"};
    tc::StorValue<std::set<std::string>> pubkeys{"pubkeys"};
    tc::StorValue<std::set<tc::Address>> coinbases{"coinbase"};

	//init
    void Init(){}

    //add or update a candidate
    std::string SetCandidate(const Candidate& c) {
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "candidates"), "Address does not have permission");
		checkCandidate(c);

        std::string Candjson = tc::json::Marshal(c);
		std::set<std::string> keys = pubkeys.get();
		std::set<tc::Address> coinbaseAddrs = coinbases.get();

        //Add candidate
        if(keys.find(c.pub_key) == keys.end()){
            if(coinbaseAddrs.find(c.coinbase) != coinbaseAddrs.end()){
                TC_RequireWithMsg(false, "Coinbase repeat");
            }
        } else {
        //update candidate
            Candidate candiInfo;
            tc::json::Unmarshal(cand.get(c.pub_key), candiInfo);
            if(c.coinbase != candiInfo.coinbase){
                if(coinbaseAddrs.find(c.coinbase) != coinbaseAddrs.end()){
                    TC_RequireWithMsg(false, "Coinbase repeat");
                }
            	coinbaseAddrs.erase(candiInfo.coinbase);
	    }
        }

        cand.set(Candjson, c.pub_key);
        keys.insert(c.pub_key);
        coinbaseAddrs.insert(c.coinbase);

        pubkeys.set(keys);
        coinbases.set(coinbaseAddrs);
        return "";
	}
	
	//delete a candidate
    std::string DeleteCandidate(std::string& s) {
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "candidates"), "Address does not have permission");
		std::set<std::string> keys = pubkeys.get();
		TC_RequireWithMsg(keys.find(s) != keys.end(), "Candidates does not exist");
        keys.erase(s);
        pubkeys.set(keys);

        Candidate candiInfo;
        tc::json::Unmarshal(cand.get(s), candiInfo);

		std::set<tc::Address> coinbase = coinbases.get();
        coinbase.erase(candiInfo.coinbase);
        coinbases.set(coinbase);

        cand.set("", s);
        return "";
	}

    //get all candidates
	std::string GetAllCandidates() {
		auto keys = pubkeys.get();
		int i =0;

		JsonRoot root = TC_JsonNewObject();
		for (auto& key : keys){
			TC_JsonPutString(root, itoa(i++), cand.get(key).c_str());
		}
		return TC_JsonToString(root);
	}
};

TC_ABI(Candidates, (SetCandidate)\
					(DeleteCandidate)\
					(GetAllCandidates)\
);
