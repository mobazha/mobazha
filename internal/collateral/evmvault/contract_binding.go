// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package evmvault

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// CollateralVaultObligation is an auto generated low-level Go binding around an user-defined struct.
type CollateralVaultObligation struct {
	Principal      common.Address
	ExpiresAt      uint64
	State          uint8
	RequiredAmount *big.Int
	Balance        *big.Int
}

// CollateralVaultMetaData contains all meta data concerning the CollateralVault contract.
var CollateralVaultMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"assetAddress\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"admin\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"ActionConflict\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"AmountBelowRequired\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"FundingExpired\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InsufficientCollateral\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InsufficientSurplus\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidAmount\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidExpiry\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidObligationState\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidPrincipalDestination\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotPrincipal\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ObligationConflict\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ObligationNotFound\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UnsupportedTokenBehavior\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZeroAddress\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ZeroIdentifier\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"collateralId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"fundingId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"principal\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalBalance\",\"type\":\"uint256\"}],\"name\":\"CollateralFunded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"collateralId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"actionId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"principal\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"CollateralReleased\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"collateralId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"actionId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"beneficiary\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"remainingBalance\",\"type\":\"uint256\"}],\"name\":\"CollateralSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"collateralId\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"principal\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"requiredAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"expiresAt\",\"type\":\"uint64\"}],\"name\":\"ObligationCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Paused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"previousAdminRole\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"newAdminRole\",\"type\":\"bytes32\"}],\"name\":\"RoleAdminChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"}],\"name\":\"RoleGranted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"}],\"name\":\"RoleRevoked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"SurplusRecovered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Unpaused\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"DEFAULT_ADMIN_ROLE\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"OPERATOR_ROLE\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"PAUSER_ROLE\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"VERSION\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"actionDigests\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"asset\",\"outputs\":[{\"internalType\":\"contractIERC20\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"collateralId\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"principal\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"requiredAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint64\",\"name\":\"expiresAt\",\"type\":\"uint64\"}],\"name\":\"createObligation\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"collateralId\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"fundingId\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"fund\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"collateralId\",\"type\":\"bytes32\"}],\"name\":\"getObligation\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"principal\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"expiresAt\",\"type\":\"uint64\"},{\"internalType\":\"enumCollateralVault.ObligationState\",\"name\":\"state\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"requiredAmount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"balance\",\"type\":\"uint256\"}],\"internalType\":\"structCollateralVault.Obligation\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"}],\"name\":\"getRoleAdmin\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"grantRole\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"hasRole\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"pause\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"recoverSurplus\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"collateralId\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"actionId\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"destination\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"release\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"renounceRole\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"role\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"revokeRole\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"collateralId\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"actionId\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"beneficiary\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"slash\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes4\",\"name\":\"interfaceId\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalLocked\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"unpause\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x60a0346200025b57601f6200218f38819003918201601f191683019291906001600160401b03841183851017620002605781606092849260409687528339810103126200025b57620000518162000276565b6020906200006e846200006684860162000276565b940162000276565b6001805460ff199081168255600282905591936001600160a01b0393808516908115801562000250575b801562000245575b62000234573b15620002235760805260009586805286825284888820911690818852825260ff888820541615620001ee575b7f65d7a28e3265b37a6474929f336521b332c1681b933f6cb9f3376673440d862a90818852878352888820818952835260ff898920541615620001b8575b50507f97667070c54ef182b0f5858b034beac1b6f3089aa2d3188bb1e8929f4fa9b92993848752868252878720951694858752815260ff87872054161562000180575b8651611ee390816200028c82396080518181816105a0015281816107dc01528181610fe501526118390152f35b8386528581528686209085875252858520918254161790556000805160206200216f833981519152339380a438808080808062000153565b8188528783528888208189528352888820848682541617905533916000805160206200216f8339815191528980a4388062000110565b868052868252878720818852825287872083858254161790553381886000805160206200216f8339815191528180a4620000d2565b875163d92e233d60e01b8152600490fd5b885163d92e233d60e01b8152600490fd5b5085871615620000a0565b508588161562000098565b600080fd5b634e487b7160e01b600052604160045260246000fd5b51906001600160a01b03821682036200025b5756fe608060408181526004918236101561001657600080fd5b600092833560e01c91826301ffc9a714610a9a575081630e57c69a14610767578163248a9ca31461073d5781632f2ff15d1461069457816330c090021461066157816336568abe146105cf57816338d52e0f1461058b5781633f4ba83a146104f557816356891412146104d65781635c975abb146104b25781637f9e0fd2146103b85781638456cb591461035e57816385a9549a1461032857816391d14854146102e25781639e9a8003146102ba578163a082271b1461027d578163a217fddf14610262578163c03aa3fb14610210578163d547741f146101ce57508063e63ab1e914610194578063f5b541a61461015a5763ffa1ad741461011757600080fd5b3461015657816003193601126101565780516101529161013682610b67565b60058252640312e302e360dc1b60208301525191829182610bfd565b0390f35b5080fd5b5034610156578160031936011261015657602090517f97667070c54ef182b0f5858b034beac1b6f3089aa2d3188bb1e8929f4fa9b9298152f35b5034610156578160031936011261015657602090517f65d7a28e3265b37a6474929f336521b332c1681b933f6cb9f3376673440d862a8152f35b9190503461020c578060031936011261020c57610209913561020460016101f3610aed565b9383875286602052862001546116d9565b611b2b565b80f35b8280fd5b8390346101565760803660031901126101565761022b610aed565b90606435906001600160401b038216820361025e576102099261024c6113a8565b610254611a6e565b60443591356111ca565b8380fd5b50503461015657816003193601126101565751908152602090f35b839034610156576060366003190112610156576102b29061029c6117e3565b6102a4611a6e565b604435906024359035610f18565b600160025580f35b90503461020c57602036600319011261020c5760209282913581526005845220549051908152f35b90503461020c578160031936011261020c578160209360ff92610303610aed565b903582528186528282206001600160a01b039091168252855220549151911615158152f35b833461035b576102b261033a36610b08565b926103469291926113a8565b61034e6117e3565b610356611a6e565b610dcc565b80fd5b50503461015657816003193601126101565760207f62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a2589161039c611592565b6103a4611a6e565b600160ff198154161760015551338152a180f35b8383346101565760203660031901126101565781608082516103d981610b36565b828152826020820152828482015282606082015201526103f98335611ab2565b81519261040584610b36565b81546001600160a01b03808216865260a082901c6001600160401b0390811660208801908152919592878401929060e01c60ff16600581101561049f578352600260018701549660608a019788520154966080890197885284519851168852511660208701525191600583101561048c575060a09550840152516060830152516080820152f35b634e487b7160e01b815260218752602490fd5b634e487b7160e01b865260218a52602486fd5b50503461015657816003193601126101565760209060ff6001541690519015158152f35b5050346101565781600319360112610156576020906003549051908152f35b90503461020c578260031936011261020c5761050f611592565b6001549060ff821615610551575060ff1916600155513381527f5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa90602090a180f35b606490602084519162461bcd60e51b8352820152601460248201527314185d5cd8589b194e881b9bdd081c185d5cd95960621b6044820152fd5b505034610156578160031936011261015657517f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03168152602090f35b839150346101565782600319360112610156576105ea610aed565b90336001600160a01b0383160361060657906102099135611b2b565b608490602085519162461bcd60e51b8352820152602f60248201527f416363657373436f6e74726f6c3a2063616e206f6e6c792072656e6f756e636560448201526e103937b632b9903337b91039b2b63360891b6064820152fd5b833461035b576102b261067336610b08565b9261067f9291926113a8565b6106876117e3565b61068f611a6e565b610c4c565b90503461020c578160031936011261020c5735906106b0610aed565b90828452836020526106c7600182862001546116d9565b82845260208481528185206001600160a01b039093168086529290528084205460ff16156106f3578380f35b828452836020528084208285526020528320600160ff1982541617905533917f2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d8480a43880808380f35b90503461020c57602036600319011261020c57816020936001923581528085522001549051908152f35b8383346101565780600319360112610156578235926001600160a01b0380851692909190838603610a9657602491823592868052602094878652838820338952865260ff8489205416156108d6576107bd6117e3565b86156108c65785908451928380926370a0823160e01b825230878301527f0000000000000000000000000000000000000000000000000000000000000000165afa9081156108bc57879161088b575b50600354808211156108835761082191610c29565b8315908115610879575b5061086b575061085e827f8d062d570e15e665f57873e5745c84e024c3b6a4c94cfcda6af54a9ccd416af7959697611837565b51908152a2600160025580f35b90516394290ab960e01b8152fd5b905083118861082b565b505085610821565b90508481813d83116108b5575b6108a28183610bb9565b810103126108b157518861080c565b8680fd5b503d610898565b83513d89823e3d90fd5b835163d92e233d60e01b81528390fd5b508685916108e333611d0d565b855191836108f084610b9e565b60428452858401946060368737845115610a84576030865384519060019160011015610a725790607860218701536041915b818311610a09575050506109c857506109c49386936109b0936109a16048946109789a519a8576020b1b1b2b9b9a1b7b73a3937b61d1030b1b1b7bab73a1604d1b8d978801528251928391603789019101610bda565b8401917001034b99036b4b9b9b4b733903937b6329607d1b603784015251809386840190610bda565b01036028810187520185610bb9565b5162461bcd60e51b81529283928301610bfd565b0390fd5b9250505081606494519362461bcd60e51b85528401528201527f537472696e67733a20686578206c656e67746820696e73756666696369656e746044820152fd5b909192600f81166010811015610a60576f181899199a1a9b1b9c1cb0b131b232b360811b901a610a398589611ce6565b53891c928015610a4e57600019019190610922565b634e487b7160e01b825260118a528482fd5b634e487b7160e01b835260328b528583fd5b634e487b7160e01b8152603289528390fd5b634e487b7160e01b8152603288529050fd5b8480fd5b84913461020c57602036600319011261020c573563ffffffff60e01b811680910361020c5760209250637965db0b60e01b8114908115610adc575b5015158152f35b6301ffc9a760e01b14905083610ad5565b602435906001600160a01b0382168203610b0357565b600080fd5b6080906003190112610b035760043590602435906044356001600160a01b0381168103610b03579060643590565b60a081019081106001600160401b03821117610b5157604052565b634e487b7160e01b600052604160045260246000fd5b604081019081106001600160401b03821117610b5157604052565b61010081019081106001600160401b03821117610b5157604052565b608081019081106001600160401b03821117610b5157604052565b90601f801991011681019081106001600160401b03821117610b5157604052565b60005b838110610bed5750506000910152565b8181015183820152602001610bdd565b60409160208252610c1d8151809281602086015260208686019101610bda565b601f01601f1916010190565b91908203918211610c3657565b634e487b7160e01b600052601160045260246000fd5b909192610c5882611ab2565b90604091825195610cb66020880160a08152600560c08a0152640a69882a6960db1b60e08a015286868a01528760608a015260018060a01b038316988960808201528560a082015260e08152610cad81610b82565b51902087611aea565b15610dc35781549160ff8360e01c166005811015610dad5760028114159081610da1575b50610d90578715610d7f5783158015610d72575b610d6157837fb0e4fd6efcc1188105cedb091325e1c42948d7782c4942a36120368ac33827f795949392826002610d53940194610d2c848754610c29565b865560ff60e01b1916600160e21b179055600354610d4b908390610c29565b600355611837565b5482519182526020820152a4565b8451633a23d82560e01b8152600490fd5b5060028101548411610cee565b845163d92e233d60e01b8152600490fd5b8451631cdfeead60e01b8152600490fd5b60049150141538610cda565b634e487b7160e01b600052602160045260246000fd5b50505050505050565b909192610dd882611ab2565b936040908151956020870160a08152600760c08901526652454c4541534560c81b60e08901528584890152866060890152610e3a60018060a01b0391828516998a60808201528760a082015260e08152610e3181610b82565b51902088611aea565b15610f0e5781549060ff8260e01c166005811015610dad5760028114159081610f02575b50610d905781168803610ef15784158015610ee3575b610ed2576000600283015560ff60e01b1916600360e01b1790556003547f258c7762affe3584f997aa5ee6c2e5fccfa6073f1461dfc6382b0d28e286455c9360209390929091610ecb918491610d4b908390610c29565b51908152a4565b835163162908e360e11b8152600490fd5b506002820154851415610e74565b83516374eb39e560e01b8152600490fd5b60049150141538610e5e565b5050505050505050565b610f2181611ab2565b926040908151602095610f6e87830160a081526004938460c0820152631195539160e21b60e082015287878201528860608201523360808201528560a082015260e08152610cad81610b82565b15610dc35780549060ff8260e01c1660058110156111b5576001036111a5576001600160a01b03918083163303611195576001600160401b038160a01c1642116111855760018201548510611175576002820185905560ff60e01b1916600160e11b179055600354808401908110611160576003557f00000000000000000000000000000000000000000000000000000000000000009081168451916370a0823160e01b9081845230858501528984602481865afa93841561115557908a9291600095611121575b5087516323b872dd60e01b848201523360248201523060448201526064808201899052815261106d9161106882610b36565b611b9f565b6024875180948193825230888301525afa90811561111657908492916000916110e1575b509061109c91610c29565b036110d35750907f5be65e993dfcc065578c63c356d3f95047265a5663ce5aa8cd611e73f3690349918151958187528601523394a4565b8251631f75486b60e21b8152fd5b80929350898092503d831161110f575b6110fb8183610bb9565b81010312610b03575183919061109c611091565b503d6110f1565b85513d6000823e3d90fd5b8381949296503d831161114e575b6111398183610bb9565b81010312610b035761106d8a92519490611036565b503d61112f565b87513d6000823e3d90fd5b601183634e487b7160e01b6000525260246000fd5b8551632f3849bf60e01b81528490fd5b85516305ac732f60e41b81528490fd5b8551630789a70b60e41b81528490fd5b8451631cdfeead60e01b81528390fd5b602184634e487b7160e01b6000525260246000fd5b9093928115611396576001600160a01b03948516928315611384578015611372576001600160401b0380921642811115611361576000848152600460205260409384822090815460ff8160e01c16600581101561134d576112f45750505083519061123482610b36565b86825260208201838152858301600181526060840192868452608085019b818d528982526004602052888220955116908554936001600160401b0360a01b905160a01b1692519060058210156112e05750917f69651d5472102a8974405fe359e638ff78212233561a392eaa32adec520a57e4999a9b9c93916002959360ff60e01b9060e01b169262ffffff60e81b1617171784555160018401555191015582519182526020820152a3565b634e487b7160e01b81526021600452602490fd5b94939650949798909691508316149485159561133e575b5050831561132e575b50505061131e5750565b5163341d85cf60e11b8152600490fd5b60a01c1614159050388080611314565b6001015414159350388061130b565b634e487b7160e01b85526021600452602485fd5b60405162d36c8560e81b8152600490fd5b60405163162908e360e11b8152600490fd5b60405163d92e233d60e01b8152600490fd5b60405163e334302b60e01b8152600490fd5b3360009081527fee57cd81e84075558e8fcc182a1f4393f91fc97f963a136e66b7f949a62f319f60209081526040808320549092907f97667070c54ef182b0f5858b034beac1b6f3089aa2d3188bb1e8929f4fa9b9299060ff161561140d5750505050565b61141633611d0d565b84519161142283610b9e565b6042835284830193606036863783511561157e57603085538351906001916001101561157e5790607860218601536041915b818311611510575050506114ce576109789385936114b8936114a96048946109c49951988576020b1b1b2b9b9a1b7b73a3937b61d1030b1b1b7bab73a1604d1b8b978801528251928391603789019101610bda565b01036028810185520183610bb9565b5162461bcd60e51b815291829160048301610bfd565b60648486519062461bcd60e51b825280600483015260248201527f537472696e67733a20686578206c656e67746820696e73756666696369656e746044820152fd5b909192600f8116601081101561156a576f181899199a1a9b1b9c1cb0b131b232b360811b901a6115408588611ce6565b5360041c92801561155657600019019190611454565b634e487b7160e01b82526011600452602482fd5b634e487b7160e01b83526032600452602483fd5b634e487b7160e01b81526032600452602490fd5b3360009081527ff7c9542c591017a21c74b6f3fab6263c7952fc0aaf9db4c22a2a04ddc7f8674f60209081526040808320549092907f65d7a28e3265b37a6474929f336521b332c1681b933f6cb9f3376673440d862a9060ff16156115f75750505050565b61160033611d0d565b84519161160c83610b9e565b6042835284830193606036863783511561157e57603085538351906001916001101561157e5790607860218601536041915b818311611693575050506114ce576109789385936114b8936114a96048946109c49951988576020b1b1b2b9b9a1b7b73a3937b61d1030b1b1b7bab73a1604d1b8b978801528251928391603789019101610bda565b909192600f8116601081101561156a576f181899199a1a9b1b9c1cb0b131b232b360811b901a6116c38588611ce6565b5360041c9280156115565760001901919061163e565b6000818152602090808252604092838220338352835260ff8483205416156117015750505050565b61170a33611d0d565b84519161171683610b9e565b6042835284830193606036863783511561157e57603085538351906001916001101561157e5790607860218601536041915b81831161179d575050506114ce576109789385936114b8936114a96048946109c49951988576020b1b1b2b9b9a1b7b73a3937b61d1030b1b1b7bab73a1604d1b8b978801528251928391603789019101610bda565b909192600f8116601081101561156a576f181899199a1a9b1b9c1cb0b131b232b360811b901a6117cd8588611ce6565b5360041c92801561155657600019019190611748565b60028054146117f25760028055565b60405162461bcd60e51b815260206004820152601f60248201527f5265656e7472616e637947756172643a207265656e7472616e742063616c6c006044820152606490fd5b7f000000000000000000000000000000000000000000000000000000000000000060018060a01b039283821660409384516370a0823160e01b948582526004973089840152602092602491848284818a5afa9182156119f157600092611a3f575b508951958987521692838b870152848684818a5afa9586156119f157600096611a10575b5089519063a9059cbb60e01b86830152848483015288604483015260448252608082018281106001600160401b038211176119fc578b526118fd9190611b9f565b8851888152308b820152848184818a5afa9081156119f1579088916000916119be575b5061192b9192610c29565b1496871597611951575b50505050505050611944575050565b51631f75486b60e21b8152fd5b8394959697508851968793849283528b8301525afa9081156111165760009161198e575b506119809250610c29565b141538808080808080611935565b905082813d83116119b7575b6119a48183610bb9565b81010312610b0357611980915138611975565b503d61199a565b809250868092503d83116119ea575b6119d78183610bb9565b81010312610b035751879061192b611920565b503d6119cd565b8a513d6000823e3d90fd5b8460418e634e487b7160e01b600052526000fd5b9095508481813d8311611a38575b611a288183610bb9565b81010312610b03575194386118bc565b503d611a1e565b9091508481813d8311611a67575b611a578183610bb9565b81010312610b0357519038611898565b503d611a4d565b60ff60015416611a7a57565b60405162461bcd60e51b815260206004820152601060248201526f14185d5cd8589b194e881c185d5cd95960821b6044820152606490fd5b600052600460205260406000209060ff825460e01c166005811015610dad5715611ad857565b6040516303581dbd60e31b8152600490fd5b801561139657600052600560205260406000208054908115611b23575003611b1157600090565b60405163df9e71d560e01b8152600490fd5b905055600190565b9060009180835282602052604083209160018060a01b03169182845260205260ff604084205416611b5b57505050565b80835282602052604083208284526020526040832060ff1981541690557ff6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b339380a4565b60018060a01b031690604051611bb481610b67565b6020928382527f5361666545524332303a206c6f772d6c6576656c2063616c6c206661696c6564848301526000808486829651910182855af13d15611cd8573d916001600160401b038311611cc45790611c2e93929160405192611c2188601f19601f8401160185610bb9565b83523d868885013e611e1c565b805191821591848315611ca0575b505050905015611c495750565b6084906040519062461bcd60e51b82526004820152602a60248201527f5361666545524332303a204552433230206f7065726174696f6e20646964206e6044820152691bdd081cdd58d8d9595960b21b6064820152fd5b9193818094500103126101565782015190811515820361035b575080388084611c3c565b634e487b7160e01b85526041600452602485fd5b90611c2e9291606091611e1c565b908151811015611cf7570160200190565b634e487b7160e01b600052603260045260246000fd5b60405190606082018281106001600160401b03821117610b5157604052602a8252602082016040368237825115611cf75760309053815160019060011015611cf757607860218401536029905b808211611dae575050611d6a5790565b606460405162461bcd60e51b815260206004820152602060248201527f537472696e67733a20686578206c656e67746820696e73756666696369656e746044820152fd5b9091600f81166010811015611e07576f181899199a1a9b1b9c1cb0b131b232b360811b901a611ddd8486611ce6565b5360041c918015611df2576000190190611d5a565b60246000634e487b7160e01b81526011600452fd5b60246000634e487b7160e01b81526032600452fd5b91929015611e7e5750815115611e30575090565b3b15611e395790565b60405162461bcd60e51b815260206004820152601d60248201527f416464726573733a2063616c6c20746f206e6f6e2d636f6e74726163740000006044820152606490fd5b825190915015611e915750805190602001fd5b60405162461bcd60e51b81529081906109c49060048301610bfd56fea26469706673582212208d35512cebb5403b9e3ec78d3ac2f5074acf64f2b11c49c0229e22df5439089064736f6c634300081600332f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d",
}

