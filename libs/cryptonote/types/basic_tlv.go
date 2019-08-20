package types

import (
	"unsafe"
)

// Key : tlv interface implements

func (k *Key) TlvSize() int {
	return COMMONLEN
}
func (k *Key) TlvEncode(encodeDate []byte) (int, error) {
	if len(encodeDate) < COMMONLEN {
		return 0, EncodeSpaceERR
	}
	copy(encodeDate[:COMMONLEN], (*k)[:])
	return COMMONLEN, nil
}
func (k *Key) TlvDecode(data []byte) (error) {
	if len(data) != COMMONLEN {
		return IvalidCodeERR
	}
	copy((*k)[:], data[:])
	return nil
}

// Key64 : tlv interface implements
func (k *Key64) TlvSize() int {
	return KEY64 * COMMONLEN
}
func (k *Key64) TlvEncode(data []byte) (int, error) {
	klen := len((*k))
	datalen := len(data)
	offset := 0
	for i := 0; i < klen; i++ {
		inc := (*k)[i].TlvSize()
		if offset+inc > datalen {
			return 0, EncodeSpaceERR
		}
		(*k)[i].TlvEncode(data[offset : offset+inc])
		offset += inc
	}
	return offset, nil
}
func (k *Key64) TlvDecode(data []byte) error {
	offset := 0
	for i := 0; i < len(*k); i++ {
		if offset+COMMONLEN > len(data) {
			return IvalidCodeERR
		}
		if err := (*k)[i].TlvDecode(data[offset : offset+COMMONLEN]); err != nil {
			return err
		}
		offset = offset + COMMONLEN
	}
	return nil
}

// KeyV : tlv interface implements
func (k *KeyV) TlvSize() int {
	return len(*k) * COMMONLEN
}
func (k *KeyV) TlvEncode(data []byte) (int, error) {
	klen := len((*k))
	offset := 0
	for i := 0; i < klen; i++ {
		inc := (*k)[i].TlvSize()
		if offset+inc > len(data) {
			return 0, EncodeSpaceERR
		}
		(*k)[i].TlvEncode(data[offset : offset+inc])
		offset += inc
	}
	return offset, nil
}
func (k *KeyV) TlvDecode(data []byte) error {
	offset := 0
	i := 0
	(*k) = make([]Key, 0)
	for ; offset < len(data); i++ {
		if offset+COMMONLEN > len(data) {
			return IvalidCodeERR
		}
		(*k) = append((*k), Key{})
		if err := (*k)[i].TlvDecode(data[offset : offset+COMMONLEN]); err != nil {
			return err
		}
		offset = offset + COMMONLEN

	}
	if offset != len(data) {
		return IvalidCodeERR
	}
	return nil
}

// KeyM : tlv interface implements
func (k *KeyM) TlvSize() int {
	ret := 0
	klen := len((*k))
	for i := 0; i < klen; i++ {
		ret += (*k)[i].TlvSize() + HEADSIZE
	}
	return ret
}
func (k *KeyM) TlvEncode(data []byte) (int, error) {
	klen := len((*k))
	datalen := len(data)
	offset := 0
	for i := 0; i < klen; i++ {
		inc := (*k)[i].TlvSize()
		if offset+inc+HEADSIZE > datalen {
			return 0, EncodeSpaceERR
		}
		TagTo2Byte(Tlvtag(i), data[offset:offset+TAGSIZE])
		LenTo2Byte(inc, data[offset+LENOFFSET:offset+LENOFFSET+LENSIZE])
		(*k)[i].TlvEncode(data[offset+HEADSIZE : offset+HEADSIZE+inc])
		offset += inc + HEADSIZE
	}
	return offset, nil
}
func (k *KeyM) TlvDecode(data []byte) error {
	offset := 0
	datalen := len(data)
	i := 0
	for ; offset < datalen; i++ {
		_, dlen, err := ParseTagAndLen(data[offset:])
		if err != nil {
			return err
		}
		if offset+HEADSIZE+dlen > datalen {
			return IvalidCodeERR
		}
		(*k) = append((*k), KeyV{})
		if err := (*k)[i].TlvDecode(data[offset+HEADSIZE : offset+HEADSIZE+dlen]); err != nil {
			return err
		}
		offset += HEADSIZE + dlen
	}
	if offset != datalen {
		return IvalidCodeERR
	}
	return nil
}

// Ctkey : tlv interface implements
func (k *Ctkey) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *Ctkey) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 2)
	ret[0x0001] = &(k.Dest)
	ret[0x0002] = &(k.Mask)
	return ret
}
func (k *Ctkey) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *Ctkey) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())
}
func (k *Ctkey) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}

