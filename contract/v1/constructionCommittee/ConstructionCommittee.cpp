#include "tctpl.hpp"

struct AddressInfo {
    bool isMember = false;      // effective member of the committee or not
    bool firstMake = true;      // the first generation or not
    int status = 0;             // 0:to-be-examine 1:examine-failed 2:examine-succeeded 3:left-committee
    tc::BInt deposit = 0;       // deposit amount
};

TC_STRUCT(AddressInfo,
    TC_FIELD_NAME(isMember, "isMember"),
    TC_FIELD_NAME(firstMake, "firstMake"),
    TC_FIELD_NAME(status, "status"),
    TC_FIELD_NAME(deposit, "deposit")
);

class ConstructionCommittee : public TCBaseContract{                // TCBaseContract
public:
    tc::StorMap<Key<tc::Address>, AddressInfo>  AddressInfoMap{"AddressInfoMap"};
    
    void DepositIn(const tc::Address& address, const tc::BInt& amount);
    void Examine(const tc::Address& address, const int status);
    void DepositBack(const tc::Address& address, const tc::BInt& amount);
    void DepositOut(const tc::Address& address, const tc::BInt& amount);
    const char* DepositQuery(const tc::Address& address);
};
TC_ABI(ConstructionCommittee,   (DepositIn)\
                    (Examine)\
                    (DepositBack)\
                    (DepositOut)\
                    (DepositQuery))		                // TC_ABI 

// DepositIn: 
void ConstructionCommittee::DepositIn(const tc::Address& address, const tc::BInt& amount) {
    TC_Payable(true);
    TC_RequireWithMsg(amount >= BInt(100000), "ConstructionCommittee DepositIn amount < 100000!");

    AddressInfo getAddressInfo = AddressInfoMap.get(address);
    if(getAddressInfo.firstMake == false) {
        TC_RequireWithMsg(false, "ConstructionCommittee DepositIn amount <= 100000!");
    } else getAddressInfo.firstMake = false;

    TC_RequireWithMsg(tc::App::getInstance()->value() == amount, "Tx value < amount!");
    getAddressInfo.deposit = amount;
    AddressInfoMap.set(getAddressInfo, address);
}

// Examine: Called by the Owner to modify the user audit status
void ConstructionCommittee::Examine(const tc::Address& address, const int status) {
    TC_Payable(false);
    TC_RequireWithMsg(tc::App::getInstance()->sender() == tc::Address{"xxx"}, "Address does not have permission!");
    
    AddressInfo getAddressInfo = AddressInfoMap.get(address);
    getAddressInfo.status = status;
    if(status == 2) getAddressInfo.isMember = true;
    AddressInfoMap.set(getAddressInfo, address);
}

// DepositBack: Return the deposit to the address
void ConstructionCommittee::DepositBack(const tc::Address& address, const tc::BInt& amount) {
    TC_Payable(true);
    TC_RequireWithMsg(tc::App::getInstance()->sender() == tc::Address{"xxx"}, "Address does not have permission");

    AddressInfo getAddressInfo = AddressInfoMap.get(address);
    getAddressInfo.isMember = false;
    getAddressInfo.deposit = 0;
    getAddressInfo.status = 3;
    AddressInfoMap.set(getAddressInfo, address);
}

// DepositOut: Return the deposit to the address, differ from DepositBack in the future
void ConstructionCommittee::DepositOut(const tc::Address& address, const tc::BInt& amount) {
    TC_Payable(true);
    TC_RequireWithMsg(tc::App::getInstance()->sender() == tc::Address{"xxx"}, "Address does not have permission");
    
    AddressInfo getAddressInfo = AddressInfoMap.get(address);
    getAddressInfo.isMember = false;
    getAddressInfo.deposit = 0;
    getAddressInfo.status = 3;
    AddressInfoMap.set(getAddressInfo, address);
}

// DepositQuery: Query the AddressInfo of the address
const char* ConstructionCommittee::DepositQuery(const tc::Address& address) {
    TC_Payable(false);
    AddressInfo getAddressInfo = AddressInfoMap.get(address);
	return tc::json::Marshal(getAddressInfo);
}
