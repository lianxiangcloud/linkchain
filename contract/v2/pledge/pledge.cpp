#include <set>
#include <vector>

#include "tctpl.hpp"

#define ContractCommitteeAddr "0x0000000000000000000000436f6d6d6974746565"
#define ContractFoundationAddr "0x00000000000000000000466f756e646174696f6e"
#define RKEY  "right"

#define E18 "000000000000000000"
#define InitialPledgeAmount "500000" E18
#define WinOutPledgeAmount "5000000" E18
//OneWeek 7*24*3600
#define OneWeek 604800
//OneMonth 30*24*3600
#define OneMonth 2592000

enum ElectorStatus {
    DEFAULT,  
    INITIAL,   
    NOPASS,     // examine Nok
    GOING,      // examine ok and pledge on going
    WINOUT,     
    FAIL,       
    DETAIN,     // disqualify and confiscate deposit
    QUIT       
};

enum Action {
    vote,    
    pledge    
};

enum Ptype {
    change,    
    quit    
};

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
bool operator <(const PledgeRecord& a, const PledgeRecord& b) {
    return a.orderid < b.orderid;
}

struct ElectorInfo {
    tc::BInt totalAmount;
    int status;
    tc::BInt voteCnts;
    uint shareRate; //percent
};
TC_STRUCT(ElectorInfo,
    TC_FIELD_NAME(totalAmount, "totalAmount"),
    TC_FIELD_NAME(status, "status"),
    TC_FIELD_NAME(voteCnts, "voteCnts"),
    TC_FIELD_NAME(shareRate, "shareRate")
	)

const tc::Address GetRightAccount(const std::string& right){
	tc::Address addr;
    StorMap<Key<std::string>, tc::Address> rights(RKEY);
	std::set<std::string> RIGHTS {"validators", "candidates","proposal","coefficient","pledge","pledgeOwner"};
	if (RIGHTS.find(right) != RIGHTS.cend()){

		const tlv::BufferWriter key = Key<std::string>::keyStr(right);
		uint8_t* tmp = rights.getKeyBytes(key);
		uint8_t* value = TC_ContractStoragePureGet(ContractCommitteeAddr, tmp, std::string(RKEY).length() + key.length());
		free(tmp);

		tc::tlv::BufferReader buffer(value);
		unpack(buffer, addr);
    }
	return addr;
}

bool CheckAddrRight(const tc::Address& addr,const std::string& right){
	return addr == GetRightAccount(right);
}


class Pledge : public TCBaseContract {

tc::BInt initialPledgeAmount{InitialPledgeAmount};
tc::BInt winOutPledgeAmount{WinOutPledgeAmount};
public:

    StorMap<Key<tc::Address>, tc::Address> voteMap{"voteMap"};

    StorValue<bool> StopVoteAct{"stopVote"};
    StorValue<bool> StopPledgeAct{"stopPledge"};

    //type 0,after time then can change deposit
    StorValue<uint64> ChangePeriod{"changePeriod"};
    //type 1,after time then can request withdraw
    StorValue<uint64> QuitPeriod{"quitPeriod"};

    //ElectorInfo Map
    //Elector Address          => Info
    StorMap<Key<tc::Address>, ElectorInfo> ElectorsMap{"electorsMap"};

    //OrderId is Exsited?
    //OrderId                        => (bool)
    StorMap<Key<uint64>, bool> orderID{"orderID"};

    //deposit or changeDeposit time
    //OrderId                        => (time)
    StorMap<Key<uint64>, uint64> orderTime{"orderTime"};

    //Elect Address           => set(order)
    StorMap<Key<tc::Address>, std::set<uint64>> pledgeRecordIndex{"recordIndex"};

    //orderid => pledgeRecord
    StorMap<Key<uint64>, PledgeRecord> pledgeRecordInfo{"pledgeRecordInfo"};

    //WinElectorAddr
    StorValue<std::set<tc::Address>> winElectorsAddress{"winaddr"};

