//foundation used to alloc gas or fine to all candidates according to their deposit
#include <set>
#include <tuple>
#include "tctpl.hpp"

#define RKEY  "right"

#define ContractCommitteeAddr "0x0000000000000000000000436f6d6d6974746565"
#define ContractPledgeAddr "0x0000000000000000000000000000506c65646765"
#define ContractCandidatesAddr "0x0000000000000000000043616e64696461746573"
#define ContractValidatorAddr "0x0000000000000000000056616c696461746f7273"

#define E18 "000000000000000000"
#define AllocMinDeposit "10000" E18

struct Validator {
	std::string pub_key;
	int64 voting_power;
	tc::Address coinbase;
};

TC_STRUCT(Validator,
		TC_FIELD_NAME(pub_key, "pub_key"),
		TC_FIELD_NAME(voting_power, "voting_power"), 
		TC_FIELD_NAME(coinbase, "coinbase"))

struct Candidate { 
	std::string pub_key; 
	int64 voting_power; 
	tc::Address coinbase;
	int64 score;
	int64 PunishHeight;
};

TC_STRUCT(Candidate,
		TC_FIELD_NAME(pub_key, "pub_key"),
		TC_FIELD_NAME(voting_power, "voting_power"),
		TC_FIELD_NAME(coinbase, "coinbase"),
		TC_FIELD_NAME(score, "score"),
		TC_FIELD_NAME(PunishHeight, "punish_height")
		);

struct ElectorInfo {
    tc::BInt totalAmount;
    int status;
    tc::BInt voteCnts;
    uint shareRate; //percent
	tc::Address coinbase;//No serial Not the same as pledge
};

TC_STRUCT(ElectorInfo,
    TC_FIELD_NAME(totalAmount, "totalAmount"),
    TC_FIELD_NAME(status, "status"),
    TC_FIELD_NAME(voteCnts, "voteCnts"),
    TC_FIELD_NAME(shareRate, "shareRate")
	)

struct PledgeRecord {
    uint64 orderid;
    tc::Address sender;
    tc::BInt amount;
    bool hasWithdraw;
};

TC_STRUCT(PledgeRecord,
    TC_FIELD_NAME(orderid, "orderid"),
    TC_FIELD_NAME(sender, "sender"),
	TC_FIELD_NAME(amount, "amount"),
	TC_FIELD_NAME(hasWithdraw, "hasWithdraw")
	)

ElectorInfo GetElectorInfo(const tc::Address& addr){
    ElectorInfo info;

    StorMap<Key<tc::Address>, ElectorInfo> ElectorsMap{"electorsMap"};
    const tlv::BufferWriter key = Key<tc::Address>::keyStr(addr);
    uint8_t* tmp = ElectorsMap.getKeyBytes(key);
    uint8_t* value = TC_ContractStoragePureGet(ContractPledgeAddr, tmp, std::string("electorsMap").length() + key.length());

	if(*value == 0){
		return info;
	}

   	free(tmp);
   	tc::tlv::BufferReader buffer(value);
   	unpack(buffer, info);
   	return info;
}

void GetCandidatesAddr(std::set<tc::Address>& candAddr){
	std::list<std::string> pubkeys;
    uint8_t* buf = TC_ContractStorageGet(ContractCandidatesAddr, "pubkeys");
    tc::tlv::BufferReader buffer((uint8_t*)buf);
    unpack(buffer, pubkeys);

    StorMap<Key<std::string>, std::string> cands{"cand"};
	for (const auto& k : pubkeys){
		Candidate cand;

		std::string candJson;
		const tlv::BufferWriter key = Key<std::string>::keyStr(k);
		uint8_t* tmp = cands.getKeyBytes(key);
		uint8_t* value = TC_ContractStoragePureGet(ContractCandidatesAddr, tmp, std::string("cand").length() + key.length());
		free(tmp);
		tc::tlv::BufferReader buffer(value);
		unpack(buffer, candJson);
		json::Unmarshal(candJson, cand);

		if(cand.score != 0){
			candAddr.insert(cand.coinbase);
		}
	}
}

void GetValidateAddr(std::set<tc::Address>& valAddr){
	std::list<std::string> pubkeys;
    uint8_t* buf = TC_ContractStorageGet(ContractValidatorAddr, "ValidatorList");
    tc::tlv::BufferReader buffer((uint8_t*)buf);
    unpack(buffer, pubkeys);

    StorMap<Key<std::string>, std::string> valInfo{"Validator"};
	for (const auto& k : pubkeys){
		Validator val;

		std::string valJson;
		const tlv::BufferWriter key = Key<std::string>::keyStr(k);
		uint8_t* tmp = valInfo.getKeyBytes(key);
		uint8_t* value = TC_ContractStoragePureGet(ContractValidatorAddr, tmp, std::string("Validator").length() + key.length());
		free(tmp);
		tc::tlv::BufferReader buffer(value);
		unpack(buffer, valJson);
		json::Unmarshal(valJson, val);
		valAddr.insert(val.coinbase);
	}
}

