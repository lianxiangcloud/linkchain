package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"unsafe"
)
/**
TLV serialization protocal
1)tlv normal format:
 tlv_format=tag + datalen + rawdata|tlv_format

|--2bytes--|--2bytes--|----datalen bytes--------|
|    tag   | datalen  |  rawdata/tlv_format     |

2) array/slice format
 tlv_format = [tlv_format,tlv_format,.......]

special:
 array/slice with const size element:
      tlv_format whitout tag field and without datalen field
 array/slice with uncertain size element:
      tag fied is unused
3）rawdata use bigEndian for basic type

datalen max value is (1<<16)-1, so just only support 65535 bytes rawdata.

 */
var (
	TAGSIZE   = 2
	LENSIZE   = 2
	TAGOFFSET = 0
	LENOFFSET = TAGSIZE
	HEADSIZE  = TAGSIZE + LENSIZE
	VOFFSET   = HEADSIZE

	IvalidCodeERR = errors.New("invalid code")

	EncodeSpaceERR = errors.New("unexpected encode space")
	NilEncoderERR  = errors.New("encoder is nil")
	EncodeTypeERR  = errors.New("unexpected encode type")

	DecodeSpaceERR = errors.New("unexpected decode space")

	NotSerializableTypeERR = errors.New("just support Serializable type")
)

type Tlvtag int

type Serializable interface {
	TlvEncode([]byte) (int, error)
	TlvDecode([]byte) (error)
	TlvSize() int
}
type FieldSerializable interface {
	TlvSize() int
	SerializableConfig() map[Tlvtag]interface{}
	DeSerializableConfig() map[Tlvtag]interface{}
}
type TlvMapSerializer struct {
	smap map[Tlvtag]Serializable
}

func (tms *TlvMapSerializer) TlvSize() int {
	ret := 0
	for _, item := range tms.smap {
		ret += item.TlvSize()+HEADSIZE
	}
	return ret
}
func (tms *TlvMapSerializer) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, tms.smap)
}
func (tms *TlvMapSerializer) TlvDecode(data []byte) (error) {
	return TlvDecodeFromMap(data, tms.smap)
}
func (tms *TlvMapSerializer) SetTagAndSerializer(tag Tlvtag, serializer Serializable) {
	tms.smap[tag] = serializer
}
func NewTlvMapSerializer()*TlvMapSerializer  {
	return &TlvMapSerializer{
		smap:make(map[Tlvtag]Serializable),
	}
}
func NewTlvMapSerializerWith(serializer ...Serializable)*TlvMapSerializer  {
	ret := &TlvMapSerializer{
		smap:make(map[Tlvtag]Serializable),
	}
	for index,s := range serializer{
		ret.SetTagAndSerializer(Tlvtag(index+1),s)
	}
	return ret
}
/**
	elementSize=0,
	elementSize > 0,Each element has same length with elementSize,tag and len is unnecessary
 */
