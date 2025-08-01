package interop

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/core"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	bss "github.com/ethereum-optimism/optimism/op-batcher/batcher"
	"github.com/ethereum-optimism/optimism/op-chain-ops/devkeys"
	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/interopgen"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/blobstore"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/contracts/bindings/emit"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/contracts/bindings/inbox"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/fakebeacon"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"
	l2os "github.com/ethereum-optimism/optimism/op-proposer/proposer"
	"github.com/ethereum-optimism/optimism/op-service/clock"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum-optimism/optimism/op-service/oppprof"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	supervisorConfig "github.com/ethereum-optimism/optimism/op-supervisor/config"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/syncnode"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// SuperSystem is an interface for the system (collection of connected resources)
// it provides a way to get the resources for a network by network ID
// and provides a way to get the list of network IDs
// this is useful for testing multiple network backends,
// for example, interopE2ESystem is the default implementation, but a shim to
// kurtosis or another testing framework could be implemented
type SuperSystem interface {
	L1() *geth.GethInstance
	L1GethClient() *ethclient.Client
	L1Beacon() *fakebeacon.FakeBeacon
	AdvanceL1Time(duration time.Duration)
	DisputeGameFactoryAddr() common.Address

	// Superchain level
	L2IDs() []string
	Supervisor() *supervisor.SupervisorService
	Batcher(network string) *bss.BatcherService
	Proposer(network string) *l2os.ProposerService
	AddUser(username string)
	SupervisorClient() *sources.SupervisorClient
	DependencySet() *depset.StaticConfigDependencySet

	// L2 client specific
	L2GethEndpoint(id string, name string) endpoint.RPC
	L2GethClient(network string, node string) *ethclient.Client
	L2RollupEndpoint(network string, node string) endpoint.RPC
	L2RollupClient(network string, node string) *sources.RollupClient
	SendL2Tx(network string, node string, username string, applyTxOpts helpers.TxOptsFn) *types.Receipt
	EmitData(ctx context.Context, network string, node string, username string, data string) *types.Receipt
	AddNode(network string, nodeName string)

	// L2 level
	ChainID(network string) *big.Int
	RollupConfig(network string) *rollup.Config
	L2Genesis(network string) *core.Genesis
	UserKey(network, username string) ecdsa.PrivateKey
	L2OperatorKey(network string, role devkeys.ChainOperatorRole) ecdsa.PrivateKey
	Address(network string, username string) common.Address
	Contract(network string, contractName string) interface{}
	DeployEmitterContract(ctx context.Context, network string, username string) common.Address
	ValidateMessage(
		ctx context.Context,
		id string,
		sender string,
		msgIdentifier supervisortypes.Identifier,
		msgHash [32]byte,
		expectedError error,
	) (*types.Receipt, error)
	// Access a contract on a network by name
}
type SuperSystemConfig struct {
	mempoolFiltering  bool
	SupportTimeTravel bool
	BatcherUsesBlobs  bool
}

// NewSuperSystem creates a new SuperSystem from a recipe. It creates an interopE2ESystem.
func NewSuperSystem(t *testing.T, recipe *interopgen.InteropDevRecipe, w WorldResourcePaths, config SuperSystemConfig) SuperSystem {
	s2 := &interopE2ESystem{recipe: recipe, config: &config}
	s2.prepare(t, w)
	return s2
}

// interopE2ESystem implements the SuperSystem interface
// it prepares network resources and provides access to them
// the functionality is broken down into smaller functions so that
// the system can be prepared iteratively if desired
type interopE2ESystem struct {
	t               *testing.T
	recipe          *interopgen.InteropDevRecipe
	logger          log.Logger
	timeTravelClock *clock.AdvancingClock
	hdWallet        *devkeys.MnemonicDevKeys
	worldDeployment *interopgen.WorldDeployment
	worldOutput     *interopgen.WorldOutput
	beacon          *fakebeacon.FakeBeacon
	l1              *geth.GethInstance
	l2s             map[string]l2Net
	// supervisor and L1 clients should be singletons, so they are cached
	l1GethClient *ethclient.Client
	superClient  *sources.SupervisorClient
	supervisor   *supervisor.SupervisorService
	config       *SuperSystemConfig
}