void GetRecordIndex(const tc::Address& addr, std::set<uint64>& index){
    StorMap<Key<tc::Address>, std::set<uint64>> pledgeRecordIndex{"recordIndex"};

    const tlv::BufferWriter key = Key<tc::Address>::keyStr(addr); 
    uint8_t* tmp = pledgeRecordIndex.getKeyBytes(key); 
    uint8_t* value = TC_ContractStoragePureGet(ContractPledgeAddr, tmp, std::string("recordIndex").length() + key.length()); 
    free(tmp);
    tc::tlv::BufferReader buffer(value); 
    unpack(buffer, index); 
}

void GetPledgeRecord(const uint64& index, PledgeRecord& record){
    StorMap<Key<uint64>, PledgeRecord> pledgeRecordInfo{"pledgeRecordInfo"};

    const tlv::BufferWriter key = Key<uint64>::keyStr(index); 
    uint8_t* tmp = pledgeRecordInfo.getKeyBytes(key); 
    uint8_t* value = TC_ContractStoragePureGet(ContractPledgeAddr, tmp, std::string("pledgeRecordInfo").length() + key.length()); 
	if(*value == 0){
		return ;
	}
    free(tmp);
    tc::tlv::BufferReader buffer(value); 
    unpack(buffer, record); 
}

tc::BInt GetSupportStock(const tc::Address& elector, const tc::Address sender){
	tc::BInt stock;
    StorMap<Key<tc::Address, tc::Address>, tc::BInt> supportStock{"supportStock"};

    const tlv::BufferWriter key = Key<tc::Address, tc::Address>::keyStr(elector, sender); 
    uint8_t* tmp = supportStock.getKeyBytes(key); 
    uint8_t* value = TC_ContractStoragePureGet(ContractPledgeAddr, tmp, std::string("supportStock").length() + key.length()); 
    free(tmp);
    tc::tlv::BufferReader buffer(value); 
    unpack(buffer, stock); 
	return stock;
}

void getStockMap(const tc::Address& candidates, std::map<tc::Address, tc::BInt>& stockMap, tc::BInt& supportTotalAmount){

	JsonRoot root  = TC_JsonNewObject();
	std::set<uint64> recordIndex;
	GetRecordIndex(candidates, recordIndex);

	int i = 0;
	for (const auto& index : recordIndex){
	    PledgeRecord record; 
		GetPledgeRecord(index, record);
		tc::BInt amount = GetSupportStock(candidates, record.sender);
		if (amount >= tc::BInt(AllocMinDeposit)){
			if (stockMap[record.sender] != amount){
				stockMap[record.sender] = amount;
				supportTotalAmount += amount;
			}
		}
	}

}

class Foundation : public TCBaseContract {
public:
    void Init(){}
    void allocAward();
	void setPoceeds(const tc::Address& coinbase, const tc::BInt& amount);
	tc::BInt getLastAward(const tc::Address& coinbase, const tc::Address& support);
	std::tuple<tc::BInt, tc::BInt> getCandidateAward(const tc::Address& coinbase);
	const char* getPoceeds();

private:
	StorValue<std::set<tc::Address>> coinbaseList{"coinbase"};
	StorMap<Key<tc::Address>, tc::BInt> awardPool{"award"};

	//record candidate's last reward in block
	//key: candidates, support address, value: award record
	//award = (blockAward + openReward) * stockRate
	StorMap<Key<tc::Address, tc::Address>, tc::BInt> allocRewardRecord{"alloc"};
	StorMap<Key<tc::Address>, std::tuple<tc::BInt,tc::BInt>> candidateReward{"open"};

	void allocCandidatesAward(const ElectorInfo& cand, const tc::BInt& totalAward);
};

TC_ABI(Foundation, (allocAward)(setPoceeds)(getLastAward)(getPoceeds)(getCandidateAward))


void Foundation::allocCandidatesAward(const ElectorInfo& info, const tc::BInt& totalAward){

	TC_Prints("totalAward");
	TC_Prints(totalAward.toString());

	tc::BInt shareAwardTotal = totalAward * tc::BInt(info.shareRate) / tc::BInt(100);
	tc::BInt selfAward = totalAward - shareAwardTotal;

	if (selfAward >= tc::BInt("0")){
		TC_Transfer(info.coinbase.toString(), selfAward.toString());
		allocRewardRecord.set(selfAward, info.coinbase, info.coinbase);
	}

	//get all support stock
	std::map<tc::Address, tc::BInt> stockMap;
	tc::BInt supportTotalAmount;

	getStockMap(info.coinbase, stockMap, supportTotalAmount);

	if (supportTotalAmount <= tc::BInt("0")){
		return;
	}

	//Pay dividends to support
	for (auto& st: stockMap){
		tc::BInt shareAward = shareAwardTotal * st.second/supportTotalAmount;
		if (shareAward >= tc::BInt("0")){
			TC_Transfer(st.first.toString(), shareAward.toString());
			if (info.coinbase == st.first){
				allocRewardRecord.set(shareAward + selfAward, info.coinbase, st.first);
			} else {
				allocRewardRecord.set(shareAward, info.coinbase, st.first);
			}
		}
	}
}

