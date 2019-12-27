## LRC-10合约标准
### 概述
LRC-10是Linkchain资产发行合约的一种约定标准，开发者可基于LRC-10标准发行资产。
基于LRC-10发行的资产token，可以作为链上资产流通进行隐私交易。

### 使用场景
<待补充>

### 合约接口规范

#### Name
可选，返回token的名字 如： "LRCToken"
```C++
tc::string Name();
```

#### Symbol
可选，返回token的简称 如："LRC"
```C++
tc::string Symbol();
```
#### Decimals
可选，返回token精度
```C++
uint32_t Decimals();
```
#### TotalSupply
可选 返回token发行总数，如果多次发行，需要在合约代码中累加发行量。
```C++
tc::string TotalSupply();
```

### token相关函数

#### TC_TransferToken
token转账
```C++
void TC_TransferToken(const char* addr, const char* token, const char* amount);
```

- `addr` token转入地址
- `token` 转账的token类型
- `amount` 转账数量
token为0x0000000000000000000000000000000000000000时,等价于TC_Transfer，原生token转账

#### TC_Issue
发行token
```C++
void TC_Issue(const char* amount);
```
- `amount` 发行token总量

#### TC_TokenBalance
token余额查询
```C++
char* TC_TokenBalance(const address addr, const address token);
```
- `addr`查询地址
- `token` token类型
- 返回值 token数量

token为0x0000000000000000000000000000000000000000,等价于TC_GetBalance，查询链克余额

#### TC_TokenAddress
```C++
const char* TC_TokenAddress(void);
```
查询msg中token类型


### 资产发行合约例子
- 个人发行 将所有发行的token转入单个地址
- 兑换发行 资产发行后，用户转入链克，来兑换新发行的token

#### 个人发行
发行资产后，将全部token转入给单个地址
```C++
#include "tcmethod.hpp"//声明合约头文件

//发行时总量设置 1000*100000000
#define E8 "00000000"
#define AMOUNT "1000"
#define TOTALSUPPLY AMOUNT E8

class LRCToken : public TCBaseContract{ //TCBaseContract合约基类
public:
    //合约初始化函数，当合约部署时会自动调用
    void Init(){
        //发行token，链上记账，此时balance[合约地址]=totalsupply
        TC_Issue(TOTALSUPPLY);
        transferALL();
    }
    
    //可选的合约接口
    //名字
    tc::string name = {"LRCToken"};
    tc::string Name(){
        return name;
    }
    
    //可选的合约接口
    //精度
    uint32_t decimals = 8;
    uint32_t Decimals(){
        return decimals;
    }
    //可选的合约接口
    //token简称
    tc::string Symbol(){
        return "LRC";
    }

    //可选的合约接口
    //发行总量 注意:如果发行多次，此处返回为多次发行量之和
    tc::string TotalSupply(){
        return TOTALSUPPLY;
    }

private:
    //初始化时将所有token发送给指定账户
    void transferALL(){
        //可选,可配置，初始化时将token发送给指定账户
        tc::string AdminAddress= {"0x54fb1c7d0f011dd63b08f85ed7b518ab82028100"};
        TC_TransferToken(AdminAddress.c_str(), TC_GetSelfAddress(), TOTALSUPPLY);
    }
};
TC_ABI(LRCToken, (Name)(Decimals)(Symbol)(TotalSupply))    //TC_ABI声明合约外部接口
```

#### 资产兑换发行
发行资产后，外部调用合约转入链克，按比例兑换token
```C++
#include "tcmethod.hpp"//声明合约头文件

//发行时总量设置 1000*100000000
#define E8 "00000000"
#define AMOUNT "1000"
#define TOTALSUPPLY AMOUNT E8

class EXToken : public TCBaseContract{ //TCBaseContract合约基类
public:
    //合约初始化函数，当合约部署时会自动调用
    void Init(){
        //发行token，链上记账，此时balance[合约地址]=totalsupply
        TC_Issue(TOTALSUPPLY);
    }
    
    //可选的合约接口
    //名字
    tc::string name = {"EXToken"};
    tc::string Name(){
        return name;
    }
    
    //可选的合约接口
    //精度
    uint32_t decimals = 8;
    uint32_t Decimals(){
        return decimals;
    }
    
    //可选的合约接口
    //发行总量 注意:如果发行多次，此处返回为多次发行量之和
    tc::string TotalSupply(){
        return TOTALSUPPLY;
    }
    
    //兑换Token
    void Exchange(){
        bool bResult = false;
        TC_Payable(true);
        // 判断支付的必须是链克
        TC_RequireWithMsg(0 == strcmp(TC_TokenAddress(), ZERO_ADDRESS),"Token is not LinkToken"); 
        // 计算可买到的token数量,此处为1:1，不需要换算

        // 将合约账户的token转给msg.sender
        TC_TransferToken(TC_GetMsgSender(), TC_GetSelfAddress(), TC_GetMsgValue());
    }
    
    //管理人员提取链克
    void WithDraw(){
        //判断管理人地址
        TC_Payable(false);
        tc::Address senderAddr = tc::App::getInstance()->sender();
        TC_RequireWithMsg(senderAddr ==  AdminAddress, "Sender is not Admin");
        
        //获取转入合约的链克总量
        char* balance = TC_GetBalance(TC_GetSelfAddress());
        
        //管理员提取全部链克
        TC_TransferToken(senderAddr.toString(), TC_GetSelfAddress(), balance);
    }
private:
    const tc::Address AdminAddress = {"0x54fb1c7d0f011dd63b08f85ed7b518ab82028100"};
};
TC_ABI(EXToken, (Name)(Decimals)(TotalSupply)(Exchange)(WithDraw))    //TC_ABI声明合约外部接口
```