// CtkeyV : tlv interface implements
func (k *CtkeyV) TlvSize() int {
	ret := 0
	for _, item := range (*k)[:] {
		ret += item.TlvSize()
	}
	return ret
}
func (k *CtkeyV) TlvEncode(data []byte) (int, error) {
	serializables := make([]Serializable, len((*k)[:]))
	eleSize := len((*k))
	for i := 0; i < eleSize; i++ {
		serializables[i] = &((*k)[i])
	}
	//fmt.Printf("Ctkey size=%v\n", len(*k))

	return TlvEncodeFromSlice(data, serializables, (&Ctkey{}).TlvSize())

}
func (k *CtkeyV) TlvDecode(data []byte) error {
	inc := (&Ctkey{}).TlvSize()
	offset := 0
	datalen := len(data)
	for i := 0; offset < datalen; i++ {
		(*k) = append((*k), Ctkey{})
		(*k)[i].TlvDecode(data[offset : offset+inc])
		offset += inc
	}
	return nil
}

// CtkeyM : tlv interface implements
func (k *CtkeyM) TlvSize() int {
	ret := 0
	for _, item := range (*k)[:] {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *CtkeyM) TlvEncode(data []byte) (int, error) {
	serializables := make([]Serializable, len((*k)[:]))
	eleSize := len((*k))
	for i := 0; i < eleSize; i++ {
		serializables[i] = &((*k)[i])
	}
	return TlvEncodeFromSlice(data, serializables, 0)

}
func (k *CtkeyM) TlvDecode(data []byte) error {
	offset := 0
	datalen := len(data)
	for i := 0; offset < datalen; i++ {
		(*k) = append((*k), CtkeyV{})
		_, inc, err := ParseTagAndLen(data)
		if err != nil {
			return err
		}
		(*k)[i].TlvDecode(data[offset+VOFFSET : offset+VOFFSET+inc])
		offset += VOFFSET + inc
	}
	return nil
}

// MultisigKLRki : tlv interface implements
func (k *MultisigKLRki) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 4)
	ret[0x0001] = &(k.K)
	ret[0x0002] = &(k.Ki)
	ret[0x0003] = &(k.L)
	ret[0x0004] = &(k.R)
	return ret
}
func (k *MultisigKLRki) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *MultisigKLRki) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *MultisigKLRki) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *MultisigKLRki) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}

// MultisigOut : tlv interface implements
func (k *MultisigOut) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 1)
	ret[0x0001] = &(k.C)
	return ret
}
func (k *MultisigOut) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *MultisigOut) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *MultisigOut) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *MultisigOut) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}

// EcdhTuple : tlv interface implements
func (k *EcdhTuple) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 3)
	ret[0x0001] = &(k.Mask)
	ret[0x0002] = &(k.Amount)
	ret[0x0003] = &(k.SenderPK)
	return ret
}
func (k *EcdhTuple) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *EcdhTuple) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *EcdhTuple) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *EcdhTuple) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}

// BoroSig : tlv interface implements
func (k *BoroSig) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 3)
	ret[0x0001] = &(k.Ee)
	ret[0x0002] = &(k.S0)
	ret[0x0003] = &(k.S1)
	return ret
}
func (k *BoroSig) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *BoroSig) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *BoroSig) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *BoroSig) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}

// MgSig : tlv interface implements
func (k *MgSig) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 3)
	ret[0x0001] = &(k.Cc)
	ret[0x0002] = &(k.II)
	ret[0x0003] = &(k.Ss)
	return ret
}
func (k *MgSig) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *MgSig) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *MgSig) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *MgSig) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}

// RangeSig : tlv interface implements
func (k *RangeSig) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 2)
	ret[0x0001] = &(k.Asig)
	ret[0x0002] = &(k.Ci)
	return ret
}
func (k *RangeSig) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *RangeSig) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *RangeSig) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *RangeSig) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}

// Bulletproof : tlv interface implements
func (k *Bulletproof) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 12)
	ret[0x0001] = &(k.V)
	ret[0x0002] = &(k.A)
	ret[0x0003] = &(k.S)
	ret[0x0004] = &(k.T1)
	ret[0x0005] = &(k.T2)
	ret[0x0006] = &(k.Taux)
	ret[0x0007] = &(k.Mu)
	ret[0x0008] = &(k.L)
	ret[0x0009] = &(k.R)
	ret[0x000a] = &(k.Aa)
	ret[0x000b] = &(k.B)
	ret[0x000c] = &(k.T)
	return ret
}
func (k *Bulletproof) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *Bulletproof) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *Bulletproof) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *Bulletproof) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}

