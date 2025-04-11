package stack

// L2ProposerID identifies a L2Proposer by name and chainID, is type-safe, and can be value-copied and used as map key.
type L2ProposerID idWithChain

const L2ProposerKind Kind = "L2Proposer"

func (id L2ProposerID) String() string {
	return idWithChain(id).string(L2ProposerKind)
}

func (id L2ProposerID) MarshalText() ([]byte, error) {
	return idWithChain(id).marshalText(L2ProposerKind)
}

func (id *L2ProposerID) UnmarshalText(data []byte) error {
	return (*idWithChain)(id).unmarshalText(L2ProposerKind, data)
}

func SortL2ProposerIDs(ids []L2ProposerID) []L2ProposerID {
	return copyAndSort(ids, func(a, b L2ProposerID) bool {
		return lessIDWithChain(idWithChain(a), idWithChain(b))
	})
}

// L2Proposer is a L2 output proposer, posting claims of L2 state to L1.
type L2Proposer interface {
	Common
	ID() L2ProposerID
}
