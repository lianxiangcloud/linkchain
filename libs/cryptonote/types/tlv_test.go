package types

import (
	"fmt"
	"testing"
)

type SliceComForTest struct {
	EcdhInfo []EcdhTuple
	value    uint8
}

func (k *SliceComForTest) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 1)
	ret[0x0001] = NewSliceSerializer(0, &(k.EcdhInfo))
	ret[0x0002] = NewBasiceSerializer(&(k.value))
	return ret
}
func (k *SliceComForTest) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *SliceComForTest) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *SliceComForTest) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *SliceComForTest) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}
func TestSliceSerializerDEncode(t *testing.T) {

	src := &SliceComForTest{
		EcdhInfo: []EcdhTuple{EcdhTuple{
			Mask: Key{1, 2,},
		},
			EcdhTuple{
				Mask: Key{1, 2,},
			},
			EcdhTuple{
				Mask: Key{1, 2,},
			}, EcdhTuple{
				Mask: Key{1, 2,},
			}, EcdhTuple{
				Mask: Key{1, 2,},
			}, EcdhTuple{
				Mask: Key{1, 2,},
			}, EcdhTuple{
				Mask: Key{1, 2,},
			}, EcdhTuple{
				Mask: Key{255, 2,},
			}, EcdhTuple{
				Mask: Key{34, 2,},
			}, EcdhTuple{
				Mask: Key{34, 2,},
			}, EcdhTuple{
				Mask: Key{1, 5,},
			}},
		value: 123,
	}
	data := make([]byte, src.TlvSize())
	//fmt.Printf("datalen=%v\n", len(data))
	if _, err := src.TlvEncode(data); err != nil {
		t.Fatal(err.Error())
	}
	//fmt.Printf("encode finish=%v\n", data)
	to := &SliceComForTest{}
	err := to.TlvDecode(data)
	if err != nil {
		t.Fatal(err.Error())
	}
	if src.value != src.value || len(src.EcdhInfo) != len(to.EcdhInfo) {
		t.Fatalf("TestRctConfigDEncode fail expected byte=%v got=%v", src, to)
	}
	for i := 0; i < len(src.EcdhInfo); i++ {
		if !src.EcdhInfo[i].Mask.IsEqual(&(to.EcdhInfo[i].Mask)) || !src.EcdhInfo[i].SenderPK.IsEqual(&(to.EcdhInfo[i].SenderPK)) || !src.EcdhInfo[i].Amount.IsEqual(&(to.EcdhInfo[i].Amount)) {
			t.Fatalf("TestRctConfigDEncode fail expected byte=%v got=%v", src, to)
		}
	}
}
func TestTlvMapSerializer(t *testing.T) {
	var a int32 = -1
	var b int32
	tag1 := defualtTestRctsig()
	tag2 := NewBasiceSerializer(&a)
	if tag2 == nil {
		t.Fatalf("TestTlvMapSerializer fail")
	}
	from := NewTlvMapSerializerWith(tag1, NewBasiceSerializer(&a))
	data := make([]byte, from.TlvSize())
	if _, err := from.TlvEncode(data); err != nil {
		fmt.Printf("datalen=%v\n", len(data))
		t.Fatal(err.Error())
	}
	var toSig = RctSig{}
	to := NewTlvMapSerializerWith(&toSig, NewBasiceSerializer(&b))
	err := to.TlvDecode(data)
	if err != nil {
		fmt.Printf("TlvDecode datalen=%v\n", len(data))
		t.Fatal(err.Error())
	}
	if false == IsEqualJsonForTest(&toSig, tag1) || b != a {
		t.Fatalf("TestTlvMapSerializer fail a=%v b=%v\n", a, b)
	}
}
