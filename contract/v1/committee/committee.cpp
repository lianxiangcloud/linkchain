#include "tctpl.hpp"

#include<string>
#include<vector>
#include<set>

#define OPADD       "add_member"
#define OPDELETE    "delete_member"
#define OPAUTHORIZE "account_authorize"

#define CKEY "Committee"
#define PKEY "Proposal"
#define PKEYLIST "Proposal_list"
#define RKEY  "right"

//enum OP {OPADD, OPDELETE,OPAUTHORIZE};
typedef std::string OP;

struct Member {
    tc::Address addr;
};

TC_STRUCT(Member,
    TC_FIELD_NAME(addr, "address"))

struct AuthorizedObject {
    tc::Address address;
    std::string rights;
};

TC_STRUCT(AuthorizedObject,
    TC_FIELD_NAME(address, "address"),
    TC_FIELD_NAME(rights, "rights"))
	
struct Proposal {
    std::string Operation;
    tc::Address Creator;
    std::string Parameters;
    std::set<tc::Address> Committees;
    bool finished; //1 finished
};

TC_STRUCT(Proposal,
    TC_FIELD_NAME(Operation, "Operation"),
    TC_FIELD_NAME(Creator, "Creator"),
	TC_FIELD_NAME(Parameters, "Parameters"),
    TC_FIELD_NAME(Committees, "Committees"),
	TC_FIELD_NAME(finished, "finished")
	);


class Committee : public TCBaseContract {
private:
	bool isMemAddr(const tc::Address& addr){
		auto mem = memberList.get();
		return mem.find(addr) != mem.end();
	}

	const char* createProposal(const OP& op, const std::string& args);

public:
	tc::StorMap<Key<std::string>, Proposal> prop{PKEY};
	tc::StorValue<std::set<tc::Hash>> propList{PKEYLIST};
	tc::StorMap<Key<std::string>, tc::Address> rights{RKEY};
	tc::StorValue<std::set<tc::Address>> memberList{CKEY};
	std::set<std::string> RIGHTS {"validators","candidates","coefficient","pledge"};

public:
	void Init();
	std::string proposaAddMember(const Member& s);
	std::string proposaDeleteMember(const Member& s);
	std::string proposaAccountAuthorize(const AuthorizedObject& s);
	void finishProposal(const std::string& proposalID);
	void voteProposal(const std::string& proposalID);
	std::string execProposal(const std::string& proposalID);
	std::string getProposal(const std::string& proposalID);
	std::string getAllProposalID(void);
	std::string getCommittee();
	tc::Address getRightsAccount(std::string& right);
};

TC_ABI(Committee, (proposaAddMember)\
                (proposaDeleteMember)\
                (proposaAccountAuthorize)\
				(finishProposal)\
                (voteProposal)\
                (execProposal)\
                (getProposal)\
                (getAllProposalID)\
                (getCommittee)\
				(getRightsAccount)\
)
                

void Committee::Init() {
	//Used to be a committee
	tc::Address committee_owner = "0xb6b403be413fff19294e984dfe5964f2cfe7bc15";
	//Used to change pledge contract state and withdraw
	tc::Address pledge_contract = "0x60d4d088ad5cd7f93024eedf8d58a1b226b65138";
	//used to confiscate in pledge contract
	tc::Address account_admin = "0xfd13fb25b38143e50e8226989a8c83652dc77f3e";
	//used to change other inner contract status
	tc::Address inner_contract = "0x0fd0eb798571a75ee2bd655bd9d26a30e49391ba";

	auto mem = memberList.get();
	mem.insert(committee_owner);
	memberList.set(mem);

	rights.set(inner_contract, "validators");
	rights.set(inner_contract, "candidates");
	rights.set(inner_contract, "coefficient");
	rights.set(inner_contract, "blacklist");
	rights.set(inner_contract, "consCommittee");

	rights.set(pledge_contract, "pledge");
	rights.set(account_admin, "pledgeOwner");
}
	
//
std::string Committee::proposaAddMember(const Member& s) {
	std::string memjson = tc::json::Marshal(s);
	return createProposal(OPADD, memjson);
}

//
std::string Committee::proposaDeleteMember(const Member& s) {
	std::string memjson = tc::json::Marshal(s);
	return createProposal(OPDELETE, memjson);
}

//
std::string Committee::proposaAccountAuthorize(const AuthorizedObject& s) {
	TC_RequireWithMsg(RIGHTS.find(s.rights) != RIGHTS.cend(),"error rights");
	
	std::string authjson = tc::json::Marshal(s);
	return createProposal(OPAUTHORIZE, authjson);
}

//
void Committee::finishProposal(const std::string& proposalID) {
	TC_RequireWithMsg(proposalID.size() == 42, "proposalID’size error");
	
	Proposal p = prop.get(proposalID);
	TC_RequireWithMsg(p.Operation != "", "proposalID does not exist");
	TC_RequireWithMsg(tc::App::getInstance()->sender() == p.Creator, "Permission denied: Not Createor");
	p.finished = true;
	prop.set(p, proposalID);
	
}

