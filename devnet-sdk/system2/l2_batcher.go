package system2

import (
	"github.com/ethereum-optimism/optimism/op-service/client"
)

// L2BatcherID identifies a L2Batcher by name and chainID, is type-safe, and can be value-copied and used as map key.
type L2BatcherID idWithChain

const L2BatcherKind Kind = "L2Batcher"

func (id L2BatcherID) String() string {
	return idWithChain(id).string(L2BatcherKind)
}

func (id L2BatcherID) MarshalText() ([]byte, error) {
	return idWithChain(id).marshalText(L2BatcherKind)
}

func (id *L2BatcherID) UnmarshalText(data []byte) error {
	return (*idWithChain)(id).unmarshalText(L2BatcherKind, data)
}

func SortL2BatcherIDs(ids []L2BatcherID) []L2BatcherID {
	return copyAndSort(ids, func(a, b L2BatcherID) bool {
		return lessIDWithChain(idWithChain(a), idWithChain(b))
	})
}

// L2Batcher represents an L2 batch-submission service, posting L2 data of an L2 to L1.
type L2Batcher interface {
	Common
	ID() L2BatcherID

	// API to interact with batcher will be added here later
}

type L2BatcherConfig struct {
	CommonConfig
	ID     L2BatcherID
	Client client.RPC
}

type rpcL2Batcher struct {
	commonImpl
	id     L2BatcherID
	client client.RPC
}

var _ L2Batcher = (*rpcL2Batcher)(nil)

func NewL2Batcher(cfg L2BatcherConfig) L2Batcher {
	cfg.Log = cfg.Log.New("chainID", cfg.ID.ChainID, "id", cfg.ID)
	return &rpcL2Batcher{
		commonImpl: newCommon(cfg.CommonConfig),
		id:         cfg.ID,
		client:     cfg.Client,
	}
}

func (r *rpcL2Batcher) ID() L2BatcherID {
	return r.id
}