    //supportStock
    //K<elector, support> => totalAmount
    StorMap<Key<tc::Address, tc::Address>, tc::BInt> supportStock{"supportStock"};

private:
    void savePledgeRecord(const PledgeRecord& record, const tc::Address& elector){
        std::set<uint64> index = pledgeRecordIndex.get(elector);
        index.insert(record.orderid);
        pledgeRecordIndex.set(index, elector);
        pledgeRecordInfo.set(record, record.orderid);
        orderID.set(true, record.orderid);
    }

public:
    void Init() {    StopPledgeAct.set(false);   StopVoteAct.set(false);  }
	void participate(const tc::Address& elector, const tc::BInt& amount, const uint64& orderid, const uint& shareRate);
	void deposit(const tc::Address& elector, const tc::BInt& amount, const uint64& orderid);
	void vote(const tc::Address& elector);
	void setElectorStatus(const tc::Address& elector, const ElectorStatus& status);
	void setVoteCnts(const tc::Address& elector, const tc::BInt& voteCnts);
	void withDraw(const tc::Address& elector);
	void confiscate(const tc::Address& elector);
    void setAction(Action act, bool stop);
    void setShareRate(const tc::Address& elector, const uint& shareRate);
    void changeDeposit(const tc::Address& electorFrom, const tc::Address& electorTo, const uint64& orderid);
    void requestWithdraw(const tc::Address& elector, const uint64& orderid);
    const char* version(){return "v2.0";}
    void setPeriod(Ptype ptype, const uint64& period);
    const char* getPeriod();
    const char* getDepositTime(const uint64& orderid);

	const char* getDeposit();
	const char* getPledgeRecord(tc::Address& elector);
	ElectorInfo getElectorInfo(tc::Address& elector){return ElectorsMap.get(elector);}
	tc::Address getWhoVote(tc::Address& addr){return voteMap.get(addr);}
};

TC_ABI(Pledge, (participate)(deposit)(vote)(setElectorStatus)(setVoteCnts)(withDraw)\
(confiscate)(setAction)(setShareRate)(getDeposit)(getElectorInfo)(getPledgeRecord)(getWhoVote)\
(changeDeposit)(requestWithdraw)(version)(setPeriod)(getPeriod)(getDepositTime))


void Pledge::participate(const tc::Address& elector, const tc::BInt& amount, const uint64& orderid, const uint& shareRate){
	TC_RequireWithMsg(shareRate <= 100, "share percent is over 100");
	TC_RequireWithMsg(!orderID.get(orderid), "Orderid is exist");
    TC_RequireWithMsg(tc::App::getInstance()->value() >= initialPledgeAmount, "Initial pledge amount should bigger than 500000 ether");
	TC_RequireWithMsg(amount == tc::App::getInstance()->value(), "Value Not Equal Amount");
    TC_RequireWithMsg(StopPledgeAct.get() == false, "pledge Action Stoped");

    ElectorInfo elec = ElectorsMap.get(elector);
    TC_RequireWithMsg(elec.status == ElectorStatus::DEFAULT, "elector is already elected");

    elec.status = ElectorStatus::INITIAL;
    elec.shareRate = shareRate;
    elec.totalAmount = tc::App::getInstance()->value();

    ElectorsMap.set(elec, elector);

    //participate not allow requestWithdraw or change
    orderTime.set(0, orderid);

    PledgeRecord pledgeRecord = PledgeRecord{orderid, tc::App::getInstance()->sender(), tc::App::getInstance()->value(), false};
    savePledgeRecord(pledgeRecord, elector);

    supportStock.set(supportStock.get(elector, tc::App::getInstance()->sender()) + amount, elector, tc::App::getInstance()->sender());

    TC_Log1(tc::json::Marshal(std::make_tuple(tc::App::getInstance()->sender(), amount, orderid)), "Deposit");
}

