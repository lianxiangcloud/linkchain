package commands

//ContractData used to store the contract data that we need repaly
type ContractData struct {
	sender       string
	contractAddr string
	amount       string
	input        string
}

var contractData = []ContractData{
	/*	test code
		{"0x54fb1c7d0f011dd63b08f85ed7b518ab82028101", "0x0000000000000000000000000000506c65646765", "50000000000000000000",
			`participate|{"0":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028101","1":"50000000000000000000","2":1,"3":90}`},
		{"0x54fb1c7d0f011dd63b08f85ed7b518ab82028100", "0x0000000000000000000000000000506c65646765", "0",
			`setElectorStatus|{"0":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028101","1":3}`},
		{"0x54fb1c7d0f011dd63b08f85ed7b518ab82028102", "0x0000000000000000000000000000506c65646765", "500000000000000000000",
			`deposit|{"0":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028101","1":"500000000000000000000","2":2}`},
		{"0x54fb1c7d0f011dd63b08f85ed7b518ab82028100", "0x0000000000000000000000000000506c65646765", "0",
			`setElectorStatus|{"0":"0x54fb1c7d0f011dd63b08f85ed7b518ab82028101","1":4}`},
	*/
}