// CollateralVaultABI is the input ABI used to generate the binding from.
// Deprecated: Use CollateralVaultMetaData.ABI instead.
var CollateralVaultABI = CollateralVaultMetaData.ABI

// CollateralVaultBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use CollateralVaultMetaData.Bin instead.
var CollateralVaultBin = CollateralVaultMetaData.Bin

// DeployCollateralVault deploys a new Ethereum contract, binding an instance of CollateralVault to it.
func DeployCollateralVault(auth *bind.TransactOpts, backend bind.ContractBackend, assetAddress common.Address, admin common.Address, operator common.Address) (common.Address, *types.Transaction, *CollateralVault, error) {
	parsed, err := CollateralVaultMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(CollateralVaultBin), backend, assetAddress, admin, operator)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &CollateralVault{CollateralVaultCaller: CollateralVaultCaller{contract: contract}, CollateralVaultTransactor: CollateralVaultTransactor{contract: contract}, CollateralVaultFilterer: CollateralVaultFilterer{contract: contract}}, nil
}

// CollateralVault is an auto generated Go binding around an Ethereum contract.
type CollateralVault struct {
	CollateralVaultCaller     // Read-only binding to the contract
	CollateralVaultTransactor // Write-only binding to the contract
	CollateralVaultFilterer   // Log filterer for contract events
}

