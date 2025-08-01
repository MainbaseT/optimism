package cross

import (
	"errors"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestHazardUnsafeFrontierChecks(t *testing.T) {
	t.Run("empty hazards", func(t *testing.T) {
		ufcd := &mockUnsafeFrontierCheckDeps{}
		hazards := map[eth.ChainID]types.BlockSeal{}
		// when there are no hazards,
		// no work is done, and no error is returned
		err := HazardUnsafeFrontierChecks(ufcd, NewHazardSetFromEntries(hazards))
		require.NoError(t, err)
	})
	t.Run("is cross unsafe", func(t *testing.T) {
		ufcd := &mockUnsafeFrontierCheckDeps{}
		hazards := map[eth.ChainID]types.BlockSeal{eth.ChainIDFromUInt64(123): {Number: 0}}
		ufcd.isCrossUnsafe = nil
		// when there is one hazard, and IsCrossUnsafe returns nil (no error)
		// no error is returned
		err := HazardUnsafeFrontierChecks(ufcd, NewHazardSetFromEntries(hazards))
		require.NoError(t, err)
	})
	t.Run("errFuture: is not local unsafe", func(t *testing.T) {
		ufcd := &mockUnsafeFrontierCheckDeps{}
		hazards := map[eth.ChainID]types.BlockSeal{eth.ChainIDFromUInt64(123): {Number: 0}}
		ufcd.isCrossUnsafe = types.ErrFuture
		ufcd.isLocalUnsafe = errors.New("some error")
		// when there is one hazard, and IsCrossUnsafe returns an ErrFuture,
		// and IsLocalUnsafe returns an error,
		// the error from IsLocalUnsafe is (wrapped and) returned
		err := HazardUnsafeFrontierChecks(ufcd, NewHazardSetFromEntries(hazards))
		require.ErrorContains(t, err, "some error")
	})
	t.Run("errFuture: genesis block", func(t *testing.T) {
		ufcd := &mockUnsafeFrontierCheckDeps{}
		hazards := map[eth.ChainID]types.BlockSeal{eth.ChainIDFromUInt64(123): {Number: 0}}
		ufcd.isCrossUnsafe = types.ErrFuture
		// when there is one hazard, and IsCrossUnsafe returns an ErrFuture,
		// BUT the hazard's block number is 0,
		// no error is returned
		err := HazardUnsafeFrontierChecks(ufcd, NewHazardSetFromEntries(hazards))
		require.NoError(t, err)
	})
	t.Run("errFuture: error getting parent block", func(t *testing.T) {
		ufcd := &mockUnsafeFrontierCheckDeps{}
		hazards := map[eth.ChainID]types.BlockSeal{eth.ChainIDFromUInt64(123): {Number: 3}}
		ufcd.isCrossUnsafe = types.ErrFuture
		ufcd.findBlockIDFn = func() (parent eth.BlockID, err error) {
			return eth.BlockID{}, errors.New("some error")
		}
		// when there is one hazard, and IsCrossUnsafe returns an ErrFuture,
		// and there is an error getting the parent block,
		// the error from ParentBlock is (wrapped and) returned
		err := HazardUnsafeFrontierChecks(ufcd, NewHazardSetFromEntries(hazards))
		require.ErrorContains(t, err, "some error")
	})
	t.Run("errFuture: parent block is not cross unsafe", func(t *testing.T) {
		ufcd := &mockUnsafeFrontierCheckDeps{}
		hazards := map[eth.ChainID]types.BlockSeal{eth.ChainIDFromUInt64(123): {Number: 3}}
		ufcd.isCrossUnsafe = types.ErrFuture
		ufcd.findBlockIDFn = func() (parent eth.BlockID, err error) {
			// when getting the parent block, prep isCrossSafe to be err
			ufcd.isCrossUnsafe = errors.New("not cross unsafe!")
			return eth.BlockID{}, nil
		}
		// when there is one hazard, and IsCrossUnsafe returns an ErrFuture,
		// and the parent block is not cross unsafe,
		// the error from IsCrossUnsafe is (wrapped and) returned
		err := HazardUnsafeFrontierChecks(ufcd, NewHazardSetFromEntries(hazards))
		require.ErrorContains(t, err, "not cross unsafe!")
	})
	t.Run("IsCrossUnsafe Error", func(t *testing.T) {
		ufcd := &mockUnsafeFrontierCheckDeps{}
		hazards := map[eth.ChainID]types.BlockSeal{eth.ChainIDFromUInt64(123): {Number: 3, Hash: common.BytesToHash([]byte{0x02})}}
		ufcd.isCrossUnsafe = errors.New("some error")
		// when there is one hazard, and IsCrossUnsafe returns an error,
		// the error from IsCrossUnsafe is (wrapped and) returned
		err := HazardUnsafeFrontierChecks(ufcd, NewHazardSetFromEntries(hazards))
		require.ErrorContains(t, err, "some error")
	})
}

type mockUnsafeFrontierCheckDeps struct {
	findBlockIDFn func() (parent eth.BlockID, err error)
	isCrossUnsafe error
	isLocalUnsafe error
}

func (m *mockUnsafeFrontierCheckDeps) FindBlockID(chainID eth.ChainID, num uint64) (parent eth.BlockID, err error) {
	if m.findBlockIDFn != nil {
		return m.findBlockIDFn()
	}
	return eth.BlockID{}, nil
}

func (m *mockUnsafeFrontierCheckDeps) IsCrossUnsafe(chainID eth.ChainID, block eth.BlockID) error {
	return m.isCrossUnsafe
}

func (m *mockUnsafeFrontierCheckDeps) IsLocalUnsafe(chainID eth.ChainID, block eth.BlockID) error {
	return m.isLocalUnsafe
}
