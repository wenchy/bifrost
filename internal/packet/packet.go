package packet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// unique sequence id
var uniqSeq uint32 

// header defines all the fields of packet's header
type header struct {
	Magic uint8      // magic number: 110
	Type  PacketType // packet type
	ID    uint64     // client id
	Seq   uint32     // sequence
	Code  int32      // error code
	Size  uint32     // size(bytes) of payload
}

type PacketType uint8

const (
	PacketTypeRequest PacketType = iota
	PacketTypeResponse
	PacketTypeNotice
)

const DefaultMagicNumber uint8 = 110

// Packet is the holder of structured message.
type Packet struct {
	Header header
	Payload   []byte // NUL-padded string
}

func NewPacket() *Packet {
	return new(Packet)
}

func NewRequestPacket(payload []byte) *Packet {
	return &Packet{
		Header: header{
			Magic: DefaultMagicNumber,
			Type: PacketTypeRequest,
			ID: 0,
			Seq: GenUniqSeq(),
			Code: 0,
			Size: 0,
		},
	}
}

// ID return the ID from packet.
func (pkt *Packet) ID() uint64 {
	return pkt.Header.ID
}

func Parse(message []byte) (*Packet, error) {
	buf := bytes.NewReader(message)
	return Decode(buf)
}

// Decode reads a packet from reader.
func Decode(r io.Reader) (*Packet, error) {
	pkt := &Packet{}
	if err := binary.Read(r, binary.BigEndian, &pkt.Header); err != nil {
		return nil, err
	}

	// Reuse the Data slice, if possible
	if cap(pkt.Payload) >= int(pkt.Header.Size) {
		pkt.Payload = pkt.Payload[:pkt.Header.Size]
	} else {
		pkt.Payload = make([]byte, pkt.Header.Size)
	}

	if _, err := io.ReadFull(r, pkt.Payload); err != nil {
		return nil, err
	}

	return pkt, nil
}

// Encode parse a packet to []byte.
func Encode(pkt *Packet) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, pkt.Header)
	if err != nil {
		fmt.Println("binary.Write header failed:", err)
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, pkt.Payload)
	if err != nil {
		fmt.Println("binary.Write payload failed:", err)
		return nil, err
	}
	// fmt.Printf("% x", buf.Bytes())

	return buf.Bytes(), nil
}

// GenUniqSeq generate a new unique sequence id
func GenUniqSeq() uint32 {
	uniqSeq += 1
	return uniqSeq
}