func (s *interopE2ESystem) L1() *geth.GethInstance {
	return s.l1
}

func (s *interopE2ESystem) L1Beacon() *fakebeacon.FakeBeacon {
	return s.beacon
}

func (s *interopE2ESystem) AdvanceL1Time(duration time.Duration) {
	require.NotNil(s.t, s.timeTravelClock, "Attempting to time travel without enabling it.")
	s.timeTravelClock.AdvanceTime(duration)
}

func (s *interopE2ESystem) DisputeGameFactoryAddr() common.Address {
	return s.worldDeployment.Interop.DisputeGameFactory
}

// prepareHDWallet creates a new HD wallet to derive keys from
func (s *interopE2ESystem) prepareHDWallet() *devkeys.MnemonicDevKeys {
	hdWallet, err := devkeys.NewMnemonicDevKeys(devkeys.TestMnemonic)
	require.NoError(s.t, err)
	return hdWallet
}

type WorldResourcePaths struct {
	FoundryArtifacts string
	SourceMap        string
}

// prepareWorld creates the world configuration from the recipe and deploys it
func (s *interopE2ESystem) prepareWorld(w WorldResourcePaths) (*interopgen.WorldDeployment, *interopgen.WorldOutput) {
	// Build the world configuration from the recipe and the HD wallet
	worldCfg, err := s.recipe.Build(s.hdWallet)
	require.NoError(s.t, err)

	for _, l2Cfg := range worldCfg.L2s {
		require.NotNil(s.t, l2Cfg.L2GenesisIsthmusTimeOffset, "expecting isthmus fork to be enabled for interop deployments")
		require.NotNil(s.t, l2Cfg.L2GenesisIsthmusTimeOffset, "expecting jovian fork to be enabled for interop deployments")
	}

	// create a logger for the world configuration
	logger := s.logger.New("role", "world")
	require.NoError(s.t, worldCfg.Check(logger))

	// create the foundry artifacts and source map
	foundryArtifacts := foundry.OpenArtifactsDir(w.FoundryArtifacts)
	sourceMap := foundry.NewSourceMapFS(os.DirFS(w.SourceMap))

	// deploy the world, using the logger, foundry artifacts, source map, and world configuration
	worldDeployment, worldOutput, err := interopgen.Deploy(logger, foundryArtifacts, sourceMap, worldCfg)
	require.NoError(s.t, err)

	return worldDeployment, worldOutput
}

// prepareL1 creates the L1 chain resources
func (s *interopE2ESystem) prepareL1() (*fakebeacon.FakeBeacon, *geth.GethInstance) {
	// Create a fake Beacon node to hold on to blobs created by the L1 miner, and to serve them to L2
	genesisTimestampL1 := s.worldOutput.L1.Genesis.Timestamp
	blockTimeL1 := uint64(6)
	blobPath := s.t.TempDir()
	bcn := fakebeacon.NewBeacon(s.logger.New("role", "l1_cl"),
		blobstore.New(), genesisTimestampL1, blockTimeL1)
	s.t.Cleanup(func() {
		_ = bcn.Close()
	})
	require.NoError(s.t, bcn.Start("127.0.0.1:0"))
	beaconApiAddr := bcn.BeaconAddr()
	require.NotEmpty(s.t, beaconApiAddr, "beacon API listener must be up")

	l1FinalizedDistance := uint64(3)
	l1Clock := clock.SystemClock
	if s.config.SupportTimeTravel {
		s.timeTravelClock = clock.NewAdvancingClock(100 * time.Millisecond)
		l1Clock = s.timeTravelClock
	}
	// Start the L1 chain
	l1Geth, _, err := geth.InitL1(
		blockTimeL1,
		l1FinalizedDistance,
		s.worldOutput.L1.Genesis,
		l1Clock,
		filepath.Join(blobPath, "l1_el"),
		bcn)

	require.NoError(s.t, err)
	require.NoError(s.t, l1Geth.Node.Start())
	s.t.Cleanup(func() {
		s.t.Logf("Closing L1 geth")
		_ = l1Geth.Close()
	})
	return bcn, l1Geth
}