// CollateralVaultCaller is an auto generated read-only Go binding around an Ethereum contract.
type CollateralVaultCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CollateralVaultTransactor is an auto generated write-only Go binding around an Ethereum contract.
type CollateralVaultTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CollateralVaultFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type CollateralVaultFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CollateralVaultSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type CollateralVaultSession struct {
	Contract     *CollateralVault  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// CollateralVaultCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type CollateralVaultCallerSession struct {
	Contract *CollateralVaultCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// CollateralVaultTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type CollateralVaultTransactorSession struct {
	Contract     *CollateralVaultTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// CollateralVaultRaw is an auto generated low-level Go binding around an Ethereum contract.
type CollateralVaultRaw struct {
	Contract *CollateralVault // Generic contract binding to access the raw methods on
}

// CollateralVaultCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type CollateralVaultCallerRaw struct {
	Contract *CollateralVaultCaller // Generic read-only contract binding to access the raw methods on
}

// CollateralVaultTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type CollateralVaultTransactorRaw struct {
	Contract *CollateralVaultTransactor // Generic write-only contract binding to access the raw methods on
}

// NewCollateralVault creates a new instance of CollateralVault, bound to a specific deployed contract.
func NewCollateralVault(address common.Address, backend bind.ContractBackend) (*CollateralVault, error) {
	contract, err := bindCollateralVault(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &CollateralVault{CollateralVaultCaller: CollateralVaultCaller{contract: contract}, CollateralVaultTransactor: CollateralVaultTransactor{contract: contract}, CollateralVaultFilterer: CollateralVaultFilterer{contract: contract}}, nil
}

// NewCollateralVaultCaller creates a new read-only instance of CollateralVault, bound to a specific deployed contract.
func NewCollateralVaultCaller(address common.Address, caller bind.ContractCaller) (*CollateralVaultCaller, error) {
	contract, err := bindCollateralVault(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &CollateralVaultCaller{contract: contract}, nil
}

// NewCollateralVaultTransactor creates a new write-only instance of CollateralVault, bound to a specific deployed contract.
func NewCollateralVaultTransactor(address common.Address, transactor bind.ContractTransactor) (*CollateralVaultTransactor, error) {
	contract, err := bindCollateralVault(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &CollateralVaultTransactor{contract: contract}, nil
}

// NewCollateralVaultFilterer creates a new log filterer instance of CollateralVault, bound to a specific deployed contract.
func NewCollateralVaultFilterer(address common.Address, filterer bind.ContractFilterer) (*CollateralVaultFilterer, error) {
	contract, err := bindCollateralVault(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &CollateralVaultFilterer{contract: contract}, nil
}

// bindCollateralVault binds a generic wrapper to an already deployed contract.
func bindCollateralVault(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := CollateralVaultMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CollateralVault *CollateralVaultRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CollateralVault.Contract.CollateralVaultCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CollateralVault *CollateralVaultRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CollateralVault.Contract.CollateralVaultTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CollateralVault *CollateralVaultRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CollateralVault.Contract.CollateralVaultTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CollateralVault *CollateralVaultCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CollateralVault.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CollateralVault *CollateralVaultTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CollateralVault.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CollateralVault *CollateralVaultTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CollateralVault.Contract.contract.Transact(opts, method, params...)
}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_CollateralVault *CollateralVaultCaller) DEFAULTADMINROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "DEFAULT_ADMIN_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_CollateralVault *CollateralVaultSession) DEFAULTADMINROLE() ([32]byte, error) {
	return _CollateralVault.Contract.DEFAULTADMINROLE(&_CollateralVault.CallOpts)
}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_CollateralVault *CollateralVaultCallerSession) DEFAULTADMINROLE() ([32]byte, error) {
	return _CollateralVault.Contract.DEFAULTADMINROLE(&_CollateralVault.CallOpts)
}

// OPERATORROLE is a free data retrieval call binding the contract method 0xf5b541a6.
//
// Solidity: function OPERATOR_ROLE() view returns(bytes32)
func (_CollateralVault *CollateralVaultCaller) OPERATORROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "OPERATOR_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// OPERATORROLE is a free data retrieval call binding the contract method 0xf5b541a6.
//
// Solidity: function OPERATOR_ROLE() view returns(bytes32)
func (_CollateralVault *CollateralVaultSession) OPERATORROLE() ([32]byte, error) {
	return _CollateralVault.Contract.OPERATORROLE(&_CollateralVault.CallOpts)
}

// OPERATORROLE is a free data retrieval call binding the contract method 0xf5b541a6.
//
// Solidity: function OPERATOR_ROLE() view returns(bytes32)
func (_CollateralVault *CollateralVaultCallerSession) OPERATORROLE() ([32]byte, error) {
	return _CollateralVault.Contract.OPERATORROLE(&_CollateralVault.CallOpts)
}

// PAUSERROLE is a free data retrieval call binding the contract method 0xe63ab1e9.
//
// Solidity: function PAUSER_ROLE() view returns(bytes32)
func (_CollateralVault *CollateralVaultCaller) PAUSERROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "PAUSER_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// PAUSERROLE is a free data retrieval call binding the contract method 0xe63ab1e9.
//
// Solidity: function PAUSER_ROLE() view returns(bytes32)
func (_CollateralVault *CollateralVaultSession) PAUSERROLE() ([32]byte, error) {
	return _CollateralVault.Contract.PAUSERROLE(&_CollateralVault.CallOpts)
}

// PAUSERROLE is a free data retrieval call binding the contract method 0xe63ab1e9.
//
// Solidity: function PAUSER_ROLE() view returns(bytes32)
func (_CollateralVault *CollateralVaultCallerSession) PAUSERROLE() ([32]byte, error) {
	return _CollateralVault.Contract.PAUSERROLE(&_CollateralVault.CallOpts)
}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() view returns(string)
func (_CollateralVault *CollateralVaultCaller) VERSION(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "VERSION")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() view returns(string)
func (_CollateralVault *CollateralVaultSession) VERSION() (string, error) {
	return _CollateralVault.Contract.VERSION(&_CollateralVault.CallOpts)
}

// VERSION is a free data retrieval call binding the contract method 0xffa1ad74.
//
// Solidity: function VERSION() view returns(string)
func (_CollateralVault *CollateralVaultCallerSession) VERSION() (string, error) {
	return _CollateralVault.Contract.VERSION(&_CollateralVault.CallOpts)
}

// ActionDigests is a free data retrieval call binding the contract method 0x9e9a8003.
//
// Solidity: function actionDigests(bytes32 ) view returns(bytes32)
func (_CollateralVault *CollateralVaultCaller) ActionDigests(opts *bind.CallOpts, arg0 [32]byte) ([32]byte, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "actionDigests", arg0)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// ActionDigests is a free data retrieval call binding the contract method 0x9e9a8003.
//
// Solidity: function actionDigests(bytes32 ) view returns(bytes32)
func (_CollateralVault *CollateralVaultSession) ActionDigests(arg0 [32]byte) ([32]byte, error) {
	return _CollateralVault.Contract.ActionDigests(&_CollateralVault.CallOpts, arg0)
}

// ActionDigests is a free data retrieval call binding the contract method 0x9e9a8003.
//
// Solidity: function actionDigests(bytes32 ) view returns(bytes32)
func (_CollateralVault *CollateralVaultCallerSession) ActionDigests(arg0 [32]byte) ([32]byte, error) {
	return _CollateralVault.Contract.ActionDigests(&_CollateralVault.CallOpts, arg0)
}

// Asset is a free data retrieval call binding the contract method 0x38d52e0f.
//
// Solidity: function asset() view returns(address)
func (_CollateralVault *CollateralVaultCaller) Asset(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "asset")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Asset is a free data retrieval call binding the contract method 0x38d52e0f.
//
// Solidity: function asset() view returns(address)
func (_CollateralVault *CollateralVaultSession) Asset() (common.Address, error) {
	return _CollateralVault.Contract.Asset(&_CollateralVault.CallOpts)
}

// Asset is a free data retrieval call binding the contract method 0x38d52e0f.
//
// Solidity: function asset() view returns(address)
func (_CollateralVault *CollateralVaultCallerSession) Asset() (common.Address, error) {
	return _CollateralVault.Contract.Asset(&_CollateralVault.CallOpts)
}

// GetObligation is a free data retrieval call binding the contract method 0x7f9e0fd2.
//
// Solidity: function getObligation(bytes32 collateralId) view returns((address,uint64,uint8,uint256,uint256))
func (_CollateralVault *CollateralVaultCaller) GetObligation(opts *bind.CallOpts, collateralId [32]byte) (CollateralVaultObligation, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "getObligation", collateralId)

	if err != nil {
		return *new(CollateralVaultObligation), err
	}

	out0 := *abi.ConvertType(out[0], new(CollateralVaultObligation)).(*CollateralVaultObligation)

	return out0, err

}

