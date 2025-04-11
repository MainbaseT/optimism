package opcm_test

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestNewDeployProxyScript(t *testing.T) {
	t.Run("should not fail with current version of DeployProxy2 contract", func(t *testing.T) {
		// First we grab a test host
		host1 := createTestHost(t)

		// Then we load the script
		//
		// This would raise an error if the Go types didn't match the ABI
		deployProxy, err := opcm.NewDeployProxyScript(host1)
		require.NoError(t, err)

		// Then we deploy
		output, err := deployProxy.Run(opcm.DeployProxy2Input{
			Owner: common.Address{'O'},
		})

		// And do some simple asserts
		require.NoError(t, err)
		require.NotNil(t, output)

		// Now we run the old deployer
		//
		// We run it on a fresh host so that the deployer nonces are the same
		// which in turn means we should get identical output
		host2 := createTestHost(t)
		deprecatedOutput, err := opcm.DeployProxy(host2, opcm.DeployProxyInput{
			Owner: common.Address{'O'},
		})

		// Make sure it succeeded
		require.NoError(t, err)
		require.NotNil(t, deprecatedOutput)

		// Now make sure the addresses are the same
		require.Equal(t, deprecatedOutput.Proxy, output.Proxy)

		// And just to be super sure we also compare the code deployed to the addresses
		require.Equal(t, host2.GetCode(deprecatedOutput.Proxy), host1.GetCode(output.Proxy))
	})
}
