// Kiki
#include "tctpl.hpp"

#define RKEY "right"
#define ContractCommitteeAddr "0x0000000000000000000000436f6d6d6974746565"

#define E18 "000000000000000000"
#define AllocMinDeposit "100000" E18

enum {
    FirstMake = 0,				// Default status, Used in DepositIn
    ToBeExamine = 1,			// The deposit has been paid, waiting for examination
    ExamineFailed = 2,			// Examine failed
    ExamineSucceeded = 3,		// Examine succeed, became a member of the committee
    CommitQuit = 4,				// Commit request for refund and quit committee
    LeftCommittee = 5,			// The address status used to be "ExamineFailed" or "CommitQuit", LeftCommittee after DepositBack
};

struct AddressInfo {
    uint status = FirstMake;				// User status, default "FirstMake"
    tc::BInt deposit = tc::BInt("0");       // Deposit amount, default "0"
    std::string date = "00000000";          // Date of last InterestPayment
};

TC_STRUCT(AddressInfo,
    TC_FIELD_NAME(status, "status"),
    TC_FIELD_NAME(deposit, "deposit"),
    TC_FIELD_NAME(date, "date")
);

class ConstructionCommittee : public TCBaseContract{                // TCBaseContract
private:
    std::string version = "20190930";
public:
    tc::StorValue<std::set<tc::Address>> AddressSet{"AddressSet"};
    tc::StorMap<Key<tc::Address>, AddressInfo>  AddressInfoMap{"AddressInfoMap"};
    tc::StorValue<tc::BInt> totalAmountOfBonus{"totalAmountOfBonus"};
    
    void Init();
    void DepositIn(const tc::Address& address);
    void Examine(const tc::Address& address, const uint status);
    void DepositBack(const tc::Address& address, const tc::BInt& amount);
    void Recharge(void);
    void DailyInterestPayment(const tc::Address& address, const tc::BInt& amount, const std::string date);
    const char* QueryByAddress(const tc::Address& address);
    const char* Query(void);
    const char* BonusAmount(void);
    const std::string GetVersion(void);
};
TC_ABI(ConstructionCommittee,(DepositIn)\
                    (Examine)\
                    (DepositBack)\
                    (Recharge)\
                    (DailyInterestPayment)\
                    (BonusAmount)\
                    (QueryByAddress)\
                    (Query)\
                    (GetVersion))		                // TC_ABI 

// Init: init
void ConstructionCommittee::Init() {
    totalAmountOfBonus.set(tc::BInt{"0"});
}

// GetRightAccount: Get the address of the right
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

// CheckAddrRight: Check the address's right
bool CheckAddrRight(const tc::Address& addr,const std::string& right){
    return addr == GetRightAccount(right);
}

// DepositIn: Deposit in for address 
void ConstructionCommittee::DepositIn(const tc::Address& address) {
    TC_Payable(true);
    tc::BInt amount = tc::App::getInstance()->value();
    TC_RequireWithMsg(amount >= BInt(AllocMinDeposit), "ConstructionCommittee DepositIn amount < 100000 link!");

    AddressInfo getAddressInfo = AddressInfoMap.get(address);
    TC_RequireWithMsg(getAddressInfo.status == FirstMake, "Address has been used!");
    
    getAddressInfo.status = ToBeExamine;
    getAddressInfo.deposit = amount;
    AddressInfoMap.set(getAddressInfo, address);
    
    std::set<tc::Address> addressSet = AddressSet.get();
    addressSet.insert(address);
    AddressSet.set(addressSet);
    TC_Log0(tc::json::Marshal(std::make_tuple(tc::App::getInstance()->sender(), address, amount)));
}

// Examine: Called by the Admin. Modify the status of the address.
void ConstructionCommittee::Examine(const tc::Address& address, const uint status) {
    TC_Payable(false);
    TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "consCommittee"), "Address does not have permission!");
    
    AddressInfo getAddressInfo = AddressInfoMap.get(address);
    switch (status) {
        case ExamineFailed:
            TC_RequireWithMsg((getAddressInfo.status == ToBeExamine || getAddressInfo.status == ExamineSucceeded), "Status error, input status is ExamineFailed!");
            break;
        case ExamineSucceeded:
            TC_RequireWithMsg((getAddressInfo.status == ToBeExamine || getAddressInfo.status == ExamineFailed), "Status error, input status is ExamineSucceeded!");
            break;
        case CommitQuit:
            TC_RequireWithMsg((getAddressInfo.status == ExamineSucceeded), "Status error, input status is CommitQuit!");
            break;
        default:
            TC_RequireWithMsg(false, "Status error, invalid status input!");
    }
    getAddressInfo.status = status;
    AddressInfoMap.set(getAddressInfo, address);
}