// GetObligation is a free data retrieval call binding the contract method 0x7f9e0fd2.
//
// Solidity: function getObligation(bytes32 collateralId) view returns((address,uint64,uint8,uint256,uint256))
func (_CollateralVault *CollateralVaultSession) GetObligation(collateralId [32]byte) (CollateralVaultObligation, error) {
	return _CollateralVault.Contract.GetObligation(&_CollateralVault.CallOpts, collateralId)
}

// GetObligation is a free data retrieval call binding the contract method 0x7f9e0fd2.
//
// Solidity: function getObligation(bytes32 collateralId) view returns((address,uint64,uint8,uint256,uint256))
func (_CollateralVault *CollateralVaultCallerSession) GetObligation(collateralId [32]byte) (CollateralVaultObligation, error) {
	return _CollateralVault.Contract.GetObligation(&_CollateralVault.CallOpts, collateralId)
}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_CollateralVault *CollateralVaultCaller) GetRoleAdmin(opts *bind.CallOpts, role [32]byte) ([32]byte, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "getRoleAdmin", role)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_CollateralVault *CollateralVaultSession) GetRoleAdmin(role [32]byte) ([32]byte, error) {
	return _CollateralVault.Contract.GetRoleAdmin(&_CollateralVault.CallOpts, role)
}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_CollateralVault *CollateralVaultCallerSession) GetRoleAdmin(role [32]byte) ([32]byte, error) {
	return _CollateralVault.Contract.GetRoleAdmin(&_CollateralVault.CallOpts, role)
}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_CollateralVault *CollateralVaultCaller) HasRole(opts *bind.CallOpts, role [32]byte, account common.Address) (bool, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "hasRole", role, account)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_CollateralVault *CollateralVaultSession) HasRole(role [32]byte, account common.Address) (bool, error) {
	return _CollateralVault.Contract.HasRole(&_CollateralVault.CallOpts, role, account)
}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_CollateralVault *CollateralVaultCallerSession) HasRole(role [32]byte, account common.Address) (bool, error) {
	return _CollateralVault.Contract.HasRole(&_CollateralVault.CallOpts, role, account)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_CollateralVault *CollateralVaultCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "paused")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_CollateralVault *CollateralVaultSession) Paused() (bool, error) {
	return _CollateralVault.Contract.Paused(&_CollateralVault.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_CollateralVault *CollateralVaultCallerSession) Paused() (bool, error) {
	return _CollateralVault.Contract.Paused(&_CollateralVault.CallOpts)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_CollateralVault *CollateralVaultCaller) SupportsInterface(opts *bind.CallOpts, interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "supportsInterface", interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_CollateralVault *CollateralVaultSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _CollateralVault.Contract.SupportsInterface(&_CollateralVault.CallOpts, interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_CollateralVault *CollateralVaultCallerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _CollateralVault.Contract.SupportsInterface(&_CollateralVault.CallOpts, interfaceId)
}

// TotalLocked is a free data retrieval call binding the contract method 0x56891412.
//
// Solidity: function totalLocked() view returns(uint256)
func (_CollateralVault *CollateralVaultCaller) TotalLocked(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _CollateralVault.contract.Call(opts, &out, "totalLocked")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalLocked is a free data retrieval call binding the contract method 0x56891412.
//
// Solidity: function totalLocked() view returns(uint256)
func (_CollateralVault *CollateralVaultSession) TotalLocked() (*big.Int, error) {
	return _CollateralVault.Contract.TotalLocked(&_CollateralVault.CallOpts)
}

// TotalLocked is a free data retrieval call binding the contract method 0x56891412.
//
// Solidity: function totalLocked() view returns(uint256)
func (_CollateralVault *CollateralVaultCallerSession) TotalLocked() (*big.Int, error) {
	return _CollateralVault.Contract.TotalLocked(&_CollateralVault.CallOpts)
}

// CreateObligation is a paid mutator transaction binding the contract method 0xc03aa3fb.
//
// Solidity: function createObligation(bytes32 collateralId, address principal, uint256 requiredAmount, uint64 expiresAt) returns()
func (_CollateralVault *CollateralVaultTransactor) CreateObligation(opts *bind.TransactOpts, collateralId [32]byte, principal common.Address, requiredAmount *big.Int, expiresAt uint64) (*types.Transaction, error) {
	return _CollateralVault.contract.Transact(opts, "createObligation", collateralId, principal, requiredAmount, expiresAt)
}

// CreateObligation is a paid mutator transaction binding the contract method 0xc03aa3fb.
//
// Solidity: function createObligation(bytes32 collateralId, address principal, uint256 requiredAmount, uint64 expiresAt) returns()
func (_CollateralVault *CollateralVaultSession) CreateObligation(collateralId [32]byte, principal common.Address, requiredAmount *big.Int, expiresAt uint64) (*types.Transaction, error) {
	return _CollateralVault.Contract.CreateObligation(&_CollateralVault.TransactOpts, collateralId, principal, requiredAmount, expiresAt)
}

// CreateObligation is a paid mutator transaction binding the contract method 0xc03aa3fb.
//
// Solidity: function createObligation(bytes32 collateralId, address principal, uint256 requiredAmount, uint64 expiresAt) returns()
func (_CollateralVault *CollateralVaultTransactorSession) CreateObligation(collateralId [32]byte, principal common.Address, requiredAmount *big.Int, expiresAt uint64) (*types.Transaction, error) {
	return _CollateralVault.Contract.CreateObligation(&_CollateralVault.TransactOpts, collateralId, principal, requiredAmount, expiresAt)
}

// Fund is a paid mutator transaction binding the contract method 0xa082271b.
//
// Solidity: function fund(bytes32 collateralId, bytes32 fundingId, uint256 amount) returns()
func (_CollateralVault *CollateralVaultTransactor) Fund(opts *bind.TransactOpts, collateralId [32]byte, fundingId [32]byte, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.contract.Transact(opts, "fund", collateralId, fundingId, amount)
}

// Fund is a paid mutator transaction binding the contract method 0xa082271b.
//
// Solidity: function fund(bytes32 collateralId, bytes32 fundingId, uint256 amount) returns()
func (_CollateralVault *CollateralVaultSession) Fund(collateralId [32]byte, fundingId [32]byte, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.Contract.Fund(&_CollateralVault.TransactOpts, collateralId, fundingId, amount)
}

// Fund is a paid mutator transaction binding the contract method 0xa082271b.
//
// Solidity: function fund(bytes32 collateralId, bytes32 fundingId, uint256 amount) returns()
func (_CollateralVault *CollateralVaultTransactorSession) Fund(collateralId [32]byte, fundingId [32]byte, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.Contract.Fund(&_CollateralVault.TransactOpts, collateralId, fundingId, amount)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_CollateralVault *CollateralVaultTransactor) GrantRole(opts *bind.TransactOpts, role [32]byte, account common.Address) (*types.Transaction, error) {
	return _CollateralVault.contract.Transact(opts, "grantRole", role, account)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_CollateralVault *CollateralVaultSession) GrantRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _CollateralVault.Contract.GrantRole(&_CollateralVault.TransactOpts, role, account)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_CollateralVault *CollateralVaultTransactorSession) GrantRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _CollateralVault.Contract.GrantRole(&_CollateralVault.TransactOpts, role, account)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_CollateralVault *CollateralVaultTransactor) Pause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CollateralVault.contract.Transact(opts, "pause")
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_CollateralVault *CollateralVaultSession) Pause() (*types.Transaction, error) {
	return _CollateralVault.Contract.Pause(&_CollateralVault.TransactOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_CollateralVault *CollateralVaultTransactorSession) Pause() (*types.Transaction, error) {
	return _CollateralVault.Contract.Pause(&_CollateralVault.TransactOpts)
}

// RecoverSurplus is a paid mutator transaction binding the contract method 0x0e57c69a.
//
// Solidity: function recoverSurplus(address receiver, uint256 amount) returns()
func (_CollateralVault *CollateralVaultTransactor) RecoverSurplus(opts *bind.TransactOpts, receiver common.Address, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.contract.Transact(opts, "recoverSurplus", receiver, amount)
}

// RecoverSurplus is a paid mutator transaction binding the contract method 0x0e57c69a.
//
// Solidity: function recoverSurplus(address receiver, uint256 amount) returns()
func (_CollateralVault *CollateralVaultSession) RecoverSurplus(receiver common.Address, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.Contract.RecoverSurplus(&_CollateralVault.TransactOpts, receiver, amount)
}

// RecoverSurplus is a paid mutator transaction binding the contract method 0x0e57c69a.
//
// Solidity: function recoverSurplus(address receiver, uint256 amount) returns()
func (_CollateralVault *CollateralVaultTransactorSession) RecoverSurplus(receiver common.Address, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.Contract.RecoverSurplus(&_CollateralVault.TransactOpts, receiver, amount)
}

// Release is a paid mutator transaction binding the contract method 0x85a9549a.
//
// Solidity: function release(bytes32 collateralId, bytes32 actionId, address destination, uint256 amount) returns()
func (_CollateralVault *CollateralVaultTransactor) Release(opts *bind.TransactOpts, collateralId [32]byte, actionId [32]byte, destination common.Address, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.contract.Transact(opts, "release", collateralId, actionId, destination, amount)
}

// Release is a paid mutator transaction binding the contract method 0x85a9549a.
//
// Solidity: function release(bytes32 collateralId, bytes32 actionId, address destination, uint256 amount) returns()
func (_CollateralVault *CollateralVaultSession) Release(collateralId [32]byte, actionId [32]byte, destination common.Address, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.Contract.Release(&_CollateralVault.TransactOpts, collateralId, actionId, destination, amount)
}

// Release is a paid mutator transaction binding the contract method 0x85a9549a.
//
// Solidity: function release(bytes32 collateralId, bytes32 actionId, address destination, uint256 amount) returns()
func (_CollateralVault *CollateralVaultTransactorSession) Release(collateralId [32]byte, actionId [32]byte, destination common.Address, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.Contract.Release(&_CollateralVault.TransactOpts, collateralId, actionId, destination, amount)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address account) returns()
func (_CollateralVault *CollateralVaultTransactor) RenounceRole(opts *bind.TransactOpts, role [32]byte, account common.Address) (*types.Transaction, error) {
	return _CollateralVault.contract.Transact(opts, "renounceRole", role, account)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address account) returns()
func (_CollateralVault *CollateralVaultSession) RenounceRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _CollateralVault.Contract.RenounceRole(&_CollateralVault.TransactOpts, role, account)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address account) returns()
func (_CollateralVault *CollateralVaultTransactorSession) RenounceRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _CollateralVault.Contract.RenounceRole(&_CollateralVault.TransactOpts, role, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_CollateralVault *CollateralVaultTransactor) RevokeRole(opts *bind.TransactOpts, role [32]byte, account common.Address) (*types.Transaction, error) {
	return _CollateralVault.contract.Transact(opts, "revokeRole", role, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_CollateralVault *CollateralVaultSession) RevokeRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _CollateralVault.Contract.RevokeRole(&_CollateralVault.TransactOpts, role, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_CollateralVault *CollateralVaultTransactorSession) RevokeRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _CollateralVault.Contract.RevokeRole(&_CollateralVault.TransactOpts, role, account)
}