// newOperatorKeysForL2 creates the operator keys for an L2 chain
// it uses an L2Output to determine the chain ID and configuration,
// and then makes a key for each operator role [SequencerP2PRole, ProposerRole, BatcherRole]
func (s *interopE2ESystem) newOperatorKeysForL2(l2Out *interopgen.L2Output) map[devkeys.ChainOperatorRole]ecdsa.PrivateKey {
	// Create operatorKeys for the L2 chain actors
	operatorKeys := map[devkeys.ChainOperatorRole]ecdsa.PrivateKey{}
	// create the sequencer P2P secret
	seqP2PSecret, err := s.hdWallet.Secret(devkeys.ChainOperatorKey{
		ChainID: l2Out.Genesis.Config.ChainID,
		Role:    devkeys.SequencerP2PRole,
	})
	require.NoError(s.t, err)
	operatorKeys[devkeys.SequencerP2PRole] = *seqP2PSecret
	// create the proposer secret
	proposerSecret, err := s.hdWallet.Secret(devkeys.ChainOperatorKey{
		ChainID: l2Out.Genesis.Config.ChainID,
		Role:    devkeys.ProposerRole,
	})
	require.NoError(s.t, err)
	operatorKeys[devkeys.ProposerRole] = *proposerSecret
	// create the batcher secret
	batcherSecret, err := s.hdWallet.Secret(devkeys.ChainOperatorKey{
		ChainID: l2Out.Genesis.Config.ChainID,
		Role:    devkeys.BatcherRole,
	})
	require.NoError(s.t, err)
	operatorKeys[devkeys.BatcherRole] = *batcherSecret
	return operatorKeys
}

func (s *interopE2ESystem) ChainID(network string) *big.Int {
	return s.l2s[network].chainID
}

func (s *interopE2ESystem) RollupConfig(network string) *rollup.Config {
	return s.l2s[network].l2Out.RollupCfg
}

func (s *interopE2ESystem) L2Genesis(network string) *core.Genesis {
	return s.l2s[network].l2Out.Genesis
}

// prepareSupervisor creates a new supervisor for the system
func (s *interopE2ESystem) prepareSupervisor() *supervisor.SupervisorService {
	// Be verbose with op-supervisor, it's in early test phase
	logger := testlog.Logger(s.t, log.LevelDebug).New("role", "supervisor")
	cfg := &supervisorConfig.Config{
		MetricsConfig: metrics.CLIConfig{
			Enabled: false,
		},
		PprofConfig: oppprof.CLIConfig{
			ListenEnabled: false,
		},
		LogConfig: oplog.CLIConfig{
			Level:  log.LevelDebug,
			Format: oplog.FormatText,
		},
		RPC: oprpc.CLIConfig{
			ListenAddr:  "127.0.0.1",
			ListenPort:  0,
			EnableAdmin: true,
		},
		SyncSources:             &syncnode.CLISyncNodes{}, // no sync-sources
		L1RPC:                   s.l1.UserRPC().RPC(),
		Datadir:                 path.Join(s.t.TempDir(), "supervisor"),
		RPCVerificationWarnings: true,
	}

	fullCfgSet, err := worldToFullCfgSet(s.worldOutput)
	require.NoError(s.t, err)
	cfg.FullConfigSetSource = fullCfgSet

	// Create the supervisor with the configuration
	super, err := supervisor.SupervisorFromConfig(context.Background(), cfg, logger)
	require.NoError(s.t, err)
	// Start the supervisor
	err = super.Start(context.Background())
	require.NoError(s.t, err)
	s.t.Cleanup(func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // force-quit
		s.t.Logf("Closing supervisor")
		closeErr := super.Stop(ctx)
		s.t.Logf("Closed supervisor: %v", closeErr)
	})
	return super
}

// SupervisorClient returns the supervisor client for the system, creating it if it doesn't exist
func (s *interopE2ESystem) SupervisorClient() *sources.SupervisorClient {
	if s.superClient != nil {
		return s.superClient
	}
	superClient, err := dial.DialSupervisorClientWithTimeout(context.Background(), s.logger, s.supervisor.RPC())
	require.NoError(s.t, err, "failed to dial supervisor RPC")
	s.superClient = superClient
	return superClient
}

