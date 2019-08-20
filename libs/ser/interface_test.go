package ser

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

type human interface {
	ID() string
}

type boy struct {
	Age   int
	Name  string
	Hobby string
}

func (b *boy) MarshalAmino() ([]byte, error) {
	fmt.Printf("go in boy.MarshalAmino()\n")
	return EncodeToBytes(b)
}

func (b *boy) UnmarshalAmino(bz []byte) error {
	fmt.Printf("go in boy.UnmarshalAmino()\n")
	return DecodeBytes(bz, b)
}

func (boy *boy) ID() string {
	return "boy"
}

type girl struct {
	Age   int
	Name  string
	Hobby string
}

func (g *girl) DeepCopy() *girl {
	return &girl{
		Age:   18,
		Name:  "girl",
		Hobby: "baba",
	}
}

func (girl *girl) ID() string {
	return "girl"
}

type father struct {
	Kid  human
	Age  int
	Name string
}

//Abandoned
func TestRLPEncodeUnregistedInterface(t *testing.T) {
	gl := &girl{
		Age:   18,
		Name:  "LiLi",
		Hobby: "song",
	}
	fa := &father{
		Kid:  gl,
		Age:  40,
		Name: "Jack",
	}
	var buf = new(bytes.Buffer)
	err := Encode(buf, fa)
	if err != nil && err.Error() != "Cannot encode unregistered concrete type ser.girl" {
		t.Errorf("RLPEncode err: %v", err)
	}
}

func TestRLPEncodeInterface(t *testing.T) {
	gl := &girl{
		Age:   18,
		Name:  "LiLi",
		Hobby: "song",
	}
	fa := &father{
		Kid:  gl,
		Age:  40,
		Name: "Jack",
	}
	//RegisterConcrete(&girl{}, "ser/girl", nil)
	var buf = new(bytes.Buffer)
	err := Encode(buf, fa)

	if err != nil {
		t.Errorf("RLPEncode err: %v", err)
	}
	fmt.Printf("RLPEncode: %x\n", buf.Bytes())
	//ddf2dcae7f875eb9cd823132844c694c6984736f6e67823238844a61636b
}

func TestRLPEncodeInterface1(t *testing.T) {
	gl := &girl{
		Age:   18,
		Name:  "LiLi",
		Hobby: "song",
	}
	var h human
	h = gl
	//RegisterConcrete(&girl{}, "ser/girl", nil)
	var buf = new(bytes.Buffer)
	err := Encode(buf, h)
	if err != nil {
		t.Errorf("RLPEncode err: %v", err)
	}
	fmt.Printf("RLPEncode: %x\n", buf.Bytes())
	//f2dcae7f875eb9cd823132844c694c6984736f6e67
}

func TestRLPDecodeUnregisterInterface(t *testing.T) {
	bz, _ := hex.DecodeString("ddf2dcae7f875eb9cd823132844c694c6984736f6e67823238844a61636b")
	var buf = bytes.NewBuffer(bz)
	var fa father

	err := Decode(buf, &fa)
	if err != nil && err.Error() != "unrecognized disambiguation+prefix bytes 4B362553B7B519" {
		t.Errorf("RLPDecode err: %v", err)
	}
}

func TestRLPDecodeInterface(t *testing.T) {
	//RegisterConcrete(&girl{}, "ser/girl", nil)
	bz, _ := hex.DecodeString("ddf2dcae7f875eb9cd823132844c694c6984736f6e67823238844a61636b")
	var buf = bytes.NewBuffer(bz)
	var fa father

	err := Decode(buf, &fa)
	if err != nil {
		t.Errorf("RLPDecode err: %v", err)
	}
	fmt.Printf("RLPDecode: %v\n", fa)
	fmt.Printf("RLPDecode: %v\n", fa.Kid)
}

func TestRLPDecodeInterface1(t *testing.T) {
	//RegisterConcrete(&girl{}, "ser/girl", nil)
	bz, _ := hex.DecodeString("f2dcae7f875eb9cd823132844c694c6984736f6e67")
	var buf = bytes.NewBuffer(bz)
	var h human

	err := Decode(buf, &h)
	if err != nil {
		t.Errorf("RLPDecode err: %v", err)
	}
	fmt.Printf("RLPDecode: %v\n", h)
}

func TestRLPDecodeInterface2(t *testing.T) {
	//RegisterConcrete(&girl{}, "ser/girl", nil)
	bz, _ := hex.DecodeString("ddf2dcae7f875eb9cd823132844c694c6984736f6e67823238844a61636b")
	var buf = bytes.NewBuffer(bz)
	var fa father

	n, err := DecodeReader(buf, &fa, 30)
	if err != nil {
		t.Errorf("RLPDecode err: %v", err)
	}
	fmt.Printf("RLPDecode: %d\n", n)
	fmt.Printf("RLPDecode: %v\n", fa)
	fmt.Printf("RLPDecode: %v\n", fa.Kid)
}

