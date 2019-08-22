//coefficient used to define some parameters
#include "tctpl.hpp"
#include "tcapi.h"

#include<string>

#define CKey "Coefficient"
#define RKEY  "right"

//VoteRate decide how many candidates we will choose
//total num * Nume/Deno and no more than UpperLimit
struct VoteRate {
	int Deno;   //Denominator
	int Nume;   //molecule
	int UpperLimit;
};

TC_STRUCT(VoteRate,
    TC_FIELD_NAME(Deno, "Deno"),
    TC_FIELD_NAME(Nume, "Nume"),
    TC_FIELD_NAME(UpperLimit, "UpperLimit"));

//CalRate decide the rate when calculate the candidate's rankresult
struct CalRate {
	int64 Srate; //score rate
	int64 Drate; //deposit rate
	int64 Rrate; //randnum rate
};

TC_STRUCT(CalRate,
    TC_FIELD_NAME(Srate, "Srate"),
    TC_FIELD_NAME(Drate, "Drate"),
    TC_FIELD_NAME(Rrate, "Rrate"));

//Coefficient some coefficient which may be changed
struct coefficient {
    VoteRate    voteRate;
	CalRate     calRate;
	int64      VotePeriod;
	int64       MaxScore;
	tc::BInt      UTXOFee;
};

TC_STRUCT(coefficient,
    TC_FIELD_NAME(voteRate, "voteRate"),
    TC_FIELD_NAME(calRate, "calRate"),
    TC_FIELD_NAME(VotePeriod, "VotePeriod"),
    TC_FIELD_NAME(MaxScore, "MaxScore"),
    TC_FIELD_NAME(UTXOFee, "UTXOFee"));


#define ContractCommitteeAddr "0x0000000000000000000000436f6d6d6974746565"

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


class Coefficient : public TCBaseContract {
public:

    void Init(){
        coefficient co;
        co.VotePeriod =1321;
        co.voteRate.Deno = 5;
        co.voteRate.Nume =3;
        co.voteRate.UpperLimit =12;
        co.calRate.Srate =4;
        co.calRate.Drate =4;
        co.calRate.Rrate =2;
        co.MaxScore =500;
        co.UTXOFee = tc::BInt("500000");

        std::string cojson = tc::json::Marshal(co);

        tc::StorValue<std::string> coStore(CKey);
        coStore.set(cojson);
    };

    std::string updateVoteRate(const VoteRate& vr){
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "coefficient"), "Address does not have permission");
		TC_RequireWithMsg(vr.Deno > 0, "VoteRate.Deno must be greater than 0");

        coefficient co;
        tc::StorValue<std::string> coStore(CKey);
        std::string cojson = coStore.get();
        
        tc::json::Unmarshal<coefficient>(cojson.c_str(), co);
        co.voteRate = vr;

        cojson = tc::json::Marshal(co);
        coStore.set(cojson);
        return "";
    };

    std::string updateCalRate(const CalRate& cr){
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "coefficient"), "Address does not have permission");
        coefficient co;
        tc::StorValue<std::string> coStore(CKey);
        std::string cojson = coStore.get();
        
        tc::json::Unmarshal<coefficient>(cojson.c_str(), co);
        co.calRate = cr;

        cojson = tc::json::Marshal(co);
        coStore.set(cojson);
        return "";
    };

    std::string updateVotePeriod(const int64& vp){
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "coefficient"), "Address does not have permission");
		TC_RequireWithMsg(vp > 0, "VotePeriod must be greater than 0");

        coefficient co;
        tc::StorValue<std::string> coStore(CKey);
        std::string cojson = coStore.get();
        
        tc::json::Unmarshal<coefficient>(cojson.c_str(), co);
        co.VotePeriod = vp;

        cojson = tc::json::Marshal(co);
        coStore.set(cojson);
        return "";
    };

    std::string updateMaxScore(const int64& ms){
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "coefficient"), "Address does not have permission");
		TC_RequireWithMsg(ms > 0, "MaxScore must be greater than 0");

        coefficient co;
        tc::StorValue<std::string> coStore(CKey);
        std::string cojson = coStore.get();
        
        tc::json::Unmarshal<coefficient>(cojson.c_str(), co);
        co.MaxScore = ms;

        cojson = tc::json::Marshal(co);
        coStore.set(cojson);
        return "";
    };

    std::string updateUTXOFee(const tc::BInt& uf){
        TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "coefficient"), "Address does not have permission");
        coefficient co;
        tc::StorValue<std::string> coStore(CKey);
        std::string cojson = coStore.get();
        
        tc::json::Unmarshal<coefficient>(cojson.c_str(), co);
        co.UTXOFee = uf;

        cojson = tc::json::Marshal(co);
        coStore.set(cojson);
        return "";
    };

    std::string getCoefficient(){
        tc::StorValue<std::string> coStore(CKey);
        return coStore.get();
    };
};

TC_ABI(Coefficient,(updateVoteRate)\
                (updateCalRate)\
                (updateVotePeriod)\
                (updateMaxScore)\
                (updateUTXOFee)\
                (getCoefficient)\
)
