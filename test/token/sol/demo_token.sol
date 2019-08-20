pragma solidity ^0.4.19;

contract DemoToken {
    uint256 public _totalSupply;
    uint8 public decimals = 18;

    event Transfer(address from, address to, address token, uint256 value);

    constructor() public {
        _totalSupply = 10000 * 10 ** uint256(decimals);
        issue(_totalSupply);
        emit Transfer(address(0),this, this, _totalSupply);
    }

    function name() public pure returns (string) {
        return "DemoToken";
    }

    function exchangebylk() public payable {
        require(msg.value > 0);
        uint256 changeValue = msg.value*2;
        msg.sender.transfertoken(this, changeValue);
        emit Transfer(msg.sender, this, address(0), msg.value);
        emit Transfer(this,msg.sender, this, changeValue);
    }

    function exchangebytoken() public payable {
        require(msg.tokenvalue > 0);
        msg.sender.transfertoken(this, msg.tokenvalue);
        emit Transfer(msg.sender, this, msg.tokenaddress, msg.tokenvalue);
        emit Transfer(this,msg.sender, this, msg.tokenvalue);
    }

    function balanceOf(address owner) public view returns (uint256) {
        return owner.balancetoken(this);
    }

    function addOrder(uint256 value) public payable {
        require(msg.tokenaddress == address(this));
        require(msg.tokenvalue == value && value > 0);
        
        // ...

        emit Transfer(msg.sender, this, msg.tokenaddress, msg.tokenvalue);
    }
    
    //fallback
    function () {
    }
}