// DepositBack: Called by the Admin. Return the deposit to the address.
void ConstructionCommittee::DepositBack(const tc::Address& address, const tc::BInt& amount) {
    TC_Payable(false);
    TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "consCommittee"), "Address does not have permission!");
    TC_RequireWithMsg(amount >= tc::BInt{"0"}, "Amount < 0!");

    AddressInfo getAddressInfo = AddressInfoMap.get(address);
    TC_RequireWithMsg((getAddressInfo.status == CommitQuit || getAddressInfo.status == ExamineFailed), "Address status error!");
    TC_RequireWithMsg(getAddressInfo.deposit > amount, "Amount illegal!");

    getAddressInfo.deposit = tc::BInt("0");
    getAddressInfo.status = LeftCommittee;
    AddressInfoMap.set(getAddressInfo, address);
    TC_Transfer(address.toString(), amount.toString());
    TC_Log0(tc::json::Marshal(std::make_tuple(address, amount)));
}

// Recharge: Add the amount of bonus
void ConstructionCommittee::Recharge() {
    TC_Payable(true);
	
    tc::BInt taob = totalAmountOfBonus.get();
    taob += tc::App::getInstance()->value();
    totalAmountOfBonus.set(taob);
}

// DailyInterestPayment: Called by the Admin. Interest payment.
void ConstructionCommittee::DailyInterestPayment(const tc::Address& address, const tc::BInt& amount, const std::string date) {
    TC_Payable(false);
    TC_RequireWithMsg(CheckAddrRight(tc::App::getInstance()->sender(), "consCommittee"), "Address does not have permission!");
    TC_RequireWithMsg(date.size() == 8, "Date error!");
    for (int i = 0; i < 8; ++i) { if (date[i] < '0' || date[i] > '9') TC_RequireWithMsg(false, "Date error!");}
    TC_RequireWithMsg(amount >= tc::BInt{"0"}, "Amount < 0!");
    tc::BInt taob = totalAmountOfBonus.get();
    TC_RequireWithMsg(taob >= amount, "Lack of bonus!");
	
    AddressInfo getAddressInfo = AddressInfoMap.get(address);
    TC_RequireWithMsg(getAddressInfo.status == ExamineSucceeded, "Address status is not ExamineSucceeded!");
    TC_RequireWithMsg(date != getAddressInfo.date, "Address has got its payment today!");
	
    getAddressInfo.date = date;
    AddressInfoMap.set(getAddressInfo, address);
    taob -= amount;
    totalAmountOfBonus.set(taob);
    TC_Transfer(address.toString(), amount.toString());
    TC_Log0(tc::json::Marshal(std::make_tuple(address, amount)));
}

// Query: Query the address info map
const char* ConstructionCommittee::Query() {
    TC_Payable(false);
    std::set<tc::Address> addressSet = AddressSet.get();
    int i = 0;
    JsonRoot root = TC_JsonNewObject();
    for(auto& address : addressSet) {
        AddressInfo getAddressInfo = AddressInfoMap.get(address);
        TC_JsonPutString(root, address.toString(), tc::json::Marshal(getAddressInfo));
    }
    return TC_JsonToString(root);
}

// DepositQuery: Query the AddressInfo of the address
const char* ConstructionCommittee::QueryByAddress(const tc::Address& address) {
    TC_Payable(false);
    AddressInfo getAddressInfo = AddressInfoMap.get(address);
    return tc::json::Marshal(getAddressInfo);
}

// BonusAmount: Query the amount of bonus
const char* ConstructionCommittee::BonusAmount() {
    TC_Payable(false);
    return totalAmountOfBonus.get().toString();
}

// 
const std::string ConstructionCommittee::GetVersion() {
    TC_Payable(false);
    return version;
}
