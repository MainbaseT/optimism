package derive

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	// L1Block Parameters
	deployIsthmusL1BlockSource      = UpgradeDepositSource{Intent: "Isthmus: L1 Block Deployment"}
	updateIsthmusL1BlockProxySource = UpgradeDepositSource{Intent: "Isthmus: L1 Block Proxy Update"}
	L1BlockIsthmusDeployerAddress   = common.HexToAddress("0x4210000000000000000000000000000000000003")
	isthmusL1BlockAddress           = crypto.CreateAddress(L1BlockIsthmusDeployerAddress, 0)

	// Gas Price Oracle Parameters
	deployIsthmusGasPriceOracleSource    = UpgradeDepositSource{Intent: "Isthmus: Gas Price Oracle Deployment"}
	updateIsthmusGasPriceOracleSource    = UpgradeDepositSource{Intent: "Isthmus: Gas Price Oracle Proxy Update"}
	GasPriceOracleIsthmusDeployerAddress = common.HexToAddress("0x4210000000000000000000000000000000000004")
	isthmusGasPriceOracleAddress         = crypto.CreateAddress(GasPriceOracleIsthmusDeployerAddress, 0)

	// Operator fee vault Parameters
	deployOperatorFeeVaultSource    = UpgradeDepositSource{Intent: "Isthmus: Operator Fee Vault Deployment"}
	updateOperatorFeeVaultSource    = UpgradeDepositSource{Intent: "Isthmus: Operator Fee Vault Proxy Update"}
	OperatorFeeVaultDeployerAddress = common.HexToAddress("0x4210000000000000000000000000000000000005")
	OperatorFeeVaultAddress         = crypto.CreateAddress(OperatorFeeVaultDeployerAddress, 0)

	// Bytecodes
	l1BlockIsthmusDeploymentBytecode        = common.FromHex("0x608060405234801561001057600080fd5b506106ae806100206000396000f3fe608060405234801561001057600080fd5b50600436106101825760003560e01c806364ca23ef116100d8578063b80777ea1161008c578063e591b28211610066578063e591b282146103b0578063e81b2c6d146103d2578063f8206140146103db57600080fd5b8063b80777ea14610337578063c598591814610357578063d84447151461037757600080fd5b80638381f58a116100bd5780638381f58a146103115780638b239f73146103255780639e8c49661461032e57600080fd5b806364ca23ef146102e157806368d5dca6146102f557600080fd5b80634397dfef1161013a57806354fd4d501161011457806354fd4d501461025d578063550fcdc91461029f5780635cf24969146102d857600080fd5b80634397dfef146101fc578063440a5e20146102245780634d5d9a2a1461022c57600080fd5b806309bd5a601161016b57806309bd5a60146101a457806316d3bc7f146101c057806321326849146101ed57600080fd5b8063015d8eb914610187578063098999be1461019c575b600080fd5b61019a6101953660046105bc565b6103e4565b005b61019a610523565b6101ad60025481565b6040519081526020015b60405180910390f35b6008546101d49067ffffffffffffffff1681565b60405167ffffffffffffffff90911681526020016101b7565b604051600081526020016101b7565b6040805173eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee815260126020820152016101b7565b61019a61052d565b6008546102489068010000000000000000900463ffffffff1681565b60405163ffffffff90911681526020016101b7565b60408051808201909152600581527f312e362e3000000000000000000000000000000000000000000000000000000060208201525b6040516101b7919061062e565b60408051808201909152600381527f45544800000000000000000000000000000000000000000000000000000000006020820152610292565b6101ad60015481565b6003546101d49067ffffffffffffffff1681565b6003546102489068010000000000000000900463ffffffff1681565b6000546101d49067ffffffffffffffff1681565b6101ad60055481565b6101ad60065481565b6000546101d49068010000000000000000900467ffffffffffffffff1681565b600354610248906c01000000000000000000000000900463ffffffff1681565b60408051808201909152600581527f45746865720000000000000000000000000000000000000000000000000000006020820152610292565b60405173deaddeaddeaddeaddeaddeaddeaddeaddead000181526020016101b7565b6101ad60045481565b6101ad60075481565b3373deaddeaddeaddeaddeaddeaddeaddeaddead00011461048b576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603b60248201527f4c31426c6f636b3a206f6e6c7920746865206465706f7369746f72206163636f60448201527f756e742063616e20736574204c3120626c6f636b2076616c7565730000000000606482015260840160405180910390fd5b6000805467ffffffffffffffff98891668010000000000000000027fffffffffffffffffffffffffffffffff00000000000000000000000000000000909116998916999099179890981790975560019490945560029290925560038054919094167fffffffffffffffffffffffffffffffffffffffffffffffff00000000000000009190911617909255600491909155600555600655565b61052b610535565b565b61052b610548565b61053d610548565b60a43560a01c600855565b73deaddeaddeaddeaddeaddeaddeaddeaddead000133811461057257633cc50b456000526004601cfd5b60043560801c60035560143560801c60005560243560015560443560075560643560025560843560045550565b803567ffffffffffffffff811681146105b757600080fd5b919050565b600080600080600080600080610100898b0312156105d957600080fd5b6105e28961059f565b97506105f060208a0161059f565b9650604089013595506060890135945061060c60808a0161059f565b979a969950949793969560a0850135955060c08501359460e001359350915050565b600060208083528351808285015260005b8181101561065b5785810183015185820160400152820161063f565b8181111561066d576000604083870101525b50601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe01692909201604001939250505056fea164736f6c634300080f000a")
	gasPriceOracleIsthmusDeploymentBytecode = common.FromHex("0x608060405234801561001057600080fd5b50611c3c806100206000396000f3fe608060405234801561001057600080fd5b50600436106101775760003560e01c806368d5dca6116100d8578063c59859181161008c578063f45e65d811610066578063f45e65d8146102ca578063f8206140146102d2578063fe173b971461026957600080fd5b8063c59859181461029c578063de26c4a1146102a4578063f1c7a58b146102b757600080fd5b80638e98b106116100bd5780638e98b1061461026f578063960e3a2314610277578063b54501bc1461028957600080fd5b806368d5dca61461024c5780636ef25c3a1461026957600080fd5b8063313ce5671161012f5780634ef6e224116101145780634ef6e224146101de578063519b4bd3146101fb57806354fd4d501461020357600080fd5b8063313ce567146101c457806349948e0e146101cb57600080fd5b8063275aedd211610160578063275aedd2146101a1578063291b0383146101b45780632e0f2625146101bc57600080fd5b80630c18c1621461017c57806322b90ab314610197575b600080fd5b6101846102da565b6040519081526020015b60405180910390f35b61019f6103fb565b005b6101846101af36600461168e565b610584565b61019f61070f565b610184600681565b6006610184565b6101846101d93660046116d6565b610937565b6000546101eb9060ff1681565b604051901515815260200161018e565b61018461096e565b61023f6040518060400160405280600581526020017f312e342e3000000000000000000000000000000000000000000000000000000081525081565b60405161018e91906117a5565b6102546109cf565b60405163ffffffff909116815260200161018e565b48610184565b61019f610a54565b6000546101eb90610100900460ff1681565b6000546101eb9062010000900460ff1681565b610254610c4e565b6101846102b23660046116d6565b610caf565b6101846102c536600461168e565b610da9565b610184610e85565b610184610f78565b6000805460ff1615610373576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602860248201527f47617350726963654f7261636c653a206f76657268656164282920697320646560448201527f707265636174656400000000000000000000000000000000000000000000000060648201526084015b60405180910390fd5b73420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff16638b239f736040518163ffffffff1660e01b8152600401602060405180830381865afa1580156103d2573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906103f69190611818565b905090565b3373deaddeaddeaddeaddeaddeaddeaddeaddead0001146104c4576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604160248201527f47617350726963654f7261636c653a206f6e6c7920746865206465706f73697460448201527f6f72206163636f756e742063616e2073657420697345636f746f6e6520666c6160648201527f6700000000000000000000000000000000000000000000000000000000000000608482015260a40161036a565b60005460ff1615610557576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602660248201527f47617350726963654f7261636c653a2045636f746f6e6520616c72656164792060448201527f6163746976650000000000000000000000000000000000000000000000000000606482015260840161036a565b600080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00166001179055565b6000805462010000900460ff1661059d57506000919050565b610709620f42406106668473420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff16634d5d9a2a6040518163ffffffff1660e01b8152600401602060405180830381865afa158015610607573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061062b9190611831565b63ffffffff167fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff821583830293840490921491909117011790565b6106709190611886565b73420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff166316d3bc7f6040518163ffffffff1660e01b8152600401602060405180830381865afa1580156106cf573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906106f391906118c1565b67ffffffffffffffff1681019081106000031790565b92915050565b3373deaddeaddeaddeaddeaddeaddeaddeaddead0001146107d8576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604160248201527f47617350726963654f7261636c653a206f6e6c7920746865206465706f73697460448201527f6f72206163636f756e742063616e20736574206973497374686d757320666c6160648201527f6700000000000000000000000000000000000000000000000000000000000000608482015260a40161036a565b600054610100900460ff1661086f576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603960248201527f47617350726963654f7261636c653a20497374686d75732063616e206f6e6c7960448201527f2062652061637469766174656420616674657220466a6f726400000000000000606482015260840161036a565b60005462010000900460ff1615610908576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602660248201527f47617350726963654f7261636c653a20497374686d757320616c72656164792060448201527f6163746976650000000000000000000000000000000000000000000000000000606482015260840161036a565b600080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00ffff1662010000179055565b60008054610100900460ff16156109515761070982610fd9565b60005460ff16156109655761070982610ff8565b6107098261109c565b600073420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff16635cf249696040518163ffffffff1660e01b8152600401602060405180830381865afa1580156103d2573d6000803e3d6000fd5b600073420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff166368d5dca66040518163ffffffff1660e01b8152600401602060405180830381865afa158015610a30573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906103f69190611831565b3373deaddeaddeaddeaddeaddeaddeaddeaddead000114610af7576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603f60248201527f47617350726963654f7261636c653a206f6e6c7920746865206465706f73697460448201527f6f72206163636f756e742063616e20736574206973466a6f726420666c616700606482015260840161036a565b60005460ff16610b89576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603960248201527f47617350726963654f7261636c653a20466a6f72642063616e206f6e6c79206260448201527f65206163746976617465642061667465722045636f746f6e6500000000000000606482015260840161036a565b600054610100900460ff1615610c20576040517f08c379a0000000000000000000000000000000000000000000000000000000008152602060048201526024808201527f47617350726963654f7261636c653a20466a6f726420616c726561647920616360448201527f7469766500000000000000000000000000000000000000000000000000000000606482015260840161036a565b600080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00ff16610100179055565b600073420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff1663c59859186040518163ffffffff1660e01b8152600401602060405180830381865afa158015610a30573d6000803e3d6000fd5b60008054610100900460ff1615610cf657620f4240610ce1610cd0846111f0565b51610cdc9060446118eb565b61150d565b610cec906010611903565b6107099190611886565b6000610d018361156c565b60005490915060ff1615610d155792915050565b73420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff16638b239f736040518163ffffffff1660e01b8152600401602060405180830381865afa158015610d74573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610d989190611818565b610da290826118eb565b9392505050565b60008054610100900460ff16610e41576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603660248201527f47617350726963654f7261636c653a206765744c314665655570706572426f7560448201527f6e64206f6e6c7920737570706f72747320466a6f726400000000000000000000606482015260840161036a565b6000610e4e8360446118eb565b90506000610e5d60ff83611886565b610e6790836118eb565b610e729060106118eb565b9050610e7d816115fc565b949350505050565b6000805460ff1615610f19576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602660248201527f47617350726963654f7261636c653a207363616c61722829206973206465707260448201527f6563617465640000000000000000000000000000000000000000000000000000606482015260840161036a565b73420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff16639e8c49666040518163ffffffff1660e01b8152600401602060405180830381865afa1580156103d2573d6000803e3d6000fd5b600073420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff1663f82061406040518163ffffffff1660e01b8152600401602060405180830381865afa1580156103d2573d6000803e3d6000fd5b6000610709610fe7836111f0565b51610ff39060446118eb565b6115fc565b6000806110048361156c565b9050600061101061096e565b611018610c4e565b611023906010611940565b63ffffffff166110339190611903565b9050600061103f610f78565b6110476109cf565b63ffffffff166110579190611903565b9050600061106582846118eb565b61106f9085611903565b905061107d6006600a611a8c565b611088906010611903565b6110929082611886565b9695505050505050565b6000806110a88361156c565b9050600073420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff16639e8c49666040518163ffffffff1660e01b8152600401602060405180830381865afa15801561110b573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061112f9190611818565b61113761096e565b73420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff16638b239f736040518163ffffffff1660e01b8152600401602060405180830381865afa158015611196573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906111ba9190611818565b6111c490856118eb565b6111ce9190611903565b6111d89190611903565b90506111e66006600a611a8c565b610e7d9082611886565b606061137f565b818153600101919050565b600082840393505b83811015610da25782810151828201511860001a159093029260010161120a565b825b60208210611277578251611242601f836111f7565b52602092909201917fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe09091019060210161122d565b8115610da257825161128c60018403836111f7565b520160010192915050565b60006001830392505b61010782106112d8576112ca8360ff166112c560fd6112c58760081c60e001896111f7565b6111f7565b9350610106820391506112a0565b60078210611305576112fe8360ff166112c5600785036112c58760081c60e001896111f7565b9050610da2565b610e7d8360ff166112c58560081c8560051b01876111f7565b61137782820361135b61134b84600081518060001a8160011a60081b178160021a60101b17915050919050565b639e3779b90260131c611fff1690565b8060021b6040510182815160e01c1860e01b8151188152505050565b600101919050565b6180003860405139618000604051016020830180600d8551820103826002015b818110156114b2576000805b50508051604051600082901a600183901a60081b1760029290921a60101b91909117639e3779b9810260111c617ffc16909101805160e081811c878603811890911b909118909152840190818303908484106114075750611442565b600184019350611fff821161143c578251600081901a600182901a60081b1760029190911a60101b17810361143c5750611442565b506113ab565b8383106114505750506114b2565b6001830392508583111561146e5761146b878788860361122b565b96505b611482600985016003850160038501611202565b915061148f878284611297565b9650506114a7846114a28684860161131e565b61131e565b91505080935061139f565b50506114c4838384885185010361122b565b925050506040519150618000820180820391508183526020830160005b838110156114f95782810151828201526020016114e1565b506000920191825250602001604052919050565b60008061151d83620cc394611903565b611547907ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffd763200611a98565b90506115576064620f4240611b0c565b81121561070957610da26064620f4240611b0c565b80516000908190815b818110156115ef5784818151811061158f5761158f611bc8565b01602001517fff00000000000000000000000000000000000000000000000000000000000000166000036115cf576115c86004846118eb565b92506115dd565b6115da6010846118eb565b92505b806115e781611bf7565b915050611575565b50610e7d826104406118eb565b6000806116088361150d565b90506000611614610f78565b61161c6109cf565b63ffffffff1661162c9190611903565b61163461096e565b61163c610c4e565b611647906010611940565b63ffffffff166116579190611903565b61166191906118eb565b905061166f60066002611903565b61167a90600a611a8c565b6116848284611903565b610e7d9190611886565b6000602082840312156116a057600080fd5b5035919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b6000602082840312156116e857600080fd5b813567ffffffffffffffff8082111561170057600080fd5b818401915084601f83011261171457600080fd5b813581811115611726576117266116a7565b604051601f82017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0908116603f0116810190838211818310171561176c5761176c6116a7565b8160405282815287602084870101111561178557600080fd5b826020860160208301376000928101602001929092525095945050505050565b600060208083528351808285015260005b818110156117d2578581018301518582016040015282016117b6565b818111156117e4576000604083870101525b50601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe016929092016040019392505050565b60006020828403121561182a57600080fd5b5051919050565b60006020828403121561184357600080fd5b815163ffffffff81168114610da257600080fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b6000826118bc577f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fd5b500490565b6000602082840312156118d357600080fd5b815167ffffffffffffffff81168114610da257600080fd5b600082198211156118fe576118fe611857565b500190565b6000817fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff048311821515161561193b5761193b611857565b500290565b600063ffffffff8083168185168183048111821515161561196357611963611857565b02949350505050565b600181815b808511156119c557817fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff048211156119ab576119ab611857565b808516156119b857918102915b93841c9390800290611971565b509250929050565b6000826119dc57506001610709565b816119e957506000610709565b81600181146119ff5760028114611a0957611a25565b6001915050610709565b60ff841115611a1a57611a1a611857565b50506001821b610709565b5060208310610133831016604e8410600b8410161715611a48575081810a610709565b611a52838361196c565b807fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff04821115611a8457611a84611857565b029392505050565b6000610da283836119cd565b6000808212827f7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff03841381151615611ad257611ad2611857565b827f8000000000000000000000000000000000000000000000000000000000000000038412811615611b0657611b06611857565b50500190565b60007f7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff600084136000841385830485118282161615611b4d57611b4d611857565b7f80000000000000000000000000000000000000000000000000000000000000006000871286820588128184161615611b8857611b88611857565b60008712925087820587128484161615611ba457611ba4611857565b87850587128184161615611bba57611bba611857565b505050929093029392505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b60007fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8203611c2857611c28611857565b506001019056fea164736f6c634300080f000a")
	operatorFeeVaultDeploymentBytecode      = common.FromHex("0x60e060405234801561001057600080fd5b5073420000000000000000000000000000000000001960a0526000608052600160c05260805160a05160c0516107ef6100a7600039600081816101b3015281816102450152818161044b015261048601526000818160b8015281816101800152818161039a01528181610429015281816104c201526105b70152600081816101ef01528181610279015261029d01526107ef6000f3fe60806040526004361061009a5760003560e01c806382356d8a1161006957806384411d651161004e57806384411d651461021d578063d0e12f9014610233578063d3e5792b1461026757600080fd5b806382356d8a146101a45780638312f149146101e057600080fd5b80630d9019e1146100a65780633ccfd60b1461010457806354fd4d501461011b57806366d003ac1461017157600080fd5b366100a157005b600080fd5b3480156100b257600080fd5b506100da7f000000000000000000000000000000000000000000000000000000000000000081565b60405173ffffffffffffffffffffffffffffffffffffffff90911681526020015b60405180910390f35b34801561011057600080fd5b5061011961029b565b005b34801561012757600080fd5b506101646040518060400160405280600581526020017f312e302e3000000000000000000000000000000000000000000000000000000081525081565b6040516100fb9190610671565b34801561017d57600080fd5b507f00000000000000000000000000000000000000000000000000000000000000006100da565b3480156101b057600080fd5b507f00000000000000000000000000000000000000000000000000000000000000005b6040516100fb919061074e565b3480156101ec57600080fd5b507f00000000000000000000000000000000000000000000000000000000000000005b6040519081526020016100fb565b34801561022957600080fd5b5061020f60005481565b34801561023f57600080fd5b506101d37f000000000000000000000000000000000000000000000000000000000000000081565b34801561027357600080fd5b5061020f7f000000000000000000000000000000000000000000000000000000000000000081565b7f0000000000000000000000000000000000000000000000000000000000000000471015610376576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152604a60248201527f4665655661756c743a207769746864726177616c20616d6f756e74206d75737460448201527f2062652067726561746572207468616e206d696e696d756d207769746864726160648201527f77616c20616d6f756e7400000000000000000000000000000000000000000000608482015260a4015b60405180910390fd5b60004790508060008082825461038c9190610762565b9091555050604080518281527f000000000000000000000000000000000000000000000000000000000000000073ffffffffffffffffffffffffffffffffffffffff166020820152338183015290517fc8a211cc64b6ed1b50595a9fcb1932b6d1e5a6e8ef15b60e5b1f988ea9086bba9181900360600190a17f38e04cbeb8c10f8f568618aa75be0f10b6729b8b4237743b4de20cbcde2839ee817f0000000000000000000000000000000000000000000000000000000000000000337f000000000000000000000000000000000000000000000000000000000000000060405161047a94939291906107a1565b60405180910390a160017f000000000000000000000000000000000000000000000000000000000000000060018111156104b6576104b66106e4565b0361057a5760006104e77f000000000000000000000000000000000000000000000000000000000000000083610649565b905080610576576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152603060248201527f4665655661756c743a206661696c656420746f2073656e642045544820746f2060448201527f4c322066656520726563697069656e7400000000000000000000000000000000606482015260840161036d565b5050565b6040517fc2b3e5ac00000000000000000000000000000000000000000000000000000000815273ffffffffffffffffffffffffffffffffffffffff7f000000000000000000000000000000000000000000000000000000000000000016600482015262061a80602482015260606044820152600060648201527342000000000000000000000000000000000000169063c2b3e5ac9083906084016000604051808303818588803b15801561062d57600080fd5b505af1158015610641573d6000803e3d6000fd5b505050505050565b6000610656835a8461065d565b9392505050565b6000806000806000858888f1949350505050565b600060208083528351808285015260005b8181101561069e57858101830151858201604001528201610682565b818111156106b0576000604083870101525b50601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe016929092016040019392505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602160045260246000fd5b6002811061074a577f4e487b7100000000000000000000000000000000000000000000000000000000600052602160045260246000fd5b9052565b6020810161075c8284610713565b92915050565b6000821982111561079c577f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b500190565b84815273ffffffffffffffffffffffffffffffffffffffff848116602083015283166040820152608081016107d96060830184610713565b9594505050505056fea164736f6c634300080f000a")

	// Enable Isthmus Parameters
	enableIsthmusSource = UpgradeDepositSource{Intent: "Isthmus: Gas Price Oracle Set Isthmus"}
	enableIsthmusInput  = crypto.Keccak256([]byte("setIsthmus()"))[:4]

	blockHashDeployerSource     = UpgradeDepositSource{Intent: "Isthmus: EIP-2935 Contract Deployment"}
	blockHashDeploymentBytecode = common.FromHex("0x60538060095f395ff33373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f35600143038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611fff60014303065500")
)

