package ser

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/common"
)

type mapEncode struct {
	ID     int64
	Name   string
	Books  map[[20]byte]*big.Int
	Tokens map[common.Address]*big.Int
	Desc   string
}

type mapEncodeUnsupport1 struct {
	ID    int64
	Name  string
	Books map[[19]byte]*big.Int
	Desc  string
}

type mapEncodeUnsupport2 struct {
	ID     int64
	Name   string
	Tokens map[common.Address]big.Int
	Desc   string
}

type mapEncodeUnsupport3 struct {
	ID     int64
	Name   string
	Tokens map[common.Address]int
	Desc   string
}

type mapEncTest struct {
	val           interface{}
	output, error string
}

var books = map[[20]byte]*big.Int{
	[20]byte{0}: big.NewInt(100),
	[20]byte{1}: big.NewInt(200),
}
var booksUnsupport = map[[19]byte]*big.Int{
	[19]byte{3}: big.NewInt(100),
	[19]byte{4}: big.NewInt(200),
}
var tokens = map[common.Address]*big.Int{
	common.HexToAddress("0xed69f6aedf597e09c128d1965de78883a46c8ca2"): big.NewInt(300),
	common.HexToAddress("0xb3f5439003514546190d78f14ddeac75274ab7eb"): big.NewInt(400),
}
var tokensUnsupport1 = map[common.Address]big.Int{
	common.HexToAddress("0xed69f6aedf597e09c128d1965de78883a46c8ca2"): *big.NewInt(300),
	common.HexToAddress("0xb3f5439003514546190d78f14ddeac75274ab7eb"): *big.NewInt(400),
}
var tokensUnsupport2 = map[common.Address]int{
	common.HexToAddress("0xed69f6aedf597e09c128d1965de78883a46c8ca2"): 300,
	common.HexToAddress("0xb3f5439003514546190d78f14ddeac75274ab7eb"): 400,
}

var mapEncTests = []mapEncTest{
	{
		val: mapEncode{
			ID:     1,
			Name:   "skyle",
			Books:  books,
			Tokens: tokens,
			Desc:   "this is a test",
		},
		output: "F8773185736B796C65EE329400000000000000000000000000000000000000006494010000000000000000000000000000000000000081C8F13294B3F5439003514546190D78F14DDEAC75274AB7EB82019094ED69F6AEDF597E09C128D1965DE78883A46C8CA282012C8E7468697320697320612074657374",
		error:  "",
	},
	{
		val: mapEncodeUnsupport1{
			ID:    2,
			Name:  "skyle",
			Books: booksUnsupport,
			Desc:  "this is a test",
		},
		output: "",
		error:  "rlp: type map[[19]uint8]*big.Int is not RLP-decode",
	},
	{
		val: mapEncodeUnsupport2{
			ID:     3,
			Name:   "skyle",
			Tokens: tokensUnsupport1,
			Desc:   "this is a test",
		},
		output: "",
		error:  "rlp: type map[common.Address]big.Int is not RLP-decode",
	},
	{
		val: mapEncodeUnsupport3{
			ID:     4,
			Name:   "skyle",
			Tokens: tokensUnsupport2,
			Desc:   "this is a test",
		},
		output: "",
		error:  "rlp: type map[common.Address]int is not RLP-decode",
	},
}

func runMapEncTests(t *testing.T, f func(val interface{}) ([]byte, error)) {
	for i, test := range mapEncTests {
		output, err := f(test.val)
		if err != nil && test.error == "" {
			t.Errorf("test %d: unexpected error: %v\nvalue %#v\ntype %T",
				i, err, test.val, test.val)
			continue
		}
		if test.error != "" && fmt.Sprint(err) != test.error {
			t.Errorf("test %d: error mismatch\ngot   %v\nwant  %v\nvalue %#v\ntype  %T",
				i, err, test.error, test.val, test.val)
			continue
		}
		if err == nil && !bytes.Equal(output, unhex(test.output)) {
			t.Errorf("test %d: output mismatch:\ngot   %X\nwant  %s\nvalue %#v\ntype  %T",
				i, output, test.output, test.val, test.val)
		}
	}
}

func TestMapEncode1(t *testing.T) {
	runMapEncTests(t, func(val interface{}) ([]byte, error) {
		b := new(bytes.Buffer)
		err := Encode(b, val)
		return b.Bytes(), err
	})
}

type mapDecTest struct {
	input string
	ptr   interface{}
	value interface{}
	error string
}

var mapDecTests = []mapDecTest{
	{
		input: "F8773185736B796C65EE329400000000000000000000000000000000000000006494010000000000000000000000000000000000000081C8F13294B3F5439003514546190D78F14DDEAC75274AB7EB82019094ED69F6AEDF597E09C128D1965DE78883A46C8CA282012C8E7468697320697320612074657374",
		ptr:   new(mapEncode),
		value: mapEncode{
			ID:     1,
			Name:   "skyle",
			Books:  books,
			Tokens: tokens,
			Desc:   "this is a test",
		},
		error: "",
	},
}

func runMapDecTests(t *testing.T, decode func([]byte, interface{}) error) {
	for i, test := range mapDecTests {
		input, err := hex.DecodeString(test.input)
		if err != nil {
			t.Errorf("test %d: invalid hex input %q", i, test.input)
			continue
		}
		err = decode(input, test.ptr)
		if err != nil && test.error == "" {
			t.Errorf("test %d: unexpected Decode error: %v\ndecoding into %T\ninput %q",
				i, err, test.ptr, test.input)
			continue
		}
		if test.error != "" && fmt.Sprint(err) != test.error {
			t.Errorf("test %d: Decode error mismatch\ngot  %v\nwant %v\ndecoding into %T\ninput %q",
				i, err, test.error, test.ptr, test.input)
			continue
		}
		fmt.Printf("origin: %v\n", test.value)
		fmt.Printf("decode: %v\n", test.ptr)
		deref := reflect.ValueOf(test.ptr).Elem().Interface()
		if err == nil && !reflect.DeepEqual(deref, test.value) {
			t.Errorf("test %d: value mismatch\ngot  %#v\nwant %#v\ndecoding into %T\ninput %q",
				i, deref, test.value, test.ptr, test.input)
		}
	}
}

func TestMapDecode(t *testing.T) {
	runMapDecTests(t, func(input []byte, into interface{}) error {
		return Decode(bytes.NewReader(input), into)
	})
}