// prepare sets up the system for testing
// components are built iteratively, so that they can be reused or modified
// their creation can't be safely skipped or reordered at this time
func (s *interopE2ESystem) prepare(t *testing.T, w WorldResourcePaths) {
	s.t = t
	s.logger = testlog.Logger(s.t, log.LevelDebug)
	s.hdWallet = s.prepareHDWallet()
	s.worldDeployment, s.worldOutput = s.prepareWorld(w)

	// L1 first so that the Supervisor and L2s can connect to it
	s.beacon, s.l1 = s.prepareL1()

	s.supervisor = s.prepareSupervisor()

	s.l2s = s.prepareL2s()

	s.prepareContracts()

	// add the L2 RPCs to the supervisor now that the L2s are created
	ctx := context.Background()
	for _, l2 := range s.l2s {
		for _, node := range l2.nodes {
			rpcEndpoint, secret := node.opNode.InteropRPC()
			err := s.SupervisorClient().AddL2RPC(ctx, rpcEndpoint, secret)
			require.NoError(s.t, err, "failed to add L2 RPC to supervisor")
		}
	}

	// Try to close the op-supervisor first
	s.t.Cleanup(func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // force-quit
		s.t.Logf("Closing supervisor")
		closeErr := s.supervisor.Stop(ctx)
		s.t.Logf("Closed supervisor: %v", closeErr)
	})
}

// AddUser adds a user to the system by creating a user key for each L2.
// each user key is stored in the L2's userKeys map.
// because all user maps start empty, a users index should be the same for all L2s,
// but if in the future these maps can diverge, the indexes for username would also diverge
// NOTE: The first 20 accounts are implicitly funded by the Recipe's World Deployment
// see: op-chain-ops/interopgen/recipe.go
func (s *interopE2ESystem) AddUser(username string) {
	for id, l2 := range s.l2s {
		bigID, _ := big.NewInt(0).SetString(id, 10)
		userSecret, _ := s.hdWallet.Secret(
			devkeys.ChainUserKey{
				ChainID: bigID,
				Index:   uint64(len(l2.userKeys)),
			},
		)
		l2.userKeys[username] = *userSecret
	}
}

// UserKey returns the user key for a user on an L2
func (s *interopE2ESystem) UserKey(id, username string) ecdsa.PrivateKey {
	return s.l2s[id].userKeys[username]
}

// Address returns the address for a user on an L2
func (s *interopE2ESystem) Address(id, username string) common.Address {
	secret := s.UserKey(id, username)
	require.NotNil(s.t, secret, "no secret found for user %s", username)
	return crypto.PubkeyToAddress(secret.PublicKey)
}

// prepareL2s creates the L2s for the system, returning a map of L2s
func (s *interopE2ESystem) prepareL2s() map[string]l2Net {
	l2s := make(map[string]l2Net)
	for id, l2Out := range s.worldOutput.L2s {
		l2s[id] = s.newL2(id, l2Out, s.DependencySet())
	}
	return l2s
}

// prepareContracts prepares contract-bindings for the L2s
func (s *interopE2ESystem) prepareContracts() {
	// Add bindings to common contracts for each L2
	for id := range s.worldDeployment.L2s {
		{
			contract, err := inbox.NewInbox(predeploys.CrossL2InboxAddr, s.L2GethClient(id, "sequencer"))
			require.NoError(s.t, err)
			s.l2s[id].contracts["inbox"] = contract
		}
	}
}

func (s *interopE2ESystem) L1GethClient() *ethclient.Client {
	if s.l1GethClient != nil {
		return s.l1GethClient
	}
	rpcEndpoint := s.l1.UserRPC()
	rpcCl := endpoint.DialRPC(
		endpoint.PreferAnyRPC,
		rpcEndpoint,
		func(v string) *rpc.Client {
			logger := testlog.Logger(s.t, log.LevelInfo)
			cl, err := dial.DialRPCClientWithTimeout(context.Background(), logger, v)
			require.NoError(s.t, err, "failed to dial L1 eth node instance")
			return cl
		})
	nodeClient := ethclient.NewClient(rpcCl)
	// register the client so it can be reused
	s.l1GethClient = nodeClient
	return nodeClient
}