func IsthmusNetworkUpgradeTransactions() ([]hexutil.Bytes, error) {
	upgradeTxns := make([]hexutil.Bytes, 0, 8)

	// Deploy L1 Block transaction
	deployL1BlockTransaction, err := types.NewTx(&types.DepositTx{
		SourceHash:          deployIsthmusL1BlockSource.SourceHash(),
		From:                L1BlockIsthmusDeployerAddress,
		To:                  nil,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 425_000,
		IsSystemTransaction: false,
		Data:                l1BlockIsthmusDeploymentBytecode,
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, deployL1BlockTransaction)

	// Deploy Gas Price Oracle transaction
	deployGasPriceOracle, err := types.NewTx(&types.DepositTx{
		SourceHash:          deployIsthmusGasPriceOracleSource.SourceHash(),
		From:                GasPriceOracleIsthmusDeployerAddress,
		To:                  nil,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 1_625_000,
		IsSystemTransaction: false,
		Data:                gasPriceOracleIsthmusDeploymentBytecode,
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, deployGasPriceOracle)

	// Deploy Operator Fee vault transaction
	deployOperatorFeeVault, err := types.NewTx(&types.DepositTx{
		SourceHash:          deployOperatorFeeVaultSource.SourceHash(),
		From:                OperatorFeeVaultDeployerAddress,
		To:                  nil,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 500_000,
		IsSystemTransaction: false,
		Data:                operatorFeeVaultDeploymentBytecode,
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, deployOperatorFeeVault)

	// Deploy L1 Block Proxy upgrade transaction
	updateL1BlockProxy, err := types.NewTx(&types.DepositTx{
		SourceHash:          updateIsthmusL1BlockProxySource.SourceHash(),
		From:                common.Address{},
		To:                  &predeploys.L1BlockAddr,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 50_000,
		IsSystemTransaction: false,
		Data:                upgradeToCalldata(isthmusL1BlockAddress),
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, updateL1BlockProxy)

	// Deploy Gas Price Oracle Proxy upgrade transaction
	updateGasPriceOracleProxy, err := types.NewTx(&types.DepositTx{
		SourceHash:          updateIsthmusGasPriceOracleSource.SourceHash(),
		From:                common.Address{},
		To:                  &predeploys.GasPriceOracleAddr,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 50_000,
		IsSystemTransaction: false,
		Data:                upgradeToCalldata(isthmusGasPriceOracleAddress),
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, updateGasPriceOracleProxy)

	// Deploy Operator Fee Vault Proxy upgrade transaction
	updateOperatorFeeVaultProxy, err := types.NewTx(&types.DepositTx{
		SourceHash:          updateOperatorFeeVaultSource.SourceHash(),
		From:                common.Address{},
		To:                  &predeploys.OperatorFeeVaultAddr,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 50_000,
		IsSystemTransaction: false,
		Data:                upgradeToCalldata(OperatorFeeVaultAddress),
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, updateOperatorFeeVaultProxy)

	// Enable Isthmus transaction
	enableIsthmus, err := types.NewTx(&types.DepositTx{
		SourceHash:          enableIsthmusSource.SourceHash(),
		From:                L1InfoDepositerAddress,
		To:                  &predeploys.GasPriceOracleAddr,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 90_000,
		IsSystemTransaction: false,
		Data:                enableIsthmusInput,
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, enableIsthmus)

	deployHistoricalBlockHashesContract, err := types.NewTx(&types.DepositTx{
		SourceHash:          blockHashDeployerSource.SourceHash(),
		From:                predeploys.EIP2935ContractDeployer,
		To:                  nil,
		Mint:                big.NewInt(0),
		Value:               big.NewInt(0),
		Gas:                 250_000,
		IsSystemTransaction: false,
		Data:                blockHashDeploymentBytecode,
	}).MarshalBinary()

	if err != nil {
		return nil, err
	}

	upgradeTxns = append(upgradeTxns, deployHistoricalBlockHashesContract)

	return upgradeTxns, nil
}
