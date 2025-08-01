package extract

import (
	"context"
	"fmt"
	"testing"

	contractMetrics "github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching/rpcblock"
	"github.com/ethereum-optimism/optimism/packages/contracts-bedrock/snapshots"
	"github.com/ethereum/go-ethereum/common"

	faultTypes "github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	batchingTest "github.com/ethereum-optimism/optimism/op-service/sources/batching/test"
	"github.com/stretchr/testify/require"
)

var (
	fdgAddr = common.HexToAddress("0x24112842371dFC380576ebb09Ae16Cb6B6caD7CB")
)

func TestMetadataCreator_CreateContract(t *testing.T) {
	tests := []struct {
		name        string
		game        types.GameMetadata
		expectedErr error
	}{
		{
			name: "validCannonGameType",
			game: types.GameMetadata{GameType: uint32(faultTypes.CannonGameType), Proxy: fdgAddr},
		},
		{
			name: "validPermissionedGameType",
			game: types.GameMetadata{GameType: uint32(faultTypes.PermissionedGameType), Proxy: fdgAddr},
		},
		{
			name: "validAsteriscGameType",
			game: types.GameMetadata{GameType: uint32(faultTypes.AsteriscGameType), Proxy: fdgAddr},
		},
		{
			name: "validAlphabetGameType",
			game: types.GameMetadata{GameType: uint32(faultTypes.AlphabetGameType), Proxy: fdgAddr},
		},
		{
			name: "validFastGameType",
			game: types.GameMetadata{GameType: uint32(faultTypes.FastGameType), Proxy: fdgAddr},
		},
		{
			name: "validAsteriscKonaGameType",
			game: types.GameMetadata{GameType: uint32(faultTypes.AsteriscKonaGameType), Proxy: fdgAddr},
		},
		{
			name: "validSuperCannonGameType",
			game: types.GameMetadata{GameType: uint32(faultTypes.SuperCannonGameType), Proxy: fdgAddr},
		},
		{
			name: "validSuperPermissionedGameType",
			game: types.GameMetadata{GameType: uint32(faultTypes.SuperPermissionedGameType), Proxy: fdgAddr},
		},
		{
			name: "validSuperAsteriscKonaGameType",
			game: types.GameMetadata{GameType: uint32(faultTypes.SuperAsteriscKonaGameType), Proxy: fdgAddr},
		},
		{
			name:        "InvalidGameType",
			game:        types.GameMetadata{GameType: 6, Proxy: fdgAddr},
			expectedErr: fmt.Errorf("unsupported game type: 6"),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			caller, metrics := setupMetadataLoaderTest(t, test.game.GameType)
			creator := NewGameCallerCreator(metrics, caller)
			_, err := creator.CreateContract(context.Background(), test.game)
			require.Equal(t, test.expectedErr, err)
			if test.expectedErr == nil {
				require.Equal(t, 1, metrics.cacheAddCalls)
				require.Equal(t, 1, metrics.cacheGetCalls)
			}
			_, err = creator.CreateContract(context.Background(), test.game)
			require.Equal(t, test.expectedErr, err)
			if test.expectedErr == nil {
				require.Equal(t, 1, metrics.cacheAddCalls)
				require.Equal(t, 2, metrics.cacheGetCalls)
			}
		})
	}
}

func setupMetadataLoaderTest(t *testing.T, gameType uint32) (*batching.MultiCaller, *mockCacheMetrics) {
	fdgAbi := snapshots.LoadFaultDisputeGameABI()
	if gameType == uint32(faultTypes.SuperPermissionedGameType) || gameType == uint32(faultTypes.SuperCannonGameType) || gameType == uint32(faultTypes.SuperAsteriscKonaGameType) {
		fdgAbi = snapshots.LoadSuperFaultDisputeGameABI()
	}
	stubRpc := batchingTest.NewAbiBasedRpc(t, fdgAddr, fdgAbi)
	caller := batching.NewMultiCaller(stubRpc, batching.DefaultBatchSize)
	stubRpc.SetResponse(fdgAddr, "version", rpcblock.Latest, nil, []interface{}{"0.18.0"})
	stubRpc.SetResponse(fdgAddr, "gameType", rpcblock.Latest, nil, []interface{}{gameType})
	return caller, &mockCacheMetrics{}
}

type mockCacheMetrics struct {
	cacheAddCalls int
	cacheGetCalls int
	*contractMetrics.NoopMetrics
}

func (m *mockCacheMetrics) CacheAdd(_ string, _ int, _ bool) {
	m.cacheAddCalls++
}
func (m *mockCacheMetrics) CacheGet(_ string, _ bool) {
	m.cacheGetCalls++
}