func (s *interopE2ESystem) Supervisor() *supervisor.SupervisorService { return s.supervisor }
func (s *interopE2ESystem) Batcher(id string) *bss.BatcherService     { return s.l2s[id].batcher }
func (s *interopE2ESystem) Proposer(id string) *l2os.ProposerService  { return s.l2s[id].proposer }
func (s *interopE2ESystem) L2OperatorKey(id string, role devkeys.ChainOperatorRole) ecdsa.PrivateKey {
	return s.l2s[id].operatorKeys[role]
}

// L2IDs returns the list of L2 IDs, which are the keys of the L2s map
func (s *interopE2ESystem) L2IDs() []string {
	ids := make([]string, 0, len(s.l2s))
	for id := range s.l2s {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// SendL2Tx sends an L2 transaction to the L2 with the given ID.
// it acts as a wrapper around op-e2e.SendL2TxWithID
// and uses the L2's chain ID, username key, and geth client.
func (s *interopE2ESystem) SendL2Tx(
	id string,
	node string,
	sender string,
	applyTxOpts helpers.TxOptsFn,
) *types.Receipt {
	senderSecret := s.UserKey(id, sender)
	require.NotNil(s.t, senderSecret, "no secret found for sender %s", sender)
	nonce, err := s.L2GethClient(id, node).PendingNonceAt(context.Background(), crypto.PubkeyToAddress(senderSecret.PublicKey))
	require.NoError(s.t, err, "failed to get nonce")
	newApply := func(opts *helpers.TxOpts) {
		applyTxOpts(opts)
		opts.Nonce = nonce
	}
	return helpers.SendL2TxWithID(
		s.t,
		s.l2s[id].chainID,
		s.L2GethClient(id, node),
		&senderSecret,
		newApply)
}

// ValidateMessage calls the CrossL2Inbox ValidateMessage function
// it uses the L2's chain ID, username key, and geth client.
// expectedError represents the error returned by `ValidateMessage` if it is expected.
// the returned err is related to `WaitMined`
func (s *interopE2ESystem) ValidateMessage(
	ctx context.Context,
	id string,
	sender string,
	msgIdentifier supervisortypes.Identifier,
	msgHash [32]byte,
	expectedError error,
) (*types.Receipt, error) {
	secret := s.UserKey(id, sender)
	auth, err := bind.NewKeyedTransactorWithChainID(&secret, s.l2s[id].chainID)
	contract := s.Contract(id, "inbox").(*inbox.Inbox)

	require.NoError(s.t, err)

	auth.GasLimit = uint64(3000_000)
	auth.GasFeeCap = big.NewInt(21_000_000_000)
	auth.GasTipCap = big.NewInt(1_000_000_000)

	identifier := inbox.Identifier{
		Origin:      msgIdentifier.Origin,
		BlockNumber: new(big.Int).SetUint64(msgIdentifier.BlockNumber),
		LogIndex:    new(big.Int).SetUint64(uint64(msgIdentifier.LogIndex)),
		Timestamp:   new(big.Int).SetUint64(msgIdentifier.Timestamp),
		ChainId:     msgIdentifier.ChainID.ToBig(),
	}
	access := msgIdentifier.ChecksumArgs(msgHash).Access()
	auth.AccessList = []types.AccessTuple{
		{
			Address:     predeploys.CrossL2InboxAddr,
			StorageKeys: supervisortypes.EncodeAccessList([]supervisortypes.Access{access}),
		},
	}

	tx, err := contract.InboxTransactor.ValidateMessage(auth, identifier, msgHash)
	if expectedError != nil {
		require.ErrorContains(s.t, err, expectedError.Error())
		return nil, err
	} else {
		require.NoError(s.t, err)
	}
	s.logger.Info("Validating message", "tx", tx.Hash(), "to", tx.To(), "data", hexutil.Bytes(tx.Data()))
	return bind.WaitMined(ctx, s.L2GethClient(id, "sequencer"), tx) // use the sequencer client to wait for the tx
}

// DeployEmitterContract deploys the Emitter contract on the L2
// it uses the sequencer node to deploy the contract
func (s *interopE2ESystem) DeployEmitterContract(
	ctx context.Context,
	id string,
	sender string,
) common.Address {
	secret := s.UserKey(id, sender)
	auth, err := bind.NewKeyedTransactorWithChainID(&secret, s.l2s[id].chainID)
	require.NoError(s.t, err)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = big.NewInt(20000000000)
	address, tx, _, err := emit.DeployEmit(auth, s.L2GethClient(id, "sequencer"))
	require.NoError(s.t, err)
	_, err = bind.WaitMined(ctx, s.L2GethClient(id, "sequencer"), tx)
	require.NoError(s.t, err)
	contract, err := emit.NewEmit(address, s.L2GethClient(id, "sequencer"))
	require.NoError(s.t, err)
	s.l2s[id].contracts["emitter"] = contract
	return address
}

func (s *interopE2ESystem) EmitData(
	ctx context.Context,
	id string,
	node string,
	sender string,
	data string,
) *types.Receipt {
	secret := s.UserKey(id, sender)
	auth, err := bind.NewKeyedTransactorWithChainID(&secret, s.l2s[id].chainID)

	require.NoError(s.t, err)

	auth.GasLimit = uint64(3000000)
	auth.GasPrice = big.NewInt(20000000000)

	contract := s.Contract(id, "emitter").(*emit.Emit)
	tx, err := contract.EmitTransactor.EmitData(auth, []byte(data))
	require.NoError(s.t, err)
	receipt, err := bind.WaitMined(ctx, s.L2GethClient(id, node), tx)
	require.NoError(s.t, err)
	return receipt
}

func (s *interopE2ESystem) Contract(id string, name string) interface{} {
	return s.l2s[id].contracts[name]
}

func (s *interopE2ESystem) DependencySet() *depset.StaticConfigDependencySet {
	stDepSet, err := worldToDepSet(s.worldOutput)
	require.NoError(s.t, err)
	return stDepSet
}

func mustDial(t *testing.T, logger log.Logger) func(v string) *rpc.Client {
	return func(v string) *rpc.Client {
		cl, err := dial.DialRPCClientWithTimeout(context.Background(), logger, v)
		require.NoError(t, err, "failed to dial")
		return cl
	}
}

var testingJWTSecret = [32]byte{123}

func writeDefaultJWT(t testing.TB) string {
	// Sadly the geth node config cannot load JWT secret from memory, it has to be a file
	jwtPath := path.Join(t.TempDir(), "jwt_secret")
	if err := os.WriteFile(jwtPath, []byte(hexutil.Encode(testingJWTSecret[:])), 0o600); err != nil {
		t.Fatalf("failed to prepare jwt file for geth: %v", err)
	}
	return jwtPath
}

func worldToDepSet(world *interopgen.WorldOutput) (*depset.StaticConfigDependencySet, error) {
	var ids []eth.ChainID
	for _, l2Out := range world.L2s {
		chainID := eth.ChainIDFromBig(l2Out.Genesis.Config.ChainID)
		ids = append(ids, chainID)
	}
	eth.SortChainID(ids)
	depSet := make(map[eth.ChainID]*depset.StaticConfigDependency)

	// Iterate over the L2 chain configs. The L2 nodes don't exist yet.
	for _, l2Out := range world.L2s {
		chainID := eth.ChainIDFromBig(l2Out.Genesis.Config.ChainID)
		depSet[chainID] = &depset.StaticConfigDependency{}
	}
	return depset.NewStaticConfigDependencySet(depSet)
}

func worldToFullCfgSet(world *interopgen.WorldOutput) (depset.FullConfigSetMerged, error) {
	depSet, err := worldToDepSet(world)
	if err != nil {
		return depset.FullConfigSetMerged{}, err
	}
	return depset.NewFullConfigSetMerged(world.RollupConfigSet(), depSet)
}
