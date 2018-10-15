package packet

import (
	"bytes"

	"github.com/taktv6/tflow2/convert"
)

// ExtendedIPReachabilityTLVType is the type code of an Extended IP Reachability TLV
const ExtendedIPReachabilityTLVType = 135

// ExtendedIPReachabilityTLV represents an Extended IP Reachability TLV
type ExtendedIPReachabilityTLV struct {
	TLVType                  uint8
	TLVLength                uint8
	ExtendedIPReachabilities []ExtendedIPReachability
}

// ExtendedIPReachability represents a single Extendend IP Reachability Information
type ExtendedIPReachability struct {
	Metric         uint32
	UDSubBitPfxLen uint8
	Address        uint32
	SubTLVs        []TLV
}

// Type gets the type of the TLV
func (e *ExtendedIPReachabilityTLV) Type() uint8 {
	return e.TLVType
}

// Length gets the length of the TLV
func (e *ExtendedIPReachabilityTLV) Length() uint8 {
	return e.TLVLength
}

// Value returns the TLV itself
func (e *ExtendedIPReachabilityTLV) Value() interface{} {
	return e
}

// Serialize serializes an ExtendedIPReachabilityTLV
func (e *ExtendedIPReachabilityTLV) Serialize(buf *bytes.Buffer) {
	buf.WriteByte(ExtendedIPReachabilityTLVType)
	buf.WriteByte(e.TLVLength)

	for _, extIPReach := range e.ExtendedIPReachabilities {
		extIPReach.Serialize(buf)
	}
}

// Serialize serializes an ExtendedIPReachability
func (e *ExtendedIPReachability) Serialize(buf *bytes.Buffer) {
	if len(e.SubTLVs) != 0 {
		e.setSFlag()
	}
	buf.Write(convert.Uint32Byte(e.Metric))
	buf.WriteByte(e.UDSubBitPfxLen)
	buf.Write(convert.Uint32Byte(e.Address))
	for _, tlv := range e.SubTLVs {
		tlv.Serialize(buf)
	}
}

func (e *ExtendedIPReachability) setSFlag() {
	sFlag := uint8(64)
	e.UDSubBitPfxLen = e.UDSubBitPfxLen | sFlag
}

func (e *ExtendedIPReachability) getPfxLen() uint8 {
	tmp := e.UDSubBitPfxLen
	tmp = tmp << 2
	tmp = tmp >> 2
	return tmp
}