void Foundation::allocAward() {
    TC_RequireWithMsg(tc::App::getInstance()->sender() == tc::Address{}, "Address does not have permission");

    std::set<tc::Address> candAddr;
    std::set<tc::Address> valAddr;
    tc::BInt allDeposit;
	std::list<ElectorInfo> infos;

	//get candidatesAddr
    GetCandidatesAddr(candAddr);

	//get validator
    GetValidateAddr(valAddr);

	if(candAddr.size() == 0){
		return ;
	}

	//Get candidates stock
    for (auto addr : candAddr) {
		auto info = GetElectorInfo(addr);
		info.coinbase = addr;
		infos.push_back(info);
        allDeposit += info.totalAmount;
    }

	if(allDeposit <= tc::BInt("0")){
		return ;
	}

    tc::BInt totalAward = TC_GetBalance(TC_GetSelfAddress());

	std::list<tc::Address> quitList;
	//Get All candidates award
	for (auto addr : coinbaseList.get()) {
		tc::BInt award = awardPool.get(addr);
		//Quit Node
		if (candAddr.find(addr) == candAddr.end() 
				&& valAddr.find(addr) == valAddr.end() 
				&& award > tc::BInt("0")){
			quitList.push_back(addr);
		}

		if (valAddr.find(addr) == valAddr.end() && GetElectorInfo(addr).totalAmount != tc::BInt("0")){
			totalAward -= award;
		}
	}

	tc::BInt openAward = totalAward;
	TC_RequireWithMsg(openAward >= tc::BInt("0"), "openAward less zero");

	//Pay dividends to candidates
	//(selfAward + vaildateAward + foundationLeftBalance)
	//Quit node only have selfAward
	int i = 0;
	for(auto info : infos) {
		if (info.totalAmount == 0){
			candidateReward.set(std::make_tuple(tc::BInt("0"), tc::BInt("0")), info.coinbase);
			continue;
		}
		tc::BInt award = awardPool.get(info.coinbase) + openAward*info.totalAmount/allDeposit;
		candidateReward.set(std::make_tuple(awardPool.get(info.coinbase), openAward*info.totalAmount/allDeposit), info.coinbase);
		allocCandidatesAward(info, award);
		awardPool.set(tc::BInt("0"), info.coinbase);
	}

	//Candidates Quit Node
	//TC_Transfer(candAddr[i].toString(), amount.toString());
	for (const auto& quit : quitList){
		auto info = GetElectorInfo(quit);
		if (info.totalAmount == 0){
			continue;
		}
		info.coinbase = quit;
		tc::BInt totalAward = awardPool.get(info.coinbase);
		allocCandidatesAward(info, totalAward);
		awardPool.set(tc::BInt("0"), info.coinbase);
	}

	//Validator
	for (const auto& val : valAddr){
		awardPool.set(tc::BInt("0"), val);
	}

	//Pocket Money (Very little money)
/*
    tc::BInt remainAward = TC_GetBalance(TC_GetSelfAddress());
	if (remainAward > tc::BInt("0")){
		TC_Transfer((*candAddr.begin()).toString(), remainAward.toString());
	}
*/
}

void Foundation::setPoceeds(const tc::Address& coinbase, const tc::BInt& amount){
    TC_RequireWithMsg(tc::App::getInstance()->sender() == tc::Address{}, "Address does not have permission");

	TC_Prints("setPoceeds:");
	TC_Prints(coinbase.toString());
	TC_Prints(amount.toString());

	if (coinbase == tc::App::getInstance()->address()) {
		//mount += amount;
		return ;
	}

	std::set<tc::Address> addrList = coinbaseList.get();
	addrList.insert(coinbase);

	awardPool.set(amount + awardPool.get(coinbase), coinbase);
	coinbaseList.set(addrList);
}

tc::BInt Foundation::getLastAward(const tc::Address& coinbase, const tc::Address& support){
	TC_Payable(false);
	return allocRewardRecord.get(coinbase, support);
}

std::tuple<tc::BInt, tc::BInt> Foundation::getCandidateAward(const tc::Address& coinbase){
	TC_Payable(false);
	return candidateReward.get(coinbase);
}

const char* Foundation::getPoceeds(){
	TC_Payable(false);
	TC_Prints("getPoceeds:");
	JsonRoot root = TC_JsonNewObject();
	std::set<tc::Address> addrList = coinbaseList.get();
	for (auto& coinbase : addrList){
		TC_JsonPutString(root, coinbase.toString(), awardPool.get(coinbase).toString());
	}
	return TC_JsonToString(root);
}