// RangeProofType : tlv interface implements
func (k *RangeProofType) TlvSize() int {
	return 1
}
func (k *RangeProofType) TlvEncode(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, EncodeSpaceERR
	}
	data[0] = byte(*k)
	return 1, nil
}
func (k *RangeProofType) TlvDecode(data []byte) error {
	if len(data) != 1 {
		return DecodeSpaceERR
	}
	*k = RangeProofType(data[0])
	return nil
}

// RctConfig : tlv interface implements
func (k *RctConfig) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 2)
	ret[0x0001] = NewBasiceSerializer(&(k.BpVersion))
	p := uintptr(unsafe.Pointer(&(k.RangeProofType)))
	ret[0x0002] = NewBasiceSerializer((*uint8)(unsafe.Pointer(p)))
	return ret
}
func (k *RctConfig) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *RctConfig) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *RctConfig) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *RctConfig) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}

// RctSigBase : tlv interface implements
func (k *RctSigBase) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 7)

	ret[0x0001] = NewBasiceSerializer((*uint8)(unsafe.Pointer(&(k.Type))))
	ret[0x0002] = &(k.Message)
	ret[0x0003] = &(k.MixRing)
	ret[0x0004] = &(k.PseudoOuts)
	ret[0x0005] = NewSliceSerializer((&(EcdhTuple{})).TlvSize(), &(k.EcdhInfo))
	ret[0x0006] = &(k.OutPk)
	ret[0x0007] = NewBasiceSerializer((*uint64)(unsafe.Pointer(&(k.TxnFee))))

	return ret
}
func (k *RctSigBase) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *RctSigBase) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *RctSigBase) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *RctSigBase) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}

// RctSigPrunable : tlv interface implements
func (k *RctSigPrunable) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 4)

	ret[0x0001] = NewSliceSerializer(0, &(k.RangeSigs))
	ret[0x0002] = NewSliceSerializer(0, &(k.Bulletproofs))
	ret[0x0003] = NewSliceSerializer(0, &(k.MGs))
	ret[0x0004] = &(k.PseudoOuts)

	return ret
}
func (k *RctSigPrunable) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *RctSigPrunable) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *RctSigPrunable) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *RctSigPrunable) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}

// RctSig : tlv interface implements
func (k *RctSig) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 2)
	ret[0x0001] = &(k.P)
	ret[0x0002] = &(k.RctSigBase)
	return ret
}
func (k *RctSig) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *RctSig) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *RctSig) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *RctSig) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}
//PublicKey
func (k *PublicKey) TlvSize() int {
	return COMMONLEN
}
func (k *PublicKey) TlvEncode(encodeDate []byte) (int, error) {
	if len(encodeDate) < COMMONLEN {
		return 0, EncodeSpaceERR
	}
	copy(encodeDate[:COMMONLEN], (*k)[:])
	return COMMONLEN, nil
}
func (k *PublicKey) TlvDecode(data []byte) (error) {
	if len(data) != COMMONLEN {
		return IvalidCodeERR
	}
	copy((*k)[:], data[:])
	return nil
}
//SecretKey
func (k *SecretKey) TlvSize() int {
	return COMMONLEN
}
func (k *SecretKey) TlvEncode(encodeDate []byte) (int, error) {
	if len(encodeDate) < COMMONLEN {
		return 0, EncodeSpaceERR
	}
	copy(encodeDate[:COMMONLEN], (*k)[:])
	return COMMONLEN, nil
}
func (k *SecretKey) TlvDecode(data []byte) (error) {
	if len(data) != COMMONLEN {
		return IvalidCodeERR
	}
	copy((*k)[:], data[:])
	return nil
}

//AccountAddress
func (k *AccountAddress) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 2)
	ret[0x0001] = &(k.SpendPublicKey)
	ret[0x0002] = &(k.ViewPublicKey)
	return ret
}
func (k *AccountAddress) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *AccountAddress) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *AccountAddress) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *AccountAddress) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}
//AccountKey
func (k *AccountKey) EncodeMap() map[Tlvtag]Serializable {
	ret := make(map[Tlvtag]Serializable, 3)
	ret[0x0001] = &(k.Addr)
	ret[0x0002] = &(k.SpendSKey)
	ret[0x0003] = &(k.ViewSKey)
	return ret
}
func (k *AccountKey) DecodeMap() map[Tlvtag]Serializable {
	return k.EncodeMap()
}
func (k *AccountKey) TlvSize() int {
	ret := 0
	for _, item := range k.EncodeMap() {
		ret += item.TlvSize() + HEADSIZE
	}
	return ret
}
func (k *AccountKey) TlvEncode(data []byte) (int, error) {
	return TlvEncodeFromMap(data, k.EncodeMap())

}
func (k *AccountKey) TlvDecode(data []byte) error {
	return TlvDecodeFromMap(data, k.DecodeMap())
}