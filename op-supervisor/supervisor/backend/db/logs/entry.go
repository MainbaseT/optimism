package logs

import (
	"fmt"
	"io"
	"strings"
)

type EntryObj interface {
	encode() Entry
}

const EntrySize = 34

type Entry [EntrySize]byte

func (e Entry) Type() EntryType {
	return EntryType(e[0])
}

type EntryBinary struct{}

func (EntryBinary) Append(dest []byte, e *Entry) []byte {
	return append(dest, e[:]...)
}

func (EntryBinary) ReadAt(dest *Entry, r io.ReaderAt, at int64) (n int, err error) {
	return r.ReadAt(dest[:], at)
}

func (EntryBinary) EntrySize() int {
	return EntrySize
}

type EntryTypeFlag uint16

const (
	FlagSearchCheckpoint EntryTypeFlag = 1 << TypeSearchCheckpoint
	FlagCanonicalHash    EntryTypeFlag = 1 << TypeCanonicalHash
	FlagInitiatingEvent  EntryTypeFlag = 1 << TypeInitiatingEvent
	FlagExecChainID      EntryTypeFlag = 1 << TypeExecChainID
	FlagExecPosition     EntryTypeFlag = 1 << TypeExecPosition
	FlagExecChecksum     EntryTypeFlag = 1 << TypeExecChecksum
	FlagPadding          EntryTypeFlag = 1 << TypePadding
	// for additional padding
	FlagPadding2 EntryTypeFlag = FlagPadding << 1
	FlagPadding3 EntryTypeFlag = FlagPadding2 << 1
)

func (x EntryTypeFlag) String() string {
	var out []string
	for i := EntryTypeFlag(1); i != 0; i <<= 1 { // iterate to bitmask
		if x.Any(i) {
			out = append(out, i.String())
		}
	}
	return strings.Join(out, "|")
}

func (x EntryTypeFlag) Any(v EntryTypeFlag) bool {
	return x&v != 0
}

func (x *EntryTypeFlag) Add(v EntryTypeFlag) {
	*x = *x | v
}

func (x *EntryTypeFlag) Remove(v EntryTypeFlag) {
	*x = *x &^ v
}

type EntryType uint8

const (
	TypeSearchCheckpoint EntryType = iota
	TypeCanonicalHash
	TypeInitiatingEvent
	TypeExecChainID
	TypeExecPosition
	TypeExecChecksum
	TypePadding
)

func (x EntryType) String() string {
	switch x {
	case TypeSearchCheckpoint:
		return "searchCheckpoint"
	case TypeCanonicalHash:
		return "canonicalHash"
	case TypeInitiatingEvent:
		return "initiatingEvent"
	case TypeExecChainID:
		return "execChainID"
	case TypeExecPosition:
		return "execPosition"
	case TypeExecChecksum:
		return "execChecksum"
	case TypePadding:
		return "padding"
	default:
		return fmt.Sprintf("unknown-%d", uint8(x))
	}
}