void Pledge::deposit(const tc::Address& elector, const tc::BInt& amount, const uint64& orderid){
	TC_RequireWithMsg(!orderID.get(orderid), "Orderid is exist");
	TC_Require(amount == tc::App::getInstance()->value());
    TC_RequireWithMsg(StopPledgeAct.get() == false, "pledge Action Stoped");

    ElectorStatus status = (ElectorStatus)ElectorsMap.get(elector).status;
    auto elec = ElectorsMap.get(elector);

    TC_RequireWithMsg(status == ElectorStatus::GOING, "Candidate node status is not on going");
    
    elec.totalAmount = elec.totalAmount + tc::App::getInstance()->value();
    ElectorsMap.set(elec, elector);

    orderTime.set(tc::App::getInstance()->now(), orderid);

    PledgeRecord pledgeRecord = PledgeRecord{orderid, tc::App::getInstance()->sender(), tc::App::getInstance()->value(), false};
    savePledgeRecord(pledgeRecord, elector);

    supportStock.set(supportStock.get(elector, tc::App::getInstance()->sender()) + amount, elector, tc::App::getInstance()->sender());

    TC_Log1(tc::json::Marshal(std::make_tuple(tc::App::getInstance()->sender(), amount, orderid)), "Deposit");
}

void Pledge::changeDeposit(const tc::Address& electorFrom, const tc::Address& electorTo, const uint64& orderid) {
    TC_Payable(false);
    ElectorInfo elecFrom = ElectorsMap.get(electorFrom);
    ElectorInfo elecTo = ElectorsMap.get(electorTo);
    TC_RequireWithMsg(elecFrom.status == ElectorStatus::GOING, "electorFrom status is not on going");
    TC_RequireWithMsg(elecFrom.totalAmount < winOutPledgeAmount, "pledge amount should be smaller than 5000000 ether");
    TC_RequireWithMsg(elecTo.status == ElectorStatus::GOING, "electorTo status is not on going");

    uint64 time = orderTime.get(orderid);
    TC_RequireWithMsg(time > 0, "History or participate deposit not allow changeDeposit");
    uint64 now = tc::App::getInstance()->now();
    uint64 needDelay = ChangePeriod.get();
    if (needDelay == 0){
        needDelay = OneWeek;
    }
    TC_RequireWithMsg((now - time) >= needDelay, "date is less then 7 days");
    orderTime.set(now, orderid);

    std::set<uint64> indexFrom = pledgeRecordIndex.get(electorFrom);
    TC_RequireWithMsg(indexFrom.find(orderid) != indexFrom.end(),"can't find this deposit in electorFrom");
    PledgeRecord record = pledgeRecordInfo.get(orderid);
    TC_RequireWithMsg(tc::App::getInstance()->sender() == record.sender, "Address does not have permission");
    TC_RequireWithMsg(record.hasWithdraw == false,"This deposit has been withdrawn");

    elecFrom.totalAmount = elecFrom.totalAmount - record.amount;
    ElectorsMap.set(elecFrom, electorFrom);
    elecTo.totalAmount = elecTo.totalAmount + record.amount;
    ElectorsMap.set(elecTo, electorTo);

    indexFrom.erase(orderid);
    pledgeRecordIndex.set(indexFrom, electorFrom);
    std::set<uint64> indexTo = pledgeRecordIndex.get(electorTo);
    indexTo.insert(orderid);
    pledgeRecordIndex.set(indexTo, electorTo);

    supportStock.set(supportStock.get(electorFrom, record.sender) - record.amount, electorFrom,  record.sender);
    supportStock.set(supportStock.get(electorTo, record.sender) + record.amount, electorTo, record.sender);
}

void Pledge::vote(const tc::Address& elector) {
    TC_Payable(false);
    TC_RequireWithMsg(ElectorsMap.get(elector).status == ElectorStatus::GOING, 
                    "Elector status is not ElectorStatus.GOING");
    TC_RequireWithMsg(StopVoteAct.get() == false, "Vote Action Stoped");
    voteMap.set(elector, tc::App::getInstance()->sender());
}