func TlvEncodeFromSlice(data []byte, sSlice []Serializable, elementSize int) (int, error) {

	offset := 0
	datalen := len(data)
	if datalen == 0 || len(sSlice) == 0 {
		return 0, nil
	}
	var err error
	for index, item := range sSlice {
		inc := 0
		if item == nil {
			return 0, NilEncoderERR
		}
		if offset > datalen {
			return 0, EncodeSpaceERR
		}
		if elementSize <= 0 {
			inc, err = item.TlvEncode(data[offset+HEADSIZE:])
		} else {
			inc, err = item.TlvEncode(data[offset:])

		}

		if err != nil {
			return 0, err
		}
		//	fmt.Printf("inc=%v offset=%v datalen=%v %v\n",inc,offset,datalen,elementSize)
		if elementSize > 0 {
			if inc != elementSize {
				return 0, fmt.Errorf("unexpected elementSize want=%v got=%v\n", elementSize, inc)
			}
			//fmt.Printf("-->offset=%v inc=%v datalen=%v\n",offset,inc,datalen)
			offset += inc
		} else {
			//	fmt.Printf("offset=%v inc=%v datalen=%v\n",offset,inc,datalen)
			TagTo2Byte(Tlvtag(index), data[offset:offset+TAGSIZE])
			LenTo2Byte(inc, data[offset+LENOFFSET:offset+LENOFFSET+LENSIZE])
			offset += inc + HEADSIZE
		}
	}
	return offset, nil
}
func TlvDecodeFromSlice(data []byte, sSlice []Serializable, elementSize int) (error) {
	offset := 0
	datalen := len(data)
	slen := len(sSlice)
	if datalen == 0 || slen == 0 {
		return nil
	}
	index := 0
	for ; offset < datalen; index++ {

		if index >= slen {
			return fmt.Errorf("unexpected slice lenght want=%v got=%v", slen, index)
		}
		if elementSize <= 0 {
			_, inc, err := ParseTagAndLen(data[offset:])
			if err != nil {
				return err
			}
			sSlice[index].TlvDecode(data[offset+VOFFSET : offset+VOFFSET+inc])
			offset += VOFFSET + inc
		} else {
			inc := elementSize
			sSlice[index].TlvDecode(data[offset : offset+inc])
			offset += inc
		}
	}
	if offset != datalen {
		return DecodeSpaceERR
	}
	if index != slen {
		return fmt.Errorf("unexpected slice lenght want=%v got=%v", slen, index)
	}
	return nil
}
func DecodeSliceSize(elementSize int, data []byte) (int, error) {
	datalen := len(data)
	if datalen == 0 {
		return 0, nil
	}
	if elementSize > 0 {
		if datalen%elementSize != 0 {
			return 0, fmt.Errorf("invalid datalen elementSize=%v datalen=%v", elementSize, datalen)
		}
		return datalen / elementSize, nil
	}
	offset := 0

	ret := 0
	for offset < datalen {
		_, inc, err := ParseTagAndLen(data)
		if err != nil {
			return 0, err
		}
		ret++
		offset += inc + HEADSIZE
	}
	return ret, nil
}
func TlvEncodeFromMap(data []byte, smap map[Tlvtag]Serializable) (int, error) {
	size := 0
	offset := 0
	datalen := len(data)
	for tag, item := range smap {

		if item == nil {
			return 0, NilEncoderERR
		}
		if offset >= datalen {
			return 0, EncodeSpaceERR
		}
		//fmt.Printf("Encode tag=%v\n",tag)
		inc, err := item.TlvEncode(data[offset+HEADSIZE:])
		if err != nil {
			return 0, err
		}
		TagTo2Byte(tag, data[offset:offset+TAGSIZE])
		LenTo2Byte(inc, data[offset+LENOFFSET:offset+LENOFFSET+LENSIZE])
		size += inc + HEADSIZE
		offset += inc + HEADSIZE
	}
	return size, nil
}
func TlvDecodeFromMap(data []byte, smap map[Tlvtag]Serializable) (error) {
	offset := 0
	datalen := len(data)
	for offset < datalen {
		tag, inc, err := ParseTagAndLen(data[offset:])
		if err != nil {
			return err
		}
		if item, ok := smap[tag]; ok {
			if offset+VOFFSET+inc > datalen {
				return DecodeSpaceERR
			}
			err = item.TlvDecode(data[offset+VOFFSET : offset+VOFFSET+inc])
			if err != nil {
				return err
			}
		}
		offset += inc + HEADSIZE
	}
	return nil
}
func ParseTagAndLen(data []byte) (tag Tlvtag, dlen int, err error) {
	if len(data) < TAGSIZE+LENSIZE {
		return 0, 0, IvalidCodeERR
	}
	return Tlvtag(int(data[0]) + (int(data[1]) << 8)), int(data[2]) + (int(data[3]) << 8), nil
}
func LenTo2Byte(datalen int, data []byte) {
	data[0] = byte(datalen & ((1 << 8) - 1))
	data[1] = byte((datalen >> 8) & ((1 << 8) - 1))
}

func TagTo2Byte(tag Tlvtag, data []byte) {
	data[0] = byte(tag & ((1 << 8) - 1))
	data[1] = byte((tag >> 8) & ((1 << 8) - 1))
}

