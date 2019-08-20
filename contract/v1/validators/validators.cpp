#include "tctpl.hpp"
#include "ctype.h"

#include<string>
#include<set>
#include<vector>

#define CKEY "Validator"
#define CKEYList "ValidatorList"
#define ContractCommitteeAddr "0x0000000000000000000000436f6d6d6974746565"
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

    std::string checkValidator(const Validator& c) {
        if (!c.coinbase.isHex()){
            return "illegal coinbase";
        }
        if ( c.voting_power < 0 ){
            return "illegal votingPower";
        }
        if (!isPubKeyHex(c.pub_key)){
            return "illegal PubKey";
        }
        return "";
    }

    void addValidator(tc::StorMap<Key<std::string>, std::string>& cand,std::set<std::string>& keys,const Validator& val){
        std::string valjson = tc::json::Marshal(val);
        cand.set(valjson, val.pub_key);
        keys.insert(val.pub_key);
    }

public:
	//init
    void Init(){
        Validator v1,v2,v3;
        tc::StorMap<Key<std::string>, std::string> cand(CKEY);
        tc::StorValue<std::set<std::string>> pubkeys(CKEYList);
		std::set<std::string> keys =  pubkeys.get();

/*
        //Use true address instead
        v1.pub_key="0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac71111";
        v1.voting_power=10;
        v1.coinbase=tc::Address("0x54fb1c7d0f011dd63b08f85ed7b518ab82028100");

        v2.pub_key="0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac72222";
        v2.voting_power=10;
        v2.coinbase=tc::Address("0x54fb1c7d0f011dd63b08f85ed7b518ab82028100");

        v3.pub_key="0x724c2517228e6aa0244cd7b37466aa824822b5c735518d2019c2a25d238ab70e7bfc4bbdeac73333";
        v3.voting_power=10;
        v3.coinbase=tc::Address("0x54fb1c7d0f011dd63b08f85ed7b518ab82028100");

        addValidator(cand,keys,v1);
        addValidator(cand,keys,v2);
        addValidator(cand,keys,v3);
*/
        pubkeys.set(keys);
	}

    //add or update a validator
    std::string SetValidator(const Validator& c) {
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "validators"), "Address does not have permission");
        std::string ret = checkValidator(c);
        if ("" !=ret ){
            return ret;
        }

        tc::StorMap<Key<std::string>, std::string> cand(CKEY);
        std::string Candjson = tc::json::Marshal(c);
        cand.set(Candjson, c.pub_key);

        tc::StorValue<std::set<std::string>> pubkeys(CKEYList);
		std::set<std::string> keys =  pubkeys.get();

        if(keys.find(c.pub_key)!=keys.end()){
            return "";
        }

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

        tc::StorMap<Key<std::string>, std::string> cand(CKEY);
        cand.set("", s);

        tc::StorValue<std::set<std::string>> pubkeys(CKEYList);
		std::set<std::string> keys =  pubkeys.get();
        keys.erase(s);
        pubkeys.set(keys);

        return "";
	}

    //get all validators
	std::string GetAllValidators() {
		tc::StorMap<Key<std::string>, std::string> cand(CKEY);
		tc::StorValue<std::set<std::string>> pubkeys(CKEYList);
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
