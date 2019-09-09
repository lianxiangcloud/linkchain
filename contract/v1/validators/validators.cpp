#include "tctpl.hpp"
#include "ctype.h"

#include<string>
#include<set>
#include<vector>

#define CKEY "Validator"
#define CKEYList "ValidatorList"
#define ContractCommitteeAddr "0x0000000000000000000000436f6d6d6974746565"
#define ContractCandidatesAddr "0x0000000000000000000043616e64696461746573"
#define RKEY  "right"

struct Validator {
	std::string pub_key;
	int64 voting_power;
	tc::Address coinbase;
};

TC_STRUCT(Validator,
    TC_FIELD_NAME(pub_key, "pub_key"),
    TC_FIELD_NAME(voting_power, "voting_power"),
	TC_FIELD_NAME(coinbase, "coinbase"))


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

bool CheckAddrRight(const tc::Address& addr,const std::string& right){
	return addr == GetRightAccount(right);
}

bool IsRepeatPubkey(const std::string& pubkey){
	std::set<std::string> pubkeys;
	uint8_t* buf = TC_ContractStorageGet(ContractCandidatesAddr, "pubkeys");
	tc::tlv::BufferReader buffer((uint8_t*)buf);
	if (*buf == 0){
		return false;
	}

	unpack(buffer, pubkeys);

	if (pubkeys.find(pubkey) != pubkeys.end()){
		return true;
	} else {
		return false;
	}
}


class Validators : public TCBaseContract {

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

    void checkValidator(const Validator& c) {
		TC_RequireWithMsg(c.coinbase.isHex(),  "illegal coinbase");
		TC_RequireWithMsg(isPubKeyHex(c.pub_key), "illegal PubKey");
		TC_RequireWithMsg(c.voting_power >= 0, "illegal votingPower");
		TC_RequireWithMsg(!IsRepeatPubkey(c.pub_key), "Pubkey is Repeat(candidate)");

    }

    void addValidator(tc::StorMap<Key<std::string>, std::string>& cand,std::set<std::string>& keys,const Validator& val){
        std::string valjson = tc::json::Marshal(val);
        cand.set(valjson, val.pub_key);
        keys.insert(val.pub_key);
    }

	tc::StorMap<Key<std::string>, std::string> cand{CKEY};
	tc::StorValue<std::set<std::string>> pubkeys{CKEYList};
public:
	//init
    void Init(){
        const int valsNum =10;
        Validator val[valsNum];
		std::set<std::string> keys =  pubkeys.get();

        std::string pubKeys[10]={
            "0x724c2517228e6aa00d698c5d9acc07a6d728b9ff208b2d69dbf6cce6da05900e811fd81dd9a4f9f5",
            "0x724c2517228e6aa06998e02e3964070cab7ef990ce0369459a4abf882387a873f2254327822aef2a",
            "0x724c2517228e6aa0eb08dae3754116c1286978423075088ad55eb92ca580f29e09ea31ca5edb7798",
            "0x724c2517228e6aa020f87b284b6a8e313317bbb8c094738392d4a208843c870f2041f3774d01fbd3",
            "0x724c2517228e6aa028882466ad0af7bef9c0894d7ab7eaa8f67688a7532f4b4e334c2397c3664a6b",
            "0x724c2517228e6aa0fba2d4ca1bf3826b4f60d3e490a06aa84cd8984b1116e78f63a125972a16fb21",
            "0x724c2517228e6aa03b80019fae3b716687c80a1e03292c4f805359f70a69e67d8aefb03205f51906",
            "0x724c2517228e6aa03fd09cea1acffae3c1ceb69da550f1fa90b35021d318a15697e3a16793c27814",
            "0x724c2517228e6aa06390bf1cef91cbf671e325d50adde4097933bd5956b4fbf1e709558b307b78ec",
            "0x724c2517228e6aa017722c13d2b5ab2167a93979bee1cb031949ec6b93e3cd182e9e3a8b50abaade"
        };

        for(int i=0;i<valsNum;i++){
            val[i].pub_key=pubKeys[i];
            val[i].voting_power=10;
            val[i].coinbase=tc::Address("0x00000000000000000000466f756e646174696f6e");
            addValidator(cand,keys,val[i]);
        }
        pubkeys.set(keys);
	}

    //add or update a validator
    std::string SetValidator(const Validator& c) {
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "validators"), "Address does not have permission");
        checkValidator(c);

        std::string Candjson = tc::json::Marshal(c);
        cand.set(Candjson, c.pub_key);

		std::set<std::string> keys =  pubkeys.get();
        keys.insert(c.pub_key);
        pubkeys.set(keys);
        return "";
	}
	
	//delete a validator
    std::string DeleteValidator(std::string& s) {
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "validators"), "Address does not have permission");
        if (!isPubKeyHex(s)){
            return "illegal PubKey";
        }
        cand.set("", s);

		std::set<std::string> keys =  pubkeys.get();
        keys.erase(s);
        pubkeys.set(keys);
        return "";
	}

    //get all validators
	std::string GetAllValidators() {
		auto keys = pubkeys.get();
		int i =0;

		JsonRoot root = TC_JsonNewObject();
		for (auto& key : keys){
			TC_JsonPutString(root, itoa(i++), cand.get(key).c_str());
		}
		return TC_JsonToString(root);
	}
};

TC_ABI(Validators, (SetValidator)\
					(DeleteValidator)\
					(GetAllValidators)\
);