void Pledge::setElectorStatus(const tc::Address& elector, const ElectorStatus& setStatus){
    TC_Payable(false);
    TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "pledge"), "Address does not have permission");
    ElectorStatus status = (ElectorStatus)ElectorsMap.get(elector).status;
    if (setStatus == ElectorStatus::NOPASS){
        TC_RequireWithMsg(status == ElectorStatus::INITIAL, "Change status from INITIAL to NOPASS error");
        withDraw(elector);
    }
    auto elec = ElectorsMap.get(elector);

    if (elec.status == ElectorStatus::WINOUT && setStatus != ElectorStatus::WINOUT){
        auto s = winElectorsAddress.get();
        s.erase(elector);
        winElectorsAddress.set(s);
    }

    elec.status = setStatus;

    if (setStatus == ElectorStatus::WINOUT) {
        TC_RequireWithMsg(elec.totalAmount >= winOutPledgeAmount, "pledge amount should bigger than 5000000 ether");
        auto s = winElectorsAddress.get();
        s.insert(elector);
        winElectorsAddress.set(s);
    }
    ElectorsMap.set(elec, elector);
}

void Pledge::setVoteCnts(const tc::Address& elector, const tc::BInt& voteCnts){
    TC_Payable(false);
    TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "pledge"), "Address does not have permission");
    TC_RequireWithMsg(ElectorsMap.get(elector).status == ElectorStatus::GOING, 
    "Elector status is not ElectorStatus.GOING");

    auto elec = ElectorsMap.get(elector);
    elec.voteCnts = voteCnts;
    ElectorsMap.set(elec,elector);
}

void Pledge::withDraw(const tc::Address& elector){
    TC_Payable(false);
    auto elec = ElectorsMap.get(elector);
    TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "pledge"), "Address does not have permission");
    TC_RequireWithMsg(elec.status != ElectorStatus::WINOUT, "elector is WINOUT");
    TC_RequireWithMsg(elec.status != ElectorStatus::DETAIN, "elector is DETAIN");

    int num = 0;        
    std::set<uint64> recordIndex = pledgeRecordIndex.get(elector);
    int i = 0;
    for (const auto& index : recordIndex){
        PledgeRecord record = pledgeRecordInfo.get(index);
        if (!record.hasWithdraw){
            num++;
            elec.totalAmount = elec.totalAmount - record.amount;
            ElectorsMap.set(elec,elector);
            record.hasWithdraw = true;
            TC_Transfer(record.sender.toString(), record.amount.toString());
            savePledgeRecord(record, elector);
        }
        if (num==50){
            break;
        }
    }
}

void Pledge:: requestWithdraw(const tc::Address& elector, const uint64& orderid) {
    TC_Payable(false);
    ElectorInfo elec = ElectorsMap.get(elector);
    TC_RequireWithMsg(elec.status == ElectorStatus::GOING, "elector status is not on going");
    TC_RequireWithMsg(elec.totalAmount < winOutPledgeAmount, "pledge amount should be smaller than 5000000 ether");

    std::set<uint64> index = pledgeRecordIndex.get(elector);
    TC_RequireWithMsg(index.find(orderid) != index.end(),"can't find this deposit in elector");

    uint64 time = orderTime.get(orderid);
    TC_RequireWithMsg(time > 0, "History or primary deposit not allow requestWithdraw");
    uint64 needDelay = QuitPeriod.get();
    if (needDelay == 0){
        needDelay = OneMonth;
    }
    TC_RequireWithMsg((tc::App::getInstance()->now() - time) >= needDelay, "date is less then 30 days");

    index.erase(orderid);
    pledgeRecordIndex.set(index, elector);

    PledgeRecord record = pledgeRecordInfo.get(orderid);
    TC_RequireWithMsg(tc::App::getInstance()->sender() == record.sender, "Address does not have permission");
    TC_RequireWithMsg(record.hasWithdraw == false,"This deposit has been withdrawn");
    record.hasWithdraw = true;
    savePledgeRecord(record, elector);

    elec.totalAmount = elec.totalAmount - record.amount;
    ElectorsMap.set(elec, elector);

    supportStock.set(supportStock.get(elector, record.sender) - record.amount, elector,  record.sender);
    TC_Transfer(record.sender.toString(), record.amount.toString());
}

