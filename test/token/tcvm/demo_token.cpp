#include "tctpl.hpp"

class DemoToken : public TCBaseContract
{
    public:
        DemoToken() = default;
        ~DemoToken() = default;

        void Init();
        
        std::string GetDecimals();
        void CallIssue();

}; // end class DemoToken

TC_ABI(DemoToken,
    (GetDecimals)\
    (CallIssue)
)

std::string DemoToken::GetDecimals() {
    return "17";
}

void DemoToken::Init() {
    TC_Issue("100");
}

void DemoToken::CallIssue() {
    TC_Issue("100");
}