// Slash is a paid mutator transaction binding the contract method 0x30c09002.
//
// Solidity: function slash(bytes32 collateralId, bytes32 actionId, address beneficiary, uint256 amount) returns()
func (_CollateralVault *CollateralVaultTransactor) Slash(opts *bind.TransactOpts, collateralId [32]byte, actionId [32]byte, beneficiary common.Address, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.contract.Transact(opts, "slash", collateralId, actionId, beneficiary, amount)
}

// Slash is a paid mutator transaction binding the contract method 0x30c09002.
//
// Solidity: function slash(bytes32 collateralId, bytes32 actionId, address beneficiary, uint256 amount) returns()
func (_CollateralVault *CollateralVaultSession) Slash(collateralId [32]byte, actionId [32]byte, beneficiary common.Address, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.Contract.Slash(&_CollateralVault.TransactOpts, collateralId, actionId, beneficiary, amount)
}

// Slash is a paid mutator transaction binding the contract method 0x30c09002.
//
// Solidity: function slash(bytes32 collateralId, bytes32 actionId, address beneficiary, uint256 amount) returns()
func (_CollateralVault *CollateralVaultTransactorSession) Slash(collateralId [32]byte, actionId [32]byte, beneficiary common.Address, amount *big.Int) (*types.Transaction, error) {
	return _CollateralVault.Contract.Slash(&_CollateralVault.TransactOpts, collateralId, actionId, beneficiary, amount)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_CollateralVault *CollateralVaultTransactor) Unpause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CollateralVault.contract.Transact(opts, "unpause")
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_CollateralVault *CollateralVaultSession) Unpause() (*types.Transaction, error) {
	return _CollateralVault.Contract.Unpause(&_CollateralVault.TransactOpts)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_CollateralVault *CollateralVaultTransactorSession) Unpause() (*types.Transaction, error) {
	return _CollateralVault.Contract.Unpause(&_CollateralVault.TransactOpts)
}