void Pledge::confiscate(const tc::Address& elector){
    TC_Payable(false);
    TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "pledgeOwner"), "Address does not have permission");
    TC_RequireWithMsg(ElectorsMap.get(elector).status == ElectorStatus::DETAIN,
    "Elector status is not ElectorStatus.DETAIN");

    tc::BInt transferAmount = ElectorsMap.get(elector).totalAmount;
    TC_RequireWithMsg(transferAmount > 0, "Owner withdraw detain elector value error");

    auto elec = ElectorsMap.get(elector);
    elec.totalAmount = 0;
    ElectorsMap.set(elec, elector);
    TC_Transfer(ContractFoundationAddr, TC_GetBalance(tc::App::getInstance()->address().toString()));
}
void Pledge::setAction(Action action, bool isStop){
    TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "pledge"), "Address does not have permission");
    if (action == Action::vote){
        StopVoteAct.set(isStop);
    }
    if (action == Action::pledge){
            StopPledgeAct.set(isStop);
    }
}

void Pledge::setPeriod(Ptype ptype, const uint64& period){
    TC_Payable(false);
    TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "pledge"), "Address does not have permission");
    if (ptype == Ptype::change){
        ChangePeriod.set(period);
    }
    if (ptype == Ptype::quit){
            QuitPeriod.set(period);
    }
}

const char* Pledge::getPeriod(){
    TC_Payable(false);
    JsonRoot root = TC_JsonNewObject();
    uint64 changePeriod = ChangePeriod.get();
    if (changePeriod == 0){
        changePeriod = OneWeek;
    }
    uint64 quitPeriod = QuitPeriod.get();
    if (quitPeriod == 0){
        quitPeriod = OneMonth;
    }
    TC_JsonPutInt64(root, "change", changePeriod);
    TC_JsonPutInt64(root, "quit", quitPeriod);
    return TC_JsonToString(root);
}

const char* Pledge::getDepositTime(const uint64& orderid){
    TC_Payable(false);
    JsonRoot root = TC_JsonNewObject();
    TC_JsonPutInt64(root, "time", orderTime.get(orderid));
    return TC_JsonToString(root);
}

void Pledge::setShareRate(const tc::Address& elector, const uint& shareRate){
    TC_Payable(false);
    auto elec = ElectorsMap.get(elector);
    TC_RequireWithMsg(tc::App::getInstance()->sender() == elector, "Address does not have permission");
    TC_RequireWithMsg(elec.status != DEFAULT, "Elector does not exist");
    TC_RequireWithMsg(shareRate <= 100 && shareRate > elec.shareRate, "shareRate is invalid");

    elec.shareRate = shareRate;
    ElectorsMap.set(elec, elector);
}

const char* Pledge::getDeposit(){
    TC_Payable(false);
    JsonRoot root = TC_JsonNewObject();
    for (auto& a : winElectorsAddress.get()){
        TC_JsonPutString(root, a.toString(), ElectorsMap.get(a).totalAmount.toString());
    }
    return TC_JsonToString(root);
}

const char* Pledge::getPledgeRecord(tc::Address& elector){
    TC_Payable(false);
    JsonRoot root  = TC_JsonNewObject();
    std::set<uint64> recordIndex = pledgeRecordIndex.get(elector);

    int i = 0;
    for (const uint64& index : recordIndex){
        tc::json::PutObject(root, itoa(i++), pledgeRecordInfo.get(index));
    }
    return TC_JsonToString(root);
}