func TestMarshalJSON(t *testing.T) {
	gl := &girl{
		Age:   18,
		Name:  "LiLi",
		Hobby: "song",
	}
	fa := &father{
		Kid:  gl,
		Age:  40,
		Name: "Jack",
	}
	RegisterInterface((*human)(nil), nil)
	bz, err := MarshalJSON(fa)
	if err != nil {
		t.Errorf("MarshalJSON err: %v", err)
	}
	fmt.Printf("MarshalJSON: %s\n", bz)
}

func TestUnmarshalJSON(t *testing.T) {
	jsonStr := `{"Kid":{"type":"ser/girl","value":{"Age":"18","Name":"LiLi","Hobby":"song"}},"Age":"40","Name":"Jack"}`
	RegisterInterface((*human)(nil), nil)
	var fa father
	err := UnmarshalJSON([]byte(jsonStr), &fa)
	if err != nil {
		t.Errorf("UnmarshalJSON err: %v", err)
	}
	fmt.Printf("UnmarshalJSON: %v\n", fa)
}

type Record struct {
	A   []string
	Ctx []interface{}
	D   string
}
type V struct {
	S string
}

func TestSlice(t *testing.T) {
	RegisterInterface((*interface{})(nil), nil)
	RegisterConcrete(&V{}, "v", nil)
	v := &Record{
		A: []string{"a", "b"},
		Ctx: []interface{}{
			//interface{}(nil),
			V{S: "s"},
			interface{}(nil),
			//interface{}(nil),
		},
		D: "d",
	}
	fmt.Printf("%#v\n", v)
	bs, err := EncodeToBytes(v)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Encode: %x\n", bs)

	var vRLP = new(Record)
	if err := DecodeBytes(bs, &vRLP); err != nil {
		panic(err)
	}
	fmt.Printf("Decode: %#v\n", vRLP)

	bs, err = EncodeToBytes(v)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Encode: %x\n", bs)
}

func TestMarshalJSONIndent(t *testing.T) {
	RegisterInterface((*interface{})(nil), nil)
	RegisterConcrete(&V{}, "v", nil)
	v := &Record{
		A: []string{"a", "b"},
		Ctx: []interface{}{
			//interface{}(nil),
			V{S: "s"},
			interface{}(nil),
			//interface{}(nil),
		},
		D: "d",
	}
	jsonStr, err := MarshalJSONIndent(v, "", "	")
	if err != nil {
		panic(err)
	}
	fmt.Printf("MarshalJSONIndent ret: %s\n", jsonStr)
}

func TestPrintTypes(t *testing.T) {
	err := PrintTypes(os.Stdout)
	if err != nil {
		panic(err)
	}
}

func TestEncodeByteSlice(t *testing.T) {
	sl := []byte("this is a test")
	buf := new(bytes.Buffer)
	err := EncodeByteSlice(buf, sl)
	if err != nil {
		panic(err)
	}
	fmt.Printf("EncodeByteSlice ret: %x\n", buf.Bytes())
}

func TestEncodeUvarint(t *testing.T) {
	i := uint64(100)
	buf := new(bytes.Buffer)
	err := EncodeUvarint(buf, i)
	if err != nil {
		panic(err)
	}
	fmt.Printf("EncodeUvarint ret: %x\n", buf.Bytes())
}

func TestDeepCopy(t *testing.T) {
	RegisterInterface((*interface{})(nil), nil)
	RegisterConcrete(&V{}, "v", nil)
	v := &Record{
		A: []string{"a", "b"},
		Ctx: []interface{}{
			//interface{}(nil),
			V{S: "s"},
			interface{}(nil),
			//interface{}(nil),
		},
		D: "d",
	}
	d := DeepCopy(v)
	fmt.Printf("%#v\n", v)
	fmt.Printf("%#v\n", d)
}

func TestDeepCopy1(t *testing.T) {
	gl := &girl{
		Age:   18,
		Name:  "LiLi",
		Hobby: "song",
	}
	g2 := DeepCopy(gl)
	fmt.Printf("%#v\n", gl)
	fmt.Printf("%#v\n", g2)
}

func TestDeepCopy2(t *testing.T) {
	by := &boy{
		Age:   18,
		Name:  "jack",
		Hobby: "dance",
	}
	b2 := DeepCopy(by)
	fmt.Printf("%#v\n", by)
	fmt.Printf("%#v\n", b2)
}
