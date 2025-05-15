package env

import (
	"fmt"
	"math/big"
	"net/url"
	"os"
	"strings"

	"github.com/ethereum-optimism/optimism/devnet-sdk/controller/kt"
	"github.com/ethereum-optimism/optimism/devnet-sdk/controller/surface"
	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum/go-ethereum/params"
)

type surfaceGetter func() (surface.ControlSurface, error)
type controllerFactory func(*descriptors.DevnetEnvironment) surfaceGetter

type DevnetEnv struct {
	Env *descriptors.DevnetEnvironment
	URL string

	ctrl surfaceGetter
}

// DataFetcher is a function type for fetching data from a URL
type DataFetcher func(*url.URL) (*descriptors.DevnetEnvironment, error)

type schemeBackend struct {
	fetcher     DataFetcher
	ctrlFactory controllerFactory
}

func getKurtosisController(env *descriptors.DevnetEnvironment) surfaceGetter {
	return func() (surface.ControlSurface, error) {
		return kt.NewKurtosisControllerSurface(env)
	}
}

var (
	ktFetcher = &kurtosisFetcher{
		devnetFSFactory: newDevnetFS,
	}

	// schemeToBackend maps URL schemes to their respective data fetcher functions
	schemeToBackend = map[string]schemeBackend{
		"":         {fetchFileData, nil},
		"file":     {fetchFileData, nil},
		"kt":       {ktFetcher.fetchKurtosisData, getKurtosisController},
		"ktnative": {fetchKurtosisNativeData, getKurtosisController},
	}
)

// fetchDevnetData retrieves data from a URL based on its scheme
func fetchDevnetData(parsedURL *url.URL) (*descriptors.DevnetEnvironment, error) {
	scheme := strings.ToLower(parsedURL.Scheme)
	backend, ok := schemeToBackend[scheme]
	if !ok {
		return nil, fmt.Errorf("unsupported URL scheme: %s", scheme)
	}

	return backend.fetcher(parsedURL)
}

func LoadDevnetFromURL(devnetURL string) (*DevnetEnv, error) {
	parsedURL, err := url.Parse(devnetURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}

	env, err := fetchDevnetData(parsedURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching devnet data: %w", err)
	}

	if err := fixupDevnetConfig(env); err != nil {
		return nil, fmt.Errorf("error fixing up devnet config: %w", err)
	}

	var ctrl surfaceGetter
	scheme := parsedURL.Scheme
	if val, ok := os.LookupEnv(EnvCtrlVar); ok {
		scheme = val
	}
	backend, ok := schemeToBackend[scheme]
	if !ok {
		return nil, fmt.Errorf("invalid scheme to lookup control interface: %s", scheme)
	}

	if backend.ctrlFactory != nil {
		ctrl = backend.ctrlFactory(env)
	}

	return &DevnetEnv{
		Env:  env,
		URL:  devnetURL,
		ctrl: ctrl,
	}, nil
}

func (d *DevnetEnv) GetChain(chainName string) (*ChainConfig, error) {
	var chain *descriptors.Chain
	if d.Env.L1.Name == chainName {
		chain = d.Env.L1
	} else {
		for _, l2Chain := range d.Env.L2 {
			if l2Chain.Name == chainName {
				chain = l2Chain.Chain
				break
			}
		}
	}

	if chain == nil {
		return nil, fmt.Errorf("chain '%s' not found in devnet config", chainName)
	}

	return &ChainConfig{
		chain:     chain,
		devnetURL: d.URL,
		name:      chainName,
	}, nil
}

func (d *DevnetEnv) Control() (surface.ControlSurface, error) {
	if d.ctrl == nil {
		return nil, fmt.Errorf("devnet is not controllable")
	}
	return d.ctrl()
}

func fixupDevnetConfig(config *descriptors.DevnetEnvironment) error {
	// we should really get this from the kurtosis output, but the data doesn't exist yet, so craft a minimal one.
	if config.L1.Config == nil {
		l1ID := new(big.Int)
		l1ID, ok := l1ID.SetString(config.L1.ID, 10)
		if !ok {
			return fmt.Errorf("invalid L1 ID: %s", config.L1.ID)
		}
		config.L1.Config = &params.ChainConfig{
			ChainID: l1ID,
		}
	}
	return nil
}