//
void Committee::voteProposal(const std::string& proposalID) {
	TC_RequireWithMsg(proposalID.size() == 42, "proposalID’size error");
	TC_RequireWithMsg(isMemAddr(tc::App::getInstance()->sender()), "Permission denied");
	Proposal p = prop.get(proposalID);
	TC_RequireWithMsg(p.Operation != "", "proposalID does not exist");
	p.Committees.insert(tc::App::getInstance()->sender());
	prop.set(p, proposalID);
	
}

//
std::string Committee::execProposal(const std::string& proposalID) {
	TC_RequireWithMsg(proposalID.size() == 42, "proposalID’size error");
	TC_RequireWithMsg(isMemAddr(tc::App::getInstance()->sender()), "Permission denied");
	
	auto mem = memberList.get();
	Proposal p = prop.get(proposalID);
	TC_RequireWithMsg(p.Operation != "", "proposalID does not exist");
	std::set<tc::Address> allcommit = memberList.get();
	int num = 0;
	int allnum = allcommit.size();
	for (auto& commit : p.Committees){
		if (allcommit.find(commit) != allcommit.cend()){
			num++;
		}
	}
	
	TC_RequireWithMsg(num*3 >= allnum*2, "not 2/3 vaild votes");
	
	//The same member can be added repeatedly because it does not affect the data.
	if (p.Operation == OPADD){
		Member m;
		tc::json::Unmarshal<Member>(p.Parameters.c_str(), m);
		mem.insert(m.addr);
		memberList.set(mem);
		p.finished = true;
		prop.set(p, proposalID);

	} else if(p.Operation == OPDELETE){
		Member m;
		tc::json::Unmarshal<Member>(p.Parameters.c_str(), m);
		TC_RequireWithMsg(mem.size() != 1, "Only One Committee Now");
		mem.erase(m.addr);
		memberList.set(mem);
		p.finished = true;
		prop.set(p, proposalID);
		
	} else if(p.Operation == OPAUTHORIZE){
		AuthorizedObject a;
		tc::json::Unmarshal<AuthorizedObject>(p.Parameters.c_str(), a);
		TC_RequireWithMsg(RIGHTS.find(a.rights) != RIGHTS.cend(), "error rights");
		rights.set(a.address,a.rights);
		p.finished = true;
		prop.set(p, proposalID);
		
	} else{
		TC_RequireWithMsg(false , "Unknown Operation");
	}
	return "";
}

//
const char* Committee::createProposal(const OP& op, const std::string& args){
	TC_RequireWithMsg(isMemAddr(tc::App::getInstance()->sender()), "Permission denied");

	auto propListC = propList.get();

	Hash porposalID = tc::ripemd160(args + op + tc::App::getInstance()->sender().toString() + i64toa(tc::App::getInstance()->height(), 10));

	Proposal p;
	p.Operation = op;
	p.Creator = tc::App::getInstance()->sender();
	p.Committees.insert(tc::App::getInstance()->sender());
	p.Parameters = args;
	p.finished = false;

	TC_RequireWithMsg(propListC.find(porposalID) == propListC.end(), "Proposal Repeat");
	prop.set(p, porposalID.toString());
	propListC.insert(porposalID);
	propList.set(propListC);
	TC_Log0(porposalID.toString());

	return porposalID.toString();
}

std::string Committee::getProposal(const std::string& proposalID){
	TC_RequireWithMsg(proposalID.size() == 42, "proposalID’size error");

	Proposal p = prop.get(proposalID);
	TC_RequireWithMsg(p.Operation != "", "proposalID does not exist");

	int i=0;
	JsonRoot committee = TC_JsonNewObject();
	for (auto& key : p.Committees){
		TC_JsonPutAddress(committee,itoa(i++),key);
	}

	JsonRoot root = TC_JsonNewObject();
	TC_JsonPutString(root, "Operation", p.Operation.c_str());
	TC_JsonPutString(root, "Creator", p.Creator.toString());
	TC_JsonPutString(root, "Parameters", p.Parameters.c_str());
	TC_JsonPutObject(root, "Committees", committee);
	TC_JsonPutInt(root, "finished", p.finished);
	return TC_JsonToString(root);
}

std::string Committee::getAllProposalID(void) {
	std::set<tc::Hash> keys =  propList.get();
	int i =0;
	JsonRoot root = TC_JsonNewObject();
	for (auto& key : keys){
		TC_JsonPutString(root, itoa(i++), key.toString());
	}
	return TC_JsonToString(root);
}

std::string Committee::getCommittee() {
	auto keys = memberList.get();

	int i =0;
	JsonRoot root = TC_JsonNewObject();
	for (auto& key : keys){
		TC_JsonPutString(root, itoa(i++), key.toString());
	}
	return TC_JsonToString(root);
}

tc::Address Committee::getRightsAccount(std::string& right){
	TC_RequireWithMsg(RIGHTS.find(right) != RIGHTS.cend(),"error rights");
	return rights.get(right);
}