type BasiceSerializer struct {
	target interface{}
	size   int
}

func NewBasiceSerializer(target interface{}) (*BasiceSerializer) {
	datasize := BasicDataSize(target)
	if datasize == 0 {
		return nil
	}
	return &BasiceSerializer{
		target: target,
		size:   datasize,
	}
}

func (e *BasiceSerializer) TlvSize() int {
	return e.size
}
func (e *BasiceSerializer) TlvEncode(data []byte) (int, error) {
	datalen := len(data)
	if datalen < e.TlvSize() {
		//fmt.Printf("-->%v %v\n", datalen, e.TlvSize())
		return 0, EncodeSpaceERR
	}
	s1 := make([]byte, 0)
	buf := bytes.NewBuffer(s1)
	binary.Write(buf, binary.BigEndian, e.target)
	bufbyte := buf.Bytes()
	if len(bufbyte) != e.TlvSize() {
		return 0, fmt.Errorf("unexpect target=%v encode size want=%v got=%v", e.target, e.TlvSize(), len(bufbyte))
	}
	copy(data[:e.TlvSize()], bufbyte)
	return e.TlvSize(), nil
}
func (e *BasiceSerializer) TlvDecode(data []byte) (error) {
	buf := bytes.NewBuffer(data)
	binary.Read(buf, binary.BigEndian, e.target)
	return nil
}

type SliceSerializer struct {
	target      interface{}
	infType     reflect.Type
	elementSize int
	size        int
	targetPtr   unsafe.Pointer
}

func (e *SliceSerializer) TlvSize() int {
	return e.size
}
func (e *SliceSerializer) TlvEncode(data []byte) (int, error) {
	elem := reflect.ValueOf(e.target).Elem()
	elen := elem.Len()
	sslice := make([]Serializable, elen)
	for i := 0; i < elen; i++ {
		sslice[i] = elem.Index(i).Addr().Interface().(Serializable)
	}
	return TlvEncodeFromSlice(data, sslice, e.elementSize)

}
func (e *SliceSerializer) TlvDecode(data []byte) (error) {
	elem := reflect.ValueOf(e.target).Elem()
	slen, err := DecodeSliceSize(e.elementSize, data)
	//fmt.Printf("slen=%v %v %v %v %v\n",slen,e.infType,elem,e.elementSize,len(data))
	if err != nil {
		return err
	}
	newElems := make([]reflect.Value, slen)
	dslice := make([]Serializable, slen)
	for i := 0; i < slen; i++ {
		newPtr := reflect.New(e.infType)
		newElems[i] = newPtr.Elem()
		dslice[i] = newPtr.Interface().(Serializable)
	}

	TlvDecodeFromSlice(data, dslice, e.elementSize)
	allelems := reflect.Append(elem, newElems...)
	elem.Set(allelems)
	return err
}
func NewSliceSerializer(elementSize int, target interface{}) (*SliceSerializer) {
	elem := reflect.ValueOf(target).Elem()
	ret := &SliceSerializer{
		target:      target,
		infType:     elem.Type().Elem(),
		elementSize: elementSize,
		size:        0,
	}

	slen := elem.Len()
	for i := 0; i < slen; i++ {
		ret.size += elem.Index(i).Addr().Interface().(Serializable).TlvSize()
		if elementSize <= 0 {
			ret.size += HEADSIZE
		}
	}
	return ret
}
func BasicDataSize(data interface{}) int {
	switch data := data.(type) {
	case bool, int8, uint8, *bool, *int8, *uint8:
		return 1
	case []int8:
		return len(data)
	case []uint8:
		return len(data)
	case int16, uint16, *int16, *uint16:
		return 2
	case []int16:
		return 2 * len(data)
	case []uint16:
		return 2 * len(data)
	case int32, uint32, *int32, *uint32:
		return 4
	case []int32:
		return 4 * len(data)
	case []uint32:
		return 4 * len(data)
	case int64, uint64, *int64, *uint64:
		return 8
	case []int64:
		return 8 * len(data)
	case []uint64:
		return 8 * len(data)
	}
	return 0
}