// CollateralVaultCollateralFundedIterator is returned from FilterCollateralFunded and is used to iterate over the raw logs and unpacked data for CollateralFunded events raised by the CollateralVault contract.
type CollateralVaultCollateralFundedIterator struct {
	Event *CollateralVaultCollateralFunded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CollateralVaultCollateralFundedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CollateralVaultCollateralFunded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CollateralVaultCollateralFunded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CollateralVaultCollateralFundedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CollateralVaultCollateralFundedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CollateralVaultCollateralFunded represents a CollateralFunded event raised by the CollateralVault contract.
type CollateralVaultCollateralFunded struct {
	CollateralId [32]byte
	FundingId    [32]byte
	Principal    common.Address
	Amount       *big.Int
	TotalBalance *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterCollateralFunded is a free log retrieval operation binding the contract event 0x5be65e993dfcc065578c63c356d3f95047265a5663ce5aa8cd611e73f3690349.
//
// Solidity: event CollateralFunded(bytes32 indexed collateralId, bytes32 indexed fundingId, address indexed principal, uint256 amount, uint256 totalBalance)
func (_CollateralVault *CollateralVaultFilterer) FilterCollateralFunded(opts *bind.FilterOpts, collateralId [][32]byte, fundingId [][32]byte, principal []common.Address) (*CollateralVaultCollateralFundedIterator, error) {

	var collateralIdRule []interface{}
	for _, collateralIdItem := range collateralId {
		collateralIdRule = append(collateralIdRule, collateralIdItem)
	}
	var fundingIdRule []interface{}
	for _, fundingIdItem := range fundingId {
		fundingIdRule = append(fundingIdRule, fundingIdItem)
	}
	var principalRule []interface{}
	for _, principalItem := range principal {
		principalRule = append(principalRule, principalItem)
	}

	logs, sub, err := _CollateralVault.contract.FilterLogs(opts, "CollateralFunded", collateralIdRule, fundingIdRule, principalRule)
	if err != nil {
		return nil, err
	}
	return &CollateralVaultCollateralFundedIterator{contract: _CollateralVault.contract, event: "CollateralFunded", logs: logs, sub: sub}, nil
}

// WatchCollateralFunded is a free log subscription operation binding the contract event 0x5be65e993dfcc065578c63c356d3f95047265a5663ce5aa8cd611e73f3690349.
//
// Solidity: event CollateralFunded(bytes32 indexed collateralId, bytes32 indexed fundingId, address indexed principal, uint256 amount, uint256 totalBalance)
func (_CollateralVault *CollateralVaultFilterer) WatchCollateralFunded(opts *bind.WatchOpts, sink chan<- *CollateralVaultCollateralFunded, collateralId [][32]byte, fundingId [][32]byte, principal []common.Address) (event.Subscription, error) {

	var collateralIdRule []interface{}
	for _, collateralIdItem := range collateralId {
		collateralIdRule = append(collateralIdRule, collateralIdItem)
	}
	var fundingIdRule []interface{}
	for _, fundingIdItem := range fundingId {
		fundingIdRule = append(fundingIdRule, fundingIdItem)
	}
	var principalRule []interface{}
	for _, principalItem := range principal {
		principalRule = append(principalRule, principalItem)
	}

	logs, sub, err := _CollateralVault.contract.WatchLogs(opts, "CollateralFunded", collateralIdRule, fundingIdRule, principalRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CollateralVaultCollateralFunded)
				if err := _CollateralVault.contract.UnpackLog(event, "CollateralFunded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCollateralFunded is a log parse operation binding the contract event 0x5be65e993dfcc065578c63c356d3f95047265a5663ce5aa8cd611e73f3690349.
//
// Solidity: event CollateralFunded(bytes32 indexed collateralId, bytes32 indexed fundingId, address indexed principal, uint256 amount, uint256 totalBalance)
func (_CollateralVault *CollateralVaultFilterer) ParseCollateralFunded(log types.Log) (*CollateralVaultCollateralFunded, error) {
	event := new(CollateralVaultCollateralFunded)
	if err := _CollateralVault.contract.UnpackLog(event, "CollateralFunded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CollateralVaultCollateralReleasedIterator is returned from FilterCollateralReleased and is used to iterate over the raw logs and unpacked data for CollateralReleased events raised by the CollateralVault contract.
type CollateralVaultCollateralReleasedIterator struct {
	Event *CollateralVaultCollateralReleased // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CollateralVaultCollateralReleasedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CollateralVaultCollateralReleased)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CollateralVaultCollateralReleased)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CollateralVaultCollateralReleasedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CollateralVaultCollateralReleasedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CollateralVaultCollateralReleased represents a CollateralReleased event raised by the CollateralVault contract.
type CollateralVaultCollateralReleased struct {
	CollateralId [32]byte
	ActionId     [32]byte
	Principal    common.Address
	Amount       *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterCollateralReleased is a free log retrieval operation binding the contract event 0x258c7762affe3584f997aa5ee6c2e5fccfa6073f1461dfc6382b0d28e286455c.
//
// Solidity: event CollateralReleased(bytes32 indexed collateralId, bytes32 indexed actionId, address indexed principal, uint256 amount)
func (_CollateralVault *CollateralVaultFilterer) FilterCollateralReleased(opts *bind.FilterOpts, collateralId [][32]byte, actionId [][32]byte, principal []common.Address) (*CollateralVaultCollateralReleasedIterator, error) {

	var collateralIdRule []interface{}
	for _, collateralIdItem := range collateralId {
		collateralIdRule = append(collateralIdRule, collateralIdItem)
	}
	var actionIdRule []interface{}
	for _, actionIdItem := range actionId {
		actionIdRule = append(actionIdRule, actionIdItem)
	}
	var principalRule []interface{}
	for _, principalItem := range principal {
		principalRule = append(principalRule, principalItem)
	}

	logs, sub, err := _CollateralVault.contract.FilterLogs(opts, "CollateralReleased", collateralIdRule, actionIdRule, principalRule)
	if err != nil {
		return nil, err
	}
	return &CollateralVaultCollateralReleasedIterator{contract: _CollateralVault.contract, event: "CollateralReleased", logs: logs, sub: sub}, nil
}

// WatchCollateralReleased is a free log subscription operation binding the contract event 0x258c7762affe3584f997aa5ee6c2e5fccfa6073f1461dfc6382b0d28e286455c.
//
// Solidity: event CollateralReleased(bytes32 indexed collateralId, bytes32 indexed actionId, address indexed principal, uint256 amount)
func (_CollateralVault *CollateralVaultFilterer) WatchCollateralReleased(opts *bind.WatchOpts, sink chan<- *CollateralVaultCollateralReleased, collateralId [][32]byte, actionId [][32]byte, principal []common.Address) (event.Subscription, error) {

	var collateralIdRule []interface{}
	for _, collateralIdItem := range collateralId {
		collateralIdRule = append(collateralIdRule, collateralIdItem)
	}
	var actionIdRule []interface{}
	for _, actionIdItem := range actionId {
		actionIdRule = append(actionIdRule, actionIdItem)
	}
	var principalRule []interface{}
	for _, principalItem := range principal {
		principalRule = append(principalRule, principalItem)
	}

	logs, sub, err := _CollateralVault.contract.WatchLogs(opts, "CollateralReleased", collateralIdRule, actionIdRule, principalRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CollateralVaultCollateralReleased)
				if err := _CollateralVault.contract.UnpackLog(event, "CollateralReleased", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCollateralReleased is a log parse operation binding the contract event 0x258c7762affe3584f997aa5ee6c2e5fccfa6073f1461dfc6382b0d28e286455c.
//
// Solidity: event CollateralReleased(bytes32 indexed collateralId, bytes32 indexed actionId, address indexed principal, uint256 amount)
func (_CollateralVault *CollateralVaultFilterer) ParseCollateralReleased(log types.Log) (*CollateralVaultCollateralReleased, error) {
	event := new(CollateralVaultCollateralReleased)
	if err := _CollateralVault.contract.UnpackLog(event, "CollateralReleased", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CollateralVaultCollateralSlashedIterator is returned from FilterCollateralSlashed and is used to iterate over the raw logs and unpacked data for CollateralSlashed events raised by the CollateralVault contract.
type CollateralVaultCollateralSlashedIterator struct {
	Event *CollateralVaultCollateralSlashed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CollateralVaultCollateralSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CollateralVaultCollateralSlashed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CollateralVaultCollateralSlashed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CollateralVaultCollateralSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CollateralVaultCollateralSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CollateralVaultCollateralSlashed represents a CollateralSlashed event raised by the CollateralVault contract.
type CollateralVaultCollateralSlashed struct {
	CollateralId     [32]byte
	ActionId         [32]byte
	Beneficiary      common.Address
	Amount           *big.Int
	RemainingBalance *big.Int
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterCollateralSlashed is a free log retrieval operation binding the contract event 0xb0e4fd6efcc1188105cedb091325e1c42948d7782c4942a36120368ac33827f7.
//
// Solidity: event CollateralSlashed(bytes32 indexed collateralId, bytes32 indexed actionId, address indexed beneficiary, uint256 amount, uint256 remainingBalance)
func (_CollateralVault *CollateralVaultFilterer) FilterCollateralSlashed(opts *bind.FilterOpts, collateralId [][32]byte, actionId [][32]byte, beneficiary []common.Address) (*CollateralVaultCollateralSlashedIterator, error) {

	var collateralIdRule []interface{}
	for _, collateralIdItem := range collateralId {
		collateralIdRule = append(collateralIdRule, collateralIdItem)
	}
	var actionIdRule []interface{}
	for _, actionIdItem := range actionId {
		actionIdRule = append(actionIdRule, actionIdItem)
	}
	var beneficiaryRule []interface{}
	for _, beneficiaryItem := range beneficiary {
		beneficiaryRule = append(beneficiaryRule, beneficiaryItem)
	}

	logs, sub, err := _CollateralVault.contract.FilterLogs(opts, "CollateralSlashed", collateralIdRule, actionIdRule, beneficiaryRule)
	if err != nil {
		return nil, err
	}
	return &CollateralVaultCollateralSlashedIterator{contract: _CollateralVault.contract, event: "CollateralSlashed", logs: logs, sub: sub}, nil
}

// WatchCollateralSlashed is a free log subscription operation binding the contract event 0xb0e4fd6efcc1188105cedb091325e1c42948d7782c4942a36120368ac33827f7.
//
// Solidity: event CollateralSlashed(bytes32 indexed collateralId, bytes32 indexed actionId, address indexed beneficiary, uint256 amount, uint256 remainingBalance)
func (_CollateralVault *CollateralVaultFilterer) WatchCollateralSlashed(opts *bind.WatchOpts, sink chan<- *CollateralVaultCollateralSlashed, collateralId [][32]byte, actionId [][32]byte, beneficiary []common.Address) (event.Subscription, error) {

	var collateralIdRule []interface{}
	for _, collateralIdItem := range collateralId {
		collateralIdRule = append(collateralIdRule, collateralIdItem)
	}
	var actionIdRule []interface{}
	for _, actionIdItem := range actionId {
		actionIdRule = append(actionIdRule, actionIdItem)
	}
	var beneficiaryRule []interface{}
	for _, beneficiaryItem := range beneficiary {
		beneficiaryRule = append(beneficiaryRule, beneficiaryItem)
	}

	logs, sub, err := _CollateralVault.contract.WatchLogs(opts, "CollateralSlashed", collateralIdRule, actionIdRule, beneficiaryRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CollateralVaultCollateralSlashed)
				if err := _CollateralVault.contract.UnpackLog(event, "CollateralSlashed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCollateralSlashed is a log parse operation binding the contract event 0xb0e4fd6efcc1188105cedb091325e1c42948d7782c4942a36120368ac33827f7.
//
// Solidity: event CollateralSlashed(bytes32 indexed collateralId, bytes32 indexed actionId, address indexed beneficiary, uint256 amount, uint256 remainingBalance)
func (_CollateralVault *CollateralVaultFilterer) ParseCollateralSlashed(log types.Log) (*CollateralVaultCollateralSlashed, error) {
	event := new(CollateralVaultCollateralSlashed)
	if err := _CollateralVault.contract.UnpackLog(event, "CollateralSlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CollateralVaultObligationCreatedIterator is returned from FilterObligationCreated and is used to iterate over the raw logs and unpacked data for ObligationCreated events raised by the CollateralVault contract.
type CollateralVaultObligationCreatedIterator struct {
	Event *CollateralVaultObligationCreated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CollateralVaultObligationCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CollateralVaultObligationCreated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CollateralVaultObligationCreated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CollateralVaultObligationCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CollateralVaultObligationCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CollateralVaultObligationCreated represents a ObligationCreated event raised by the CollateralVault contract.
type CollateralVaultObligationCreated struct {
	CollateralId   [32]byte
	Principal      common.Address
	RequiredAmount *big.Int
	ExpiresAt      uint64
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterObligationCreated is a free log retrieval operation binding the contract event 0x69651d5472102a8974405fe359e638ff78212233561a392eaa32adec520a57e4.
//
// Solidity: event ObligationCreated(bytes32 indexed collateralId, address indexed principal, uint256 requiredAmount, uint64 expiresAt)
func (_CollateralVault *CollateralVaultFilterer) FilterObligationCreated(opts *bind.FilterOpts, collateralId [][32]byte, principal []common.Address) (*CollateralVaultObligationCreatedIterator, error) {

	var collateralIdRule []interface{}
	for _, collateralIdItem := range collateralId {
		collateralIdRule = append(collateralIdRule, collateralIdItem)
	}
	var principalRule []interface{}
	for _, principalItem := range principal {
		principalRule = append(principalRule, principalItem)
	}

	logs, sub, err := _CollateralVault.contract.FilterLogs(opts, "ObligationCreated", collateralIdRule, principalRule)
	if err != nil {
		return nil, err
	}
	return &CollateralVaultObligationCreatedIterator{contract: _CollateralVault.contract, event: "ObligationCreated", logs: logs, sub: sub}, nil
}

// WatchObligationCreated is a free log subscription operation binding the contract event 0x69651d5472102a8974405fe359e638ff78212233561a392eaa32adec520a57e4.
//
// Solidity: event ObligationCreated(bytes32 indexed collateralId, address indexed principal, uint256 requiredAmount, uint64 expiresAt)
func (_CollateralVault *CollateralVaultFilterer) WatchObligationCreated(opts *bind.WatchOpts, sink chan<- *CollateralVaultObligationCreated, collateralId [][32]byte, principal []common.Address) (event.Subscription, error) {

	var collateralIdRule []interface{}
	for _, collateralIdItem := range collateralId {
		collateralIdRule = append(collateralIdRule, collateralIdItem)
	}
	var principalRule []interface{}
	for _, principalItem := range principal {
		principalRule = append(principalRule, principalItem)
	}

	logs, sub, err := _CollateralVault.contract.WatchLogs(opts, "ObligationCreated", collateralIdRule, principalRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CollateralVaultObligationCreated)
				if err := _CollateralVault.contract.UnpackLog(event, "ObligationCreated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseObligationCreated is a log parse operation binding the contract event 0x69651d5472102a8974405fe359e638ff78212233561a392eaa32adec520a57e4.
//
// Solidity: event ObligationCreated(bytes32 indexed collateralId, address indexed principal, uint256 requiredAmount, uint64 expiresAt)
func (_CollateralVault *CollateralVaultFilterer) ParseObligationCreated(log types.Log) (*CollateralVaultObligationCreated, error) {
	event := new(CollateralVaultObligationCreated)
	if err := _CollateralVault.contract.UnpackLog(event, "ObligationCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CollateralVaultPausedIterator is returned from FilterPaused and is used to iterate over the raw logs and unpacked data for Paused events raised by the CollateralVault contract.
type CollateralVaultPausedIterator struct {
	Event *CollateralVaultPaused // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CollateralVaultPausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CollateralVaultPaused)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CollateralVaultPaused)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CollateralVaultPausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CollateralVaultPausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CollateralVaultPaused represents a Paused event raised by the CollateralVault contract.
type CollateralVaultPaused struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterPaused is a free log retrieval operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_CollateralVault *CollateralVaultFilterer) FilterPaused(opts *bind.FilterOpts) (*CollateralVaultPausedIterator, error) {

	logs, sub, err := _CollateralVault.contract.FilterLogs(opts, "Paused")
	if err != nil {
		return nil, err
	}
	return &CollateralVaultPausedIterator{contract: _CollateralVault.contract, event: "Paused", logs: logs, sub: sub}, nil
}

// WatchPaused is a free log subscription operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_CollateralVault *CollateralVaultFilterer) WatchPaused(opts *bind.WatchOpts, sink chan<- *CollateralVaultPaused) (event.Subscription, error) {

	logs, sub, err := _CollateralVault.contract.WatchLogs(opts, "Paused")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CollateralVaultPaused)
				if err := _CollateralVault.contract.UnpackLog(event, "Paused", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePaused is a log parse operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_CollateralVault *CollateralVaultFilterer) ParsePaused(log types.Log) (*CollateralVaultPaused, error) {
	event := new(CollateralVaultPaused)
	if err := _CollateralVault.contract.UnpackLog(event, "Paused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CollateralVaultRoleAdminChangedIterator is returned from FilterRoleAdminChanged and is used to iterate over the raw logs and unpacked data for RoleAdminChanged events raised by the CollateralVault contract.
type CollateralVaultRoleAdminChangedIterator struct {
	Event *CollateralVaultRoleAdminChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CollateralVaultRoleAdminChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CollateralVaultRoleAdminChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CollateralVaultRoleAdminChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CollateralVaultRoleAdminChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CollateralVaultRoleAdminChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CollateralVaultRoleAdminChanged represents a RoleAdminChanged event raised by the CollateralVault contract.
type CollateralVaultRoleAdminChanged struct {
	Role              [32]byte
	PreviousAdminRole [32]byte
	NewAdminRole      [32]byte
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterRoleAdminChanged is a free log retrieval operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_CollateralVault *CollateralVaultFilterer) FilterRoleAdminChanged(opts *bind.FilterOpts, role [][32]byte, previousAdminRole [][32]byte, newAdminRole [][32]byte) (*CollateralVaultRoleAdminChangedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var previousAdminRoleRule []interface{}
	for _, previousAdminRoleItem := range previousAdminRole {
		previousAdminRoleRule = append(previousAdminRoleRule, previousAdminRoleItem)
	}
	var newAdminRoleRule []interface{}
	for _, newAdminRoleItem := range newAdminRole {
		newAdminRoleRule = append(newAdminRoleRule, newAdminRoleItem)
	}

	logs, sub, err := _CollateralVault.contract.FilterLogs(opts, "RoleAdminChanged", roleRule, previousAdminRoleRule, newAdminRoleRule)
	if err != nil {
		return nil, err
	}
	return &CollateralVaultRoleAdminChangedIterator{contract: _CollateralVault.contract, event: "RoleAdminChanged", logs: logs, sub: sub}, nil
}

// WatchRoleAdminChanged is a free log subscription operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_CollateralVault *CollateralVaultFilterer) WatchRoleAdminChanged(opts *bind.WatchOpts, sink chan<- *CollateralVaultRoleAdminChanged, role [][32]byte, previousAdminRole [][32]byte, newAdminRole [][32]byte) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var previousAdminRoleRule []interface{}
	for _, previousAdminRoleItem := range previousAdminRole {
		previousAdminRoleRule = append(previousAdminRoleRule, previousAdminRoleItem)
	}
	var newAdminRoleRule []interface{}
	for _, newAdminRoleItem := range newAdminRole {
		newAdminRoleRule = append(newAdminRoleRule, newAdminRoleItem)
	}

	logs, sub, err := _CollateralVault.contract.WatchLogs(opts, "RoleAdminChanged", roleRule, previousAdminRoleRule, newAdminRoleRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CollateralVaultRoleAdminChanged)
				if err := _CollateralVault.contract.UnpackLog(event, "RoleAdminChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRoleAdminChanged is a log parse operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_CollateralVault *CollateralVaultFilterer) ParseRoleAdminChanged(log types.Log) (*CollateralVaultRoleAdminChanged, error) {
	event := new(CollateralVaultRoleAdminChanged)
	if err := _CollateralVault.contract.UnpackLog(event, "RoleAdminChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CollateralVaultRoleGrantedIterator is returned from FilterRoleGranted and is used to iterate over the raw logs and unpacked data for RoleGranted events raised by the CollateralVault contract.
type CollateralVaultRoleGrantedIterator struct {
	Event *CollateralVaultRoleGranted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CollateralVaultRoleGrantedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CollateralVaultRoleGranted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CollateralVaultRoleGranted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CollateralVaultRoleGrantedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CollateralVaultRoleGrantedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CollateralVaultRoleGranted represents a RoleGranted event raised by the CollateralVault contract.
type CollateralVaultRoleGranted struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRoleGranted is a free log retrieval operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_CollateralVault *CollateralVaultFilterer) FilterRoleGranted(opts *bind.FilterOpts, role [][32]byte, account []common.Address, sender []common.Address) (*CollateralVaultRoleGrantedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _CollateralVault.contract.FilterLogs(opts, "RoleGranted", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &CollateralVaultRoleGrantedIterator{contract: _CollateralVault.contract, event: "RoleGranted", logs: logs, sub: sub}, nil
}

// WatchRoleGranted is a free log subscription operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_CollateralVault *CollateralVaultFilterer) WatchRoleGranted(opts *bind.WatchOpts, sink chan<- *CollateralVaultRoleGranted, role [][32]byte, account []common.Address, sender []common.Address) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _CollateralVault.contract.WatchLogs(opts, "RoleGranted", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CollateralVaultRoleGranted)
				if err := _CollateralVault.contract.UnpackLog(event, "RoleGranted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRoleGranted is a log parse operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_CollateralVault *CollateralVaultFilterer) ParseRoleGranted(log types.Log) (*CollateralVaultRoleGranted, error) {
	event := new(CollateralVaultRoleGranted)
	if err := _CollateralVault.contract.UnpackLog(event, "RoleGranted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CollateralVaultRoleRevokedIterator is returned from FilterRoleRevoked and is used to iterate over the raw logs and unpacked data for RoleRevoked events raised by the CollateralVault contract.
type CollateralVaultRoleRevokedIterator struct {
	Event *CollateralVaultRoleRevoked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CollateralVaultRoleRevokedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CollateralVaultRoleRevoked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CollateralVaultRoleRevoked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CollateralVaultRoleRevokedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CollateralVaultRoleRevokedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CollateralVaultRoleRevoked represents a RoleRevoked event raised by the CollateralVault contract.
type CollateralVaultRoleRevoked struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRoleRevoked is a free log retrieval operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_CollateralVault *CollateralVaultFilterer) FilterRoleRevoked(opts *bind.FilterOpts, role [][32]byte, account []common.Address, sender []common.Address) (*CollateralVaultRoleRevokedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _CollateralVault.contract.FilterLogs(opts, "RoleRevoked", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &CollateralVaultRoleRevokedIterator{contract: _CollateralVault.contract, event: "RoleRevoked", logs: logs, sub: sub}, nil
}

// WatchRoleRevoked is a free log subscription operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_CollateralVault *CollateralVaultFilterer) WatchRoleRevoked(opts *bind.WatchOpts, sink chan<- *CollateralVaultRoleRevoked, role [][32]byte, account []common.Address, sender []common.Address) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _CollateralVault.contract.WatchLogs(opts, "RoleRevoked", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CollateralVaultRoleRevoked)
				if err := _CollateralVault.contract.UnpackLog(event, "RoleRevoked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRoleRevoked is a log parse operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_CollateralVault *CollateralVaultFilterer) ParseRoleRevoked(log types.Log) (*CollateralVaultRoleRevoked, error) {
	event := new(CollateralVaultRoleRevoked)
	if err := _CollateralVault.contract.UnpackLog(event, "RoleRevoked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CollateralVaultSurplusRecoveredIterator is returned from FilterSurplusRecovered and is used to iterate over the raw logs and unpacked data for SurplusRecovered events raised by the CollateralVault contract.
type CollateralVaultSurplusRecoveredIterator struct {
	Event *CollateralVaultSurplusRecovered // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CollateralVaultSurplusRecoveredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CollateralVaultSurplusRecovered)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CollateralVaultSurplusRecovered)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CollateralVaultSurplusRecoveredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CollateralVaultSurplusRecoveredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CollateralVaultSurplusRecovered represents a SurplusRecovered event raised by the CollateralVault contract.
type CollateralVaultSurplusRecovered struct {
	Receiver common.Address
	Amount   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterSurplusRecovered is a free log retrieval operation binding the contract event 0x8d062d570e15e665f57873e5745c84e024c3b6a4c94cfcda6af54a9ccd416af7.
//
// Solidity: event SurplusRecovered(address indexed receiver, uint256 amount)
func (_CollateralVault *CollateralVaultFilterer) FilterSurplusRecovered(opts *bind.FilterOpts, receiver []common.Address) (*CollateralVaultSurplusRecoveredIterator, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}

	logs, sub, err := _CollateralVault.contract.FilterLogs(opts, "SurplusRecovered", receiverRule)
	if err != nil {
		return nil, err
	}
	return &CollateralVaultSurplusRecoveredIterator{contract: _CollateralVault.contract, event: "SurplusRecovered", logs: logs, sub: sub}, nil
}

// WatchSurplusRecovered is a free log subscription operation binding the contract event 0x8d062d570e15e665f57873e5745c84e024c3b6a4c94cfcda6af54a9ccd416af7.
//
// Solidity: event SurplusRecovered(address indexed receiver, uint256 amount)
func (_CollateralVault *CollateralVaultFilterer) WatchSurplusRecovered(opts *bind.WatchOpts, sink chan<- *CollateralVaultSurplusRecovered, receiver []common.Address) (event.Subscription, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}

	logs, sub, err := _CollateralVault.contract.WatchLogs(opts, "SurplusRecovered", receiverRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CollateralVaultSurplusRecovered)
				if err := _CollateralVault.contract.UnpackLog(event, "SurplusRecovered", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseSurplusRecovered is a log parse operation binding the contract event 0x8d062d570e15e665f57873e5745c84e024c3b6a4c94cfcda6af54a9ccd416af7.
//
// Solidity: event SurplusRecovered(address indexed receiver, uint256 amount)
func (_CollateralVault *CollateralVaultFilterer) ParseSurplusRecovered(log types.Log) (*CollateralVaultSurplusRecovered, error) {
	event := new(CollateralVaultSurplusRecovered)
	if err := _CollateralVault.contract.UnpackLog(event, "SurplusRecovered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CollateralVaultUnpausedIterator is returned from FilterUnpaused and is used to iterate over the raw logs and unpacked data for Unpaused events raised by the CollateralVault contract.
type CollateralVaultUnpausedIterator struct {
	Event *CollateralVaultUnpaused // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CollateralVaultUnpausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CollateralVaultUnpaused)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CollateralVaultUnpaused)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CollateralVaultUnpausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CollateralVaultUnpausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CollateralVaultUnpaused represents a Unpaused event raised by the CollateralVault contract.
type CollateralVaultUnpaused struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterUnpaused is a free log retrieval operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_CollateralVault *CollateralVaultFilterer) FilterUnpaused(opts *bind.FilterOpts) (*CollateralVaultUnpausedIterator, error) {

	logs, sub, err := _CollateralVault.contract.FilterLogs(opts, "Unpaused")
	if err != nil {
		return nil, err
	}
	return &CollateralVaultUnpausedIterator{contract: _CollateralVault.contract, event: "Unpaused", logs: logs, sub: sub}, nil
}

// WatchUnpaused is a free log subscription operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_CollateralVault *CollateralVaultFilterer) WatchUnpaused(opts *bind.WatchOpts, sink chan<- *CollateralVaultUnpaused) (event.Subscription, error) {

	logs, sub, err := _CollateralVault.contract.WatchLogs(opts, "Unpaused")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CollateralVaultUnpaused)
				if err := _CollateralVault.contract.UnpackLog(event, "Unpaused", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseUnpaused is a log parse operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_CollateralVault *CollateralVaultFilterer) ParseUnpaused(log types.Log) (*CollateralVaultUnpaused, error) {
	event := new(CollateralVaultUnpaused)
	if err := _CollateralVault.contract.UnpackLog(event, "Unpaused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
