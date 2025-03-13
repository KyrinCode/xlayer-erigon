package e2e

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/holiman/uint256"
	ethereum "github.com/ledgerwatch/erigon"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/accounts/abi"
	"github.com/ledgerwatch/erigon/accounts/abi/bind"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/ethclient"
	"gopkg.in/yaml.v2"

	"github.com/ledgerwatch/erigon/test/operations"
	"github.com/ledgerwatch/erigon/zkevm/encoding"
	"github.com/ledgerwatch/erigon/zkevm/etherman/smartcontracts/polygonzkevmbridge"
	"github.com/ledgerwatch/erigon/zkevm/log"

	"github.com/stretchr/testify/require"
)

const (
	blockAddress    = "0xdD2FD4581271e230360230F9337D5c0430Bf44C0"
	blockPrivateKey = "0xde9be858da4a475276426320d5e9262ecfc3ba460bfac56360bfa6c4c28b4ee0"

	testVerified                       = true
	tmpSenderPrivateKey                = "363ea277eec54278af051fb574931aec751258450a286edce9e1f64401f3b9c8"
	specificProjectSenderPrivateKey    = "100f4e42de757bdfa31122dfc5bc00f1afe508b7b3c214a96fa00cbf05d979cf"
	nonSpecificProjectSenderPrivateKay = "fc1c22e30c1d9e4f6449b3c44c2991dbc6a202d8895d18fefa224296c01949cd"
	erc20ABIJson                       = "[\n\t{\n\t\t\"inputs\": [],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"constructor\"\n\t},\n\t{\n\t\t\"anonymous\": false,\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"indexed\": true,\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"owner\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"indexed\": true,\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"spender\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"indexed\": false,\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"value\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"Approval\",\n\t\t\"type\": \"event\"\n\t},\n\t{\n\t\t\"anonymous\": false,\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"indexed\": true,\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"from\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"indexed\": true,\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"to\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"indexed\": false,\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"value\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"Transfer\",\n\t\t\"type\": \"event\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"owner\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"spender\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"allowance\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"spender\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"amount\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"approve\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"bool\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"bool\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"account\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"balanceOf\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [],\n\t\t\"name\": \"decimals\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint8\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"uint8\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"spender\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"subtractedValue\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"decreaseAllowance\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"bool\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"bool\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"spender\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"addedValue\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"increaseAllowance\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"bool\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"bool\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [],\n\t\t\"name\": \"name\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"string\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"string\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [],\n\t\t\"name\": \"symbol\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"string\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"string\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [],\n\t\t\"name\": \"totalSupply\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"view\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"to\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"amount\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"transfer\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"bool\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"bool\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"function\"\n\t},\n\t{\n\t\t\"inputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"from\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"address\",\n\t\t\t\t\"name\": \"to\",\n\t\t\t\t\"type\": \"address\"\n\t\t\t},\n\t\t\t{\n\t\t\t\t\"internalType\": \"uint256\",\n\t\t\t\t\"name\": \"amount\",\n\t\t\t\t\"type\": \"uint256\"\n\t\t\t}\n\t\t],\n\t\t\"name\": \"transferFrom\",\n\t\t\"outputs\": [\n\t\t\t{\n\t\t\t\t\"internalType\": \"bool\",\n\t\t\t\t\"name\": \"\",\n\t\t\t\t\"type\": \"bool\"\n\t\t\t}\n\t\t],\n\t\t\"stateMutability\": \"nonpayable\",\n\t\t\"type\": \"function\"\n\t}\n]"
	erc20BytecodeStr                   = "60806040523480156200001157600080fd5b506040518060400160405280600781526020017f4d79546f6b656e000000000000000000000000000000000000000000000000008152506040518060400160405280600381526020017f4d544b000000000000000000000000000000000000000000000000000000000081525081600390816200008f9190620004e4565b508060049081620000a19190620004e4565b505050620000e433620000b9620000ea60201b60201c565b600a620000c791906200075b565b6305f5e100620000d89190620007ac565b620000f360201b60201c565b620008e3565b60006012905090565b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff160362000165576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016200015c9062000858565b60405180910390fd5b62000179600083836200026060201b60201c565b80600260008282546200018d91906200087a565b92505081905550806000808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055508173ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef83604051620002409190620008c6565b60405180910390a36200025c600083836200026560201b60201c565b5050565b505050565b505050565b600081519050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b60006002820490506001821680620002ec57607f821691505b602082108103620003025762000301620002a4565b5b50919050565b60008190508160005260206000209050919050565b60006020601f8301049050919050565b600082821b905092915050565b6000600883026200036c7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff826200032d565b6200037886836200032d565b95508019841693508086168417925050509392505050565b6000819050919050565b6000819050919050565b6000620003c5620003bf620003b98462000390565b6200039a565b62000390565b9050919050565b6000819050919050565b620003e183620003a4565b620003f9620003f082620003cc565b8484546200033a565b825550505050565b600090565b6200041062000401565b6200041d818484620003d6565b505050565b5b8181101562000445576200043960008262000406565b60018101905062000423565b5050565b601f82111562000494576200045e8162000308565b62000469846200031d565b8101602085101562000479578190505b6200049162000488856200031d565b83018262000422565b50505b505050565b600082821c905092915050565b6000620004b96000198460080262000499565b1980831691505092915050565b6000620004d48383620004a6565b9150826002028217905092915050565b620004ef826200026a565b67ffffffffffffffff8111156200050b576200050a62000275565b5b620005178254620002d3565b6200052482828562000449565b600060209050601f8311600181146200055c576000841562000547578287015190505b620005538582620004c6565b865550620005c3565b601f1984166200056c8662000308565b60005b8281101562000596578489015182556001820191506020850194506020810190506200056f565b86831015620005b65784890151620005b2601f891682620004a6565b8355505b6001600288020188555050505b505050505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b60008160011c9050919050565b6000808291508390505b60018511156200065957808604811115620006315762000630620005cb565b5b6001851615620006415780820291505b80810290506200065185620005fa565b945062000611565b94509492505050565b60008262000674576001905062000747565b8162000684576000905062000747565b81600181146200069d5760028114620006a857620006de565b600191505062000747565b60ff841115620006bd57620006bc620005cb565b5b8360020a915084821115620006d757620006d6620005cb565b5b5062000747565b5060208310610133831016604e8410600b8410161715620007185782820a905083811115620007125762000711620005cb565b5b62000747565b62000727848484600162000607565b92509050818404811115620007415762000740620005cb565b5b81810290505b9392505050565b600060ff82169050919050565b6000620007688262000390565b915062000775836200074e565b9250620007a47fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff848462000662565b905092915050565b6000620007b98262000390565b9150620007c68362000390565b9250828202620007d68162000390565b91508282048414831517620007f057620007ef620005cb565b5b5092915050565b600082825260208201905092915050565b7f45524332303a206d696e7420746f20746865207a65726f206164647265737300600082015250565b600062000840601f83620007f7565b91506200084d8262000808565b602082019050919050565b60006020820190508181036000830152620008738162000831565b9050919050565b6000620008878262000390565b9150620008948362000390565b9250828201905080821115620008af57620008ae620005cb565b5b92915050565b620008c08162000390565b82525050565b6000602082019050620008dd6000830184620008b5565b92915050565b61122f80620008f36000396000f3fe608060405234801561001057600080fd5b50600436106100a95760003560e01c80633950935111610071578063395093511461016857806370a082311461019857806395d89b41146101c8578063a457c2d7146101e6578063a9059cbb14610216578063dd62ed3e14610246576100a9565b806306fdde03146100ae578063095ea7b3146100cc57806318160ddd146100fc57806323b872dd1461011a578063313ce5671461014a575b600080fd5b6100b6610276565b6040516100c39190610b0c565b60405180910390f35b6100e660048036038101906100e19190610bc7565b610308565b6040516100f39190610c22565b60405180910390f35b61010461032b565b6040516101119190610c4c565b60405180910390f35b610134600480360381019061012f9190610c67565b610335565b6040516101419190610c22565b60405180910390f35b610152610364565b60405161015f9190610cd6565b60405180910390f35b610182600480360381019061017d9190610bc7565b61036d565b60405161018f9190610c22565b60405180910390f35b6101b260048036038101906101ad9190610cf1565b6103a4565b6040516101bf9190610c4c565b60405180910390f35b6101d06103ec565b6040516101dd9190610b0c565b60405180910390f35b61020060048036038101906101fb9190610bc7565b61047e565b60405161020d9190610c22565b60405180910390f35b610230600480360381019061022b9190610bc7565b6104f5565b60405161023d9190610c22565b60405180910390f35b610260600480360381019061025b9190610d1e565b610518565b60405161026d9190610c4c565b60405180910390f35b60606003805461028590610d8d565b80601f01602080910402602001604051908101604052809291908181526020018280546102b190610d8d565b80156102fe5780601f106102d3576101008083540402835291602001916102fe565b820191906000526020600020905b8154815290600101906020018083116102e157829003601f168201915b5050505050905090565b60008061031361059f565b90506103208185856105a7565b600191505092915050565b6000600254905090565b60008061034061059f565b905061034d858285610770565b6103588585856107fc565b60019150509392505050565b60006012905090565b60008061037861059f565b905061039981858561038a8589610518565b6103949190610ded565b6105a7565b600191505092915050565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b6060600480546103fb90610d8d565b80601f016020809104026020016040519081016040528092919081815260200182805461042790610d8d565b80156104745780601f1061044957610100808354040283529160200191610474565b820191906000526020600020905b81548152906001019060200180831161045757829003601f168201915b5050505050905090565b60008061048961059f565b905060006104978286610518565b9050838110156104dc576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016104d390610e93565b60405180910390fd5b6104e982868684036105a7565b60019250505092915050565b60008061050061059f565b905061050d8185856107fc565b600191505092915050565b6000600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905092915050565b600033905090565b600073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff1603610616576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161060d90610f25565b60405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff1603610685576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161067c90610fb7565b60405180910390fd5b80600160008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508173ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925836040516107639190610c4c565b60405180910390a3505050565b600061077c8484610518565b90507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff81146107f657818110156107e8576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016107df90611023565b60405180910390fd5b6107f584848484036105a7565b5b50505050565b600073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff160361086b576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401610862906110b5565b60405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16036108da576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016108d190611147565b60405180910390fd5b6108e5838383610a72565b60008060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490508181101561096b576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401610962906111d9565b60405180910390fd5b8181036000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550816000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055508273ffffffffffffffffffffffffffffffffffffffff168473ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef84604051610a599190610c4c565b60405180910390a3610a6c848484610a77565b50505050565b505050565b505050565b600081519050919050565b600082825260208201905092915050565b60005b83811015610ab6578082015181840152602081019050610a9b565b60008484015250505050565b6000601f19601f8301169050919050565b6000610ade82610a7c565b610ae88185610a87565b9350610af8818560208601610a98565b610b0181610ac2565b840191505092915050565b60006020820190508181036000830152610b268184610ad3565b905092915050565b600080fd5b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b6000610b5e82610b33565b9050919050565b610b6e81610b53565b8114610b7957600080fd5b50565b600081359050610b8b81610b65565b92915050565b6000819050919050565b610ba481610b91565b8114610baf57600080fd5b50565b600081359050610bc181610b9b565b92915050565b60008060408385031215610bde57610bdd610b2e565b5b6000610bec85828601610b7c565b9250506020610bfd85828601610bb2565b9150509250929050565b60008115159050919050565b610c1c81610c07565b82525050565b6000602082019050610c376000830184610c13565b92915050565b610c4681610b91565b82525050565b6000602082019050610c616000830184610c3d565b92915050565b600080600060608486031215610c8057610c7f610b2e565b5b6000610c8e86828701610b7c565b9350506020610c9f86828701610b7c565b9250506040610cb086828701610bb2565b9150509250925092565b600060ff82169050919050565b610cd081610cba565b82525050565b6000602082019050610ceb6000830184610cc7565b92915050565b600060208284031215610d0757610d06610b2e565b5b6000610d1584828501610b7c565b91505092915050565b60008060408385031215610d3557610d34610b2e565b5b6000610d4385828601610b7c565b9250506020610d5485828601610b7c565b9150509250929050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b60006002820490506001821680610da557607f821691505b602082108103610db857610db7610d5e565b5b50919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b6000610df882610b91565b9150610e0383610b91565b9250828201905080821115610e1b57610e1a610dbe565b5b92915050565b7f45524332303a2064656372656173656420616c6c6f77616e63652062656c6f7760008201527f207a65726f000000000000000000000000000000000000000000000000000000602082015250565b6000610e7d602583610a87565b9150610e8882610e21565b604082019050919050565b60006020820190508181036000830152610eac81610e70565b9050919050565b7f45524332303a20617070726f76652066726f6d20746865207a65726f2061646460008201527f7265737300000000000000000000000000000000000000000000000000000000602082015250565b6000610f0f602483610a87565b9150610f1a82610eb3565b604082019050919050565b60006020820190508181036000830152610f3e81610f02565b9050919050565b7f45524332303a20617070726f766520746f20746865207a65726f20616464726560008201527f7373000000000000000000000000000000000000000000000000000000000000602082015250565b6000610fa1602283610a87565b9150610fac82610f45565b604082019050919050565b60006020820190508181036000830152610fd081610f94565b9050919050565b7f45524332303a20696e73756666696369656e7420616c6c6f77616e6365000000600082015250565b600061100d601d83610a87565b915061101882610fd7565b602082019050919050565b6000602082019050818103600083015261103c81611000565b9050919050565b7f45524332303a207472616e736665722066726f6d20746865207a65726f20616460008201527f6472657373000000000000000000000000000000000000000000000000000000602082015250565b600061109f602583610a87565b91506110aa82611043565b604082019050919050565b600060208201905081810360008301526110ce81611092565b9050919050565b7f45524332303a207472616e7366657220746f20746865207a65726f206164647260008201527f6573730000000000000000000000000000000000000000000000000000000000602082015250565b6000611131602383610a87565b915061113c826110d5565b604082019050919050565b6000602082019050818103600083015261116081611124565b9050919050565b7f45524332303a207472616e7366657220616d6f756e742065786365656473206260008201527f616c616e63650000000000000000000000000000000000000000000000000000602082015250565b60006111c3602683610a87565b91506111ce82611167565b604082019050919050565b600060208201905081810360008301526111f2816111b6565b905091905056fea2646970667358221220a1e42afa780fa0b792c1b1544459f1223cd5f165dbd77fb15760adb1e937625e64736f6c63430008130033"
	erc20FreeGasAddressStr             = "0xAD1D01007a56EE0A4FFD0488fb58fC6500Cb1fbE"
	erc20ModifiedABIJson               = "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"_name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"_symbol\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"_initialSupply\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"allowance\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"approve\",\"inputs\":[{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"approveFor\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"balanceOf\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"decimals\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"mintFor\",\"inputs\":[{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"name\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"symbol\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"totalSupply\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"transfer\",\"inputs\":[{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"transferFrom\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"Approval\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"spender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Transfer\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"ERC20InsufficientAllowance\",\"inputs\":[{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"allowance\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"needed\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"ERC20InsufficientBalance\",\"inputs\":[{\"name\":\"sender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"balance\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"needed\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidApprover\",\"inputs\":[{\"name\":\"approver\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidReceiver\",\"inputs\":[{\"name\":\"receiver\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidSender\",\"inputs\":[{\"name\":\"sender\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC20InvalidSpender\",\"inputs\":[{\"name\":\"spender\",\"type\":\"address\",\"internalType\":\"address\"}]}]"
	erc20ModifiedBytecodeStr           = "608060405234801561001057600080fd5b50604051610d6a380380610d6a83398101604081905261002f916102a8565b8282600361003d83826103a3565b50600461004a82826103a3565b5050506100793361005f61008160201b60201c565b61006a90600a610560565b6100749084610576565b610086565b5050506105a0565b601290565b6001600160a01b0382166100b55760405163ec442f0560e01b8152600060048201526024015b60405180910390fd5b6100c1600083836100c5565b5050565b6001600160a01b0383166100f05780600260008282546100e5919061058d565b909155506101629050565b6001600160a01b038316600090815260208190526040902054818110156101435760405163391434e360e21b81526001600160a01b038516600482015260248101829052604481018390526064016100ac565b6001600160a01b03841660009081526020819052604090209082900390555b6001600160a01b03821661017e5760028054829003905561019d565b6001600160a01b03821660009081526020819052604090208054820190555b816001600160a01b0316836001600160a01b03167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040516101e291815260200190565b60405180910390a3505050565b634e487b7160e01b600052604160045260246000fd5b600082601f83011261021657600080fd5b81516001600160401b0381111561022f5761022f6101ef565b604051601f8201601f19908116603f011681016001600160401b038111828210171561025d5761025d6101ef565b60405281815283820160200185101561027557600080fd5b60005b8281101561029457602081860181015183830182015201610278565b506000918101602001919091529392505050565b6000806000606084860312156102bd57600080fd5b83516001600160401b038111156102d357600080fd5b6102df86828701610205565b602086015190945090506001600160401b038111156102fd57600080fd5b61030986828701610205565b925050604084015190509250925092565b600181811c9082168061032e57607f821691505b60208210810361034e57634e487b7160e01b600052602260045260246000fd5b50919050565b601f82111561039e57806000526020600020601f840160051c8101602085101561037b5750805b601f840160051c820191505b8181101561039b5760008155600101610387565b50505b505050565b81516001600160401b038111156103bc576103bc6101ef565b6103d0816103ca845461031a565b84610354565b6020601f82116001811461040457600083156103ec5750848201515b600019600385901b1c1916600184901b17845561039b565b600084815260208120601f198516915b828110156104345787850151825560209485019460019092019101610414565b50848210156104525786840151600019600387901b60f8161c191681555b50505050600190811b01905550565b634e487b7160e01b600052601160045260246000fd5b6001815b60018411156104b25780850481111561049657610496610461565b60018416156104a457908102905b60019390931c92800261047b565b935093915050565b6000826104c95750600161055a565b816104d65750600061055a565b81600181146104ec57600281146104f657610512565b600191505061055a565b60ff84111561050757610507610461565b50506001821b61055a565b5060208310610133831016604e8410600b8410161715610535575081810a61055a565b6105426000198484610477565b806000190482111561055657610556610461565b0290505b92915050565b600061056f60ff8416836104ba565b9392505050565b808202811582820484141761055a5761055a610461565b8082018082111561055a5761055a610461565b6107bb806105af6000396000f3fe608060405234801561001057600080fd5b50600436106100a95760003560e01c8063313ce56711610071578063313ce5671461012757806370a082311461013657806395d89b411461015f578063a9059cbb14610167578063da1919b31461017a578063dd62ed3e1461018f57600080fd5b806306fdde03146100ae578063095ea7b3146100cc57806318160ddd146100ef57806323b872dd146101015780632b99174614610114575b600080fd5b6100b66101c8565b6040516100c39190610604565b60405180910390f35b6100df6100da36600461066e565b61025a565b60405190151581526020016100c3565b6002545b6040519081526020016100c3565b6100df61010f366004610698565b610274565b6100df610122366004610698565b610298565b604051601281526020016100c3565b6100f36101443660046106d5565b6001600160a01b031660009081526020819052604090205490565b6100b66102af565b6100df61017536600461066e565b6102be565b61018d61018836600461066e565b6102cc565b005b6100f361019d3660046106f7565b6001600160a01b03918216600090815260016020908152604080832093909416825291909152205490565b6060600380546101d79061072a565b80601f01602080910402602001604051908101604052809291908181526020018280546102039061072a565b80156102505780601f1061022557610100808354040283529160200191610250565b820191906000526020600020905b81548152906001019060200180831161023357829003601f168201915b5050505050905090565b6000336102688185856102da565b60019150505b92915050565b6000336102828582856102ec565b61028d858585610370565b506001949350505050565b60006102a58484846102da565b5060019392505050565b6060600480546101d79061072a565b600033610268818585610370565b6102d682826103cf565b5050565b6102e78383836001610405565b505050565b6001600160a01b0383811660009081526001602090815260408083209386168352929052205460001981101561036a578181101561035b57604051637dc7a0d960e11b81526001600160a01b038416600482015260248101829052604481018390526064015b60405180910390fd5b61036a84848484036000610405565b50505050565b6001600160a01b03831661039a57604051634b637e8f60e11b815260006004820152602401610352565b6001600160a01b0382166103c45760405163ec442f0560e01b815260006004820152602401610352565b6102e78383836104da565b6001600160a01b0382166103f95760405163ec442f0560e01b815260006004820152602401610352565b6102d6600083836104da565b6001600160a01b03841661042f5760405163e602df0560e01b815260006004820152602401610352565b6001600160a01b03831661045957604051634a1406b160e11b815260006004820152602401610352565b6001600160a01b038085166000908152600160209081526040808320938716835292905220829055801561036a57826001600160a01b0316846001600160a01b03167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925846040516104cc91815260200190565b60405180910390a350505050565b6001600160a01b0383166105055780600260008282546104fa9190610764565b909155506105779050565b6001600160a01b038316600090815260208190526040902054818110156105585760405163391434e360e21b81526001600160a01b03851660048201526024810182905260448101839052606401610352565b6001600160a01b03841660009081526020819052604090209082900390555b6001600160a01b038216610593576002805482900390556105b2565b6001600160a01b03821660009081526020819052604090208054820190555b816001600160a01b0316836001600160a01b03167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040516105f791815260200190565b60405180910390a3505050565b602081526000825180602084015260005b818110156106325760208186018101516040868401015201610615565b506000604082850101526040601f19601f83011684010191505092915050565b80356001600160a01b038116811461066957600080fd5b919050565b6000806040838503121561068157600080fd5b61068a83610652565b946020939093013593505050565b6000806000606084860312156106ad57600080fd5b6106b684610652565b92506106c460208501610652565b929592945050506040919091013590565b6000602082840312156106e757600080fd5b6106f082610652565b9392505050565b6000806040838503121561070a57600080fd5b61071383610652565b915061072160208401610652565b90509250929050565b600181811c9082168061073e57607f821691505b60208210810361075e57634e487b7160e01b600052602260045260246000fd5b50919050565b8082018082111561026e57634e487b7160e01b600052601160045260246000fdfea2646970667358221220c8b6c2889a8904f3a349788bd9528aa9a36cf8f382bd57c8e75337a5938fd5a464736f6c634300081a0033"
)

func TestGetBatchSealTime(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// latest batch seal time
	var batchNum uint64
	var batchSealTime uint64
	var err error
	for i := 0; i < 50; i++ {
		batchNum, err = operations.GetBatchNumber()
		require.NoError(t, err)
		batchSealTime, err = operations.GetBatchSealTime(new(big.Int).SetUint64(batchNum))
		require.Equal(t, batchSealTime, uint64(0))
		log.Infof("Batch number: %d, times:%v", batchNum, i)
		if batchNum > 1 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// old batch seal time
	batchNum = batchNum - 1
	batch, err := operations.GetBatchByNumber(new(big.Int).SetUint64(batchNum))
	var maxTime uint64
	for _, block := range batch.Blocks {
		blockInfo, err := operations.GetBlockByHash(common.HexToHash(block.(string)))
		require.NoError(t, err)
		log.Infof("Block Timestamp: %+v", blockInfo.Timestamp)
		blockTime := uint64(blockInfo.Timestamp)
		if blockTime > maxTime {
			maxTime = blockTime
		}
	}
	batchSealTime, err = operations.GetBatchSealTime(new(big.Int).SetUint64(batchNum))
	require.NoError(t, err)
	log.Infof("Max block time: %d, batchSealTime: %d", maxTime, batchSealTime)
	require.Equal(t, maxTime, batchSealTime)
}

func TestBridgeTx(t *testing.T) {
	ctx := context.Background()
	l1Client, err := ethclient.Dial(operations.DefaultL1NetworkURL)
	require.NoError(t, err)
	l2Client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	transToken(t, ctx, l2Client, uint256.NewInt(encoding.Gwei), operations.DefaultL2AdminAddress)

	amount := new(big.Int).SetUint64(100)
	//layer2 network id
	var destNetwork uint32 = 1
	destAddr := common.HexToAddress(operations.DefaultL2AdminAddress)
	auth, err := operations.GetAuth(operations.DefaultL1AdminPrivateKey, operations.DefaultL1ChainID)
	require.NoError(t, err)

	wethAddress := common.HexToAddress("0x17a2a2e444a7f3446877d1b71eaa2b2ae7533baf")
	wethToken, err := operations.NewToken(wethAddress, l2Client)
	require.NoError(t, err)

	balanceBefore, err := wethToken.BalanceOf(&bind.CallOpts{}, destAddr)
	require.NoError(t, err)

	err = sendBridgeAsset(ctx, common.Address{}, amount, destNetwork, &destAddr, []byte{}, auth, common.HexToAddress(operations.BridgeAddr), l1Client)
	require.NoError(t, err)

	const maxAttempts = 120

	var balanceAfter *big.Int
	for i := 0; i < maxAttempts; i++ {
		time.Sleep(1 * time.Second)

		balanceAfter, err = wethToken.BalanceOf(&bind.CallOpts{}, destAddr)
		require.NoError(t, err)

		if balanceAfter.Cmp(balanceBefore) > 0 {
			return
		}
	}

	t.Errorf("bridge transaction failed after %d seconds: balance did not increase (before: %s, after: %s)",
		maxAttempts,
		balanceBefore.String(),
		balanceAfter.String(),
	)
}

func TestClaimTx(t *testing.T) {
	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	transToken(t, ctx, client, uint256.NewInt(encoding.Gwei), operations.DefaultL2AdminAddress)

	from := common.HexToAddress(operations.DefaultL2AdminAddress)
	to := common.HexToAddress(operations.DefaultL2AdminAddress)
	nonce, err := client.PendingNonceAt(ctx, from)
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  from,
		To:    &to,
		Value: uint256.NewInt(10),
	})
	require.NoError(t, err)
	var tx types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   gas,
			Value: uint256.NewInt(10),
		},
		GasPrice: uint256.MustFromBig(big.NewInt(0)),
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
}

func TestNewAccFreeGas(t *testing.T) {
	ctx := context.Background()
	client, _ := ethclient.Dial(operations.DefaultL2NetworkURL)
	transToken(t, ctx, client, uint256.NewInt(encoding.Gwei), operations.DefaultL2AdminAddress)
	var gas uint64 = 21000

	//newAcc transfer failed
	from := common.HexToAddress(operations.DefaultL2NewAcc1Address)
	to := common.HexToAddress(operations.DefaultL2AdminAddress)
	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	var tx types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   gas,
			Value: uint256.NewInt(0),
		},
		GasPrice: uint256.MustFromBig(big.NewInt(0)),
	}
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2NewAcc1PrivateKey, "0x"))
	require.NoError(t, err)
	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "RPC error response: FEE_TOO_LOW: underpriced"), "Expected error message not found")
	err = operations.WaitTxToBeMined(ctx, client, signedTx, 5)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "context deadline exceeded"), "Expected error message not found")

	// seq -> newAcc
	from = common.HexToAddress(operations.DefaultL2AdminAddress)
	to = common.HexToAddress(operations.DefaultL2NewAcc1Address)
	nonce, err = client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	tx = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   gas,
			Value: uint256.NewInt(10),
		},
		GasPrice: uint256.MustFromBig(big.NewInt(0)),
	}
	privateKey, err = crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL1AdminPrivateKey, "0x"))
	require.NoError(t, err)
	signedTx, err = types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)

	// newAcc transfer success
	from = common.HexToAddress(operations.DefaultL2NewAcc1Address)
	to = common.HexToAddress(operations.DefaultL2AdminAddress)
	nonce, err = client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	tx = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   gas,
			Value: uint256.NewInt(0),
		},
		GasPrice: uint256.MustFromBig(big.NewInt(0)),
	}
	privateKey, err = crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2NewAcc1PrivateKey, "0x"))
	require.NoError(t, err)
	signedTx, err = types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
}
func TestWhiteAndBlockList(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)

	from := common.HexToAddress(operations.DefaultL2AdminAddress)
	blockAddressConverted := common.HexToAddress(blockAddress)
	nonBlockAddress := common.HexToAddress(operations.DefaultL2NewAcc1Address)

	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)

	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  from,
		To:    &blockAddressConverted,
		Value: uint256.NewInt(10),
	})
	require.NoError(t, err)

	var txToBlockAddress types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &blockAddressConverted,
			Gas:   gas,
			Value: uint256.NewInt(10),
		},
		GasPrice: uint256.MustFromBig(gasPrice),
	}

	var txToNonBlockAddress types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &nonBlockAddress,
			Gas:   gas,
			Value: uint256.NewInt(10),
		},
		GasPrice: uint256.MustFromBig(gasPrice),
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)

	signedTxToBlockAddress, err := types.SignTx(txToBlockAddress, *signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTxToBlockAddress)
	log.Infof("err:%v", err)
	require.True(t, strings.Contains(err.Error(), "INTERNAL_ERROR: blocked receiver"))

	signedTxToNonBlockAddress, err := types.SignTx(txToNonBlockAddress, *signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTxToNonBlockAddress)
	require.NoError(t, err)

	//TODO: sender in blocklist should fail
	//now only admin account have balance. So we may add another account that has balance.
}
func TestTransferFromACL(t *testing.T) {
	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)

	// 0. Deploy erc20 contract
	adminPrivateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	adminPublicKey := adminPrivateKey.Public()
	adminPublicKeyECDSA, ok := adminPublicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	adminAddress := crypto.PubkeyToAddress(*adminPublicKeyECDSA)

	nonce, err := client.PendingNonceAt(ctx, adminAddress)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	auth, err := bind.NewKeyedTransactorWithChainID(adminPrivateKey, big.NewInt(int64(operations.DefaultL2ChainID)))
	require.NoError(t, err)
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = gasPrice

	simpleTokenABI, err := abi.JSON(strings.NewReader(erc20ModifiedABIJson))
	require.NoError(t, err)
	simpleTokenBytecode, err := hex.DecodeString(erc20ModifiedBytecodeStr)
	require.NoError(t, err)

	tokenAddress, tx, tokenContract, err := bind.DeployContract(auth, simpleTokenABI, simpleTokenBytecode, client, "SimpleToken", "SIM", big.NewInt(1000000))
	require.NoError(t, err)

	err = operations.WaitTxToBeMined(ctx, client, tx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
	log.Infof("contract deployed at: %s", tokenAddress.Hex())

	// Test case 1: TransferFrom with blocked 'from' address
	t.Run("BlockedFromAddress", func(t *testing.T) {
		// 1. Mint tokens to blocked address using mintFor
		blockedAddress := common.HexToAddress(blockAddress)
		mintAmount := new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18))

		nonce, err = client.PendingNonceAt(ctx, adminAddress)
		require.NoError(t, err)
		auth.Nonce = big.NewInt(int64(nonce))

		mintTx, err := tokenContract.Transact(auth, "mintFor", blockedAddress, mintAmount)
		require.NoError(t, err)
		err = operations.WaitTxToBeMined(ctx, client, mintTx, operations.DefaultTimeoutTxToBeMined)
		require.NoError(t, err)

		// Verify balance
		var balance []any
		err = tokenContract.Call(&bind.CallOpts{}, &balance, "balanceOf", blockedAddress)
		require.NoError(t, err)
		require.Equal(t, mintAmount.String(), balance[0].(*big.Int).String())

		// 2. Admin approves itself to spend tokens from blocked address using approveFor
		approveAmount := new(big.Int).Mul(big.NewInt(500), big.NewInt(1e18))
		nonce, err = client.PendingNonceAt(ctx, adminAddress)
		require.NoError(t, err)
		auth.Nonce = big.NewInt(int64(nonce))

		approveTx, err := tokenContract.Transact(auth, "approveFor", blockedAddress, adminAddress, approveAmount)
		require.NoError(t, err)
		err = operations.WaitTxToBeMined(ctx, client, approveTx, operations.DefaultTimeoutTxToBeMined)
		require.NoError(t, err)

		// Verify allowance
		var allowance []any
		err = tokenContract.Call(&bind.CallOpts{}, &allowance, "allowance", blockedAddress, adminAddress)
		require.NoError(t, err)
		require.Equal(t, approveAmount.String(), allowance[0].(*big.Int).String())

		// 3. Admin attempts transferFrom from blocked address
		nonce, err = client.PendingNonceAt(ctx, adminAddress)
		require.NoError(t, err)
		auth.Nonce = big.NewInt(int64(nonce))
		auth.GasLimit = 100000

		transferAmount := new(big.Int).Mul(big.NewInt(200), big.NewInt(1e18))
		_, err = tokenContract.Transact(auth, "transferFrom", blockedAddress, adminAddress, transferAmount)
		require.Error(t, err)
		require.Contains(t, err.Error(), "sender disallowed to send tx by ACL policy", "Expected transferFrom to fail due to blocked 'from' address")

		// Verify balance hasn't changed
		var balanceAfter []any
		err = tokenContract.Call(&bind.CallOpts{}, &balanceAfter, "balanceOf", blockedAddress)
		require.NoError(t, err)
		require.Equal(t, mintAmount.String(), balanceAfter[0].(*big.Int).String(), "Balance should not have changed")
	})

	// Test case 2: TransferFrom with non-blocked 'from' address
	t.Run("NonBlockedFromAddress", func(t *testing.T) {
		// 1. Mint tokens to non-blocked address using mintFor
		nonBlockedAddress := common.HexToAddress(operations.DefaultL2NewAcc1Address)
		mintAmount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18))

		nonce, err = client.PendingNonceAt(ctx, adminAddress)
		require.NoError(t, err)
		auth.Nonce = big.NewInt(int64(nonce))
		auth.GasLimit = 100000

		mintTx, err := tokenContract.Transact(auth, "mintFor", nonBlockedAddress, mintAmount)
		require.NoError(t, err)
		err = operations.WaitTxToBeMined(ctx, client, mintTx, operations.DefaultTimeoutTxToBeMined)
		require.NoError(t, err)

		// Verify balance
		var balanceBefore []any
		err = tokenContract.Call(&bind.CallOpts{}, &balanceBefore, "balanceOf", nonBlockedAddress)
		require.NoError(t, err)
		require.Equal(t, mintAmount.String(), balanceBefore[0].(*big.Int).String())

		// 2. Admin approves itself to spend tokens from non-blocked address using approveFor
		approveAmount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18))
		nonce, err = client.PendingNonceAt(ctx, adminAddress)
		require.NoError(t, err)
		auth.Nonce = big.NewInt(int64(nonce))

		approveTx, err := tokenContract.Transact(auth, "approveFor", nonBlockedAddress, adminAddress, approveAmount)
		require.NoError(t, err)
		err = operations.WaitTxToBeMined(ctx, client, approveTx, operations.DefaultTimeoutTxToBeMined)
		require.NoError(t, err)

		// Verify allowance
		var allowance []any
		err = tokenContract.Call(&bind.CallOpts{}, &allowance, "allowance", nonBlockedAddress, adminAddress)
		require.NoError(t, err)
		require.Equal(t, approveAmount.String(), allowance[0].(*big.Int).String())

		// 3. Admin performs transferFrom from non-blocked address to itself
		transferAmount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e15))
		nonce, err = client.PendingNonceAt(ctx, adminAddress)
		require.NoError(t, err)
		auth.Nonce = big.NewInt(int64(nonce))

		transferFromTx, err := tokenContract.Transact(auth, "transferFrom", nonBlockedAddress, adminAddress, transferAmount)
		require.NoError(t, err)

		// Wait for the transaction to be mined
		err = operations.WaitTxToBeMined(ctx, client, transferFromTx, operations.DefaultTimeoutTxToBeMined)
		require.NoError(t, err)

		// Verify balance has changed
		var balanceAfter []any
		err = tokenContract.Call(&bind.CallOpts{}, &balanceAfter, "balanceOf", nonBlockedAddress)
		require.NoError(t, err)
		expectedBalance := new(big.Int).Sub(mintAmount, transferAmount)
		require.Equal(t, expectedBalance.String(), balanceAfter[0].(*big.Int).String(), "Balance should have decreased by transfer amount")

		// Verify admin received the tokens
		var adminBalance []any
		err = tokenContract.Call(&bind.CallOpts{}, &adminBalance, "balanceOf", adminAddress)
		require.NoError(t, err)

		initialAdminBalance := new(big.Int).Mul(big.NewInt(1000000), big.NewInt(1e18))
		expectedAdminBalance := new(big.Int).Add(initialAdminBalance, transferAmount)
		require.Equal(t, expectedAdminBalance.String(), adminBalance[0].(*big.Int).String(), "Admin balance should have increased by transfer amount")
	})
}

func TestRPCAPI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config, err := LoadConfig("../../test/config/test.erigon.rpc.config.yaml")
	require.NoError(t, err)

	if config.HTTPAPIKeys != "" {

		_, err := operations.GetEthSyncing(operations.DefaultL2NetworkURL)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "no authentication"))

		_, err = operations.GetEthSyncing(operations.DefaultL2NetworkURL + "/45543e0adc5dd3e316044909d32501a5")
		require.NoError(t, err)
	} else {

		var rateErr error
		for i := 0; i < 1000; i++ {
			_, err1 := operations.GetEthSyncing(operations.DefaultL2NetworkURL)
			if err1 != nil {
				rateErr = err1
				break
			}
		}

		require.True(t, strings.Contains(rateErr.Error(), "rate limit exceeded"))
	}
}

func TestChainID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chainID, err := operations.GetNetVersion(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	require.Equal(t, chainID, operations.DefaultL2ChainID)
}

func TestInnerTx(t *testing.T) {
	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	txHash := transToken(t, ctx, client, uint256.NewInt(encoding.Gwei), operations.DefaultL2AdminAddress)
	log.Infof("txHash: %s", txHash)

	result, err := operations.GetInternalTransactions(common.HexToHash(txHash))
	require.NoError(t, err)
	require.Greater(t, len(result), 0)
	require.Equal(t, result[0].From, operations.DefaultL2AdminAddress)

	tx, err := operations.GetTransactionByHash(common.HexToHash(txHash))
	require.NoError(t, err)
	log.Infof("tx: %+v", tx.BlockNumber)
	result1, err := operations.GetBlockInternalTransactions(new(big.Int).SetUint64(uint64(*tx.BlockNumber)))
	require.NoError(t, err)
	require.Greater(t, len(result1), 0)
	require.Equal(t, result1[common.HexToHash(txHash)][0].From, operations.DefaultL2AdminAddress)
}

func TestEthTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	if !testVerified {
		return
	}

	ctx := context.Background()
	auth, err := operations.GetAuth(operations.DefaultL2AdminPrivateKey, operations.DefaultL2ChainID)
	require.NoError(t, err)
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)

	from := common.HexToAddress(operations.DefaultL2AdminAddress)
	to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	var tx types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   21000,
			Value: uint256.NewInt(0),
		},
		GasPrice: uint256.NewInt(10 * encoding.Gwei),
	}
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)
	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(tx, *signer, privateKey)
	var txs []*types.Transaction
	txs = append(txs, &signedTx)
	_, err = operations.ApplyL2Txs(ctx, txs, auth, client, operations.VerifiedConfirmationLevel)
	require.NoError(t, err)
}

func TestGasPrice(t *testing.T) {
	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	log.Infof("Start TestGasPrice")
	gasPrice1, err := operations.GetGasPrice()
	gasPrice2 := gasPrice1
	require.NoError(t, err)
	for i := 1; i < 100; i++ {
		temp, err := operations.GetGasPrice()
		require.NoError(t, err)
		if temp > gasPrice2 {
			gasPrice2 = temp
		}
		require.NoError(t, err)

		from := common.HexToAddress(operations.DefaultL2AdminAddress)
		to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
		nonce, err := client.PendingNonceAt(ctx, from)
		require.NoError(t, err)
		var tx types.Transaction = &types.LegacyTx{
			CommonTx: types.CommonTx{
				Nonce: nonce,
				To:    &to,
				Gas:   21000,
				Value: uint256.NewInt(0),
			},
			GasPrice: uint256.NewInt(uint64(i) * 10 * encoding.Gwei),
		}
		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
		require.NoError(t, err)
		signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
		signedTx, err := types.SignTx(tx, *signer, privateKey)
		require.NoError(t, err)
		log.Infof("Get new GP:%v, TXGP:%v", temp, tx.GetPrice())
		err = client.SendTransaction(ctx, signedTx)
		time.Sleep(500 * time.Millisecond)
		//err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
		//require.NoError(t, err)
		if gasPrice2 > gasPrice1 {
			log.Infof("GP compare ok: [%d,%d]", gasPrice1, gasPrice2)
			break
		}
	}
	require.NoError(t, err)
	log.Infof("gasPrice: [%d,%d]", gasPrice1, gasPrice2)
	require.Greater(t, gasPrice2, gasPrice1)
}

func TestMetrics(t *testing.T) {
	result, err := operations.GetMetricsPrometheus()
	require.NoError(t, err)
	require.Equal(t, strings.Contains(result, "sequencer_batch_execute_time"), true)
	require.Equal(t, strings.Contains(result, "sequencer_pool_tx_count"), true)

	// TODO: enable this test after metrics are enabled
	//result, err = operations.GetMetrics()
	//require.NoError(t, err)
	//require.Equal(t, strings.Contains(result, "zkevm_getBatchWitness"), true)
	//require.Equal(t, strings.Contains(result, "eth_sendRawTransaction"), true)
	//require.Equal(t, strings.Contains(result, "eth_getTransactionCount"), true)
}

func transToken(t *testing.T, ctx context.Context, client *ethclient.Client, amount *uint256.Int, toAddress string) string {
	return transTokenWithFrom(t, ctx, client, operations.DefaultL2AdminPrivateKey, amount, toAddress)
}

func transTokenWithFrom(t *testing.T, ctx context.Context, client *ethclient.Client, fromPrivateKey string, amount *uint256.Int, toAddress string) string {
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := operations.GetAuth(fromPrivateKey, chainID.Uint64())
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	to := common.HexToAddress(toAddress)
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  auth.From,
		To:    &to,
		Value: amount,
	})
	require.NoError(t, err)
	log.Infof("gas: %d", gas)
	log.Infof("gasPrice: %d", gasPrice)

	var tx types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   gas,
			Value: amount,
		},
		GasPrice: uint256.MustFromBig(gasPrice),
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(fromPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)

	return signedTx.Hash().String()
}

func TestMinGasPrice(t *testing.T) {
	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	log.Infof("Start TestMinGasPrice")
	require.NoError(t, err)
	for i := 1; i < 3; i++ {
		temp, err := operations.GetMinGasPrice()
		log.Infof("minGP: [%d]", temp)
		if temp > 1 {
			temp = temp - 1
		}
		require.NoError(t, err)

		from := common.HexToAddress(operations.DefaultL2NewAcc2Address)
		to := common.HexToAddress(operations.DefaultL1AdminAddress)
		nonce, err := client.PendingNonceAt(ctx, from)
		require.NoError(t, err)
		var tx types.Transaction = &types.LegacyTx{
			CommonTx: types.CommonTx{
				Nonce: nonce,
				To:    &to,
				Gas:   21000,
				Value: uint256.NewInt(0),
			},
			GasPrice: uint256.NewInt(temp),
		}
		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2NewAcc2PrivateKey, "0x"))
		require.NoError(t, err)
		signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
		signedTx, err := types.SignTx(tx, *signer, privateKey)
		require.NoError(t, err)
		log.Infof("GP:%v", tx.GetPrice())
		err = client.SendTransaction(ctx, signedTx)
		require.Error(t, err)
	}
	for i := 3; i < 5; i++ {
		temp, err := operations.GetMinGasPrice()
		log.Infof("minGP: [%d]", temp)
		require.NoError(t, err)

		from := common.HexToAddress(operations.DefaultL2AdminAddress)
		to := common.HexToAddress(operations.DefaultL1AdminAddress)
		nonce, err := client.PendingNonceAt(ctx, from)
		require.NoError(t, err)
		var tx types.Transaction = &types.LegacyTx{
			CommonTx: types.CommonTx{
				Nonce: nonce,
				To:    &to,
				Gas:   21000,
				Value: uint256.NewInt(0),
			},
			GasPrice: uint256.NewInt(temp),
		}
		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
		require.NoError(t, err)
		signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
		signedTx, err := types.SignTx(tx, *signer, privateKey)
		require.NoError(t, err)
		log.Infof("GP:%v", tx.GetPrice())
		err = client.SendTransaction(ctx, signedTx)
		require.NoError(t, err)
	}
	require.NoError(t, err)
}

func sendBridgeAsset(
	ctx context.Context, tokenAddr common.Address, amount *big.Int, destNetwork uint32, destAddr *common.Address,
	metadata []byte, auth *bind.TransactOpts, bridgeSCAddr common.Address, c *ethclient.Client,
) error {
	emptyAddr := common.Address{}
	if tokenAddr == emptyAddr {
		auth.Value = amount
	}
	if destAddr == nil {
		destAddr = &auth.From
	}
	if len(bridgeSCAddr) == 0 {
		return fmt.Errorf("Bridge address error")
	}

	br, err := polygonzkevmbridge.NewPolygonzkevmbridge(bridgeSCAddr, c)
	if err != nil {
		return err
	}
	tx, err := br.BridgeAsset(auth, destNetwork, *destAddr, amount, tokenAddr, true, metadata)
	if err != nil {
		return err
	}
	// wait transfer to be included in a batch
	const txTimeout = 60 * time.Second
	return operations.WaitTxToBeMined(ctx, c, tx, txTimeout)
}

type Config struct {
	HTTPMethodRateLimit string `yaml:"http.methodratelimit"`
	HTTPAPIKeys         string `yaml:"http.apikeys"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func TestSpecificProjectFreeGas(t *testing.T) {
	// transfer token to the new account
	tmpPrivateKey, err := crypto.HexToECDSA(tmpSenderPrivateKey)
	require.NoError(t, err)

	tmpPublicKey := tmpPrivateKey.Public()
	tmpPublicKeyECDSA, ok := tmpPublicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	tmpFromAddress := crypto.PubkeyToAddress(*tmpPublicKeyECDSA)
	ctx := context.Background()

	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	transToken(t, ctx, client,
		new(uint256.Int).Mul(uint256.NewInt(1000), uint256.NewInt(1e18)),
		tmpFromAddress.String())

	// send to specific project sender
	privateKey, err := crypto.HexToECDSA(specificProjectSenderPrivateKey)
	require.NoError(t, err)

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	transTokenWithFrom(t, ctx, client, "0x"+tmpSenderPrivateKey,
		new(uint256.Int).Mul(uint256.NewInt(100), uint256.NewInt(1e18)),
		fromAddress.String())

	erc20FreeGasAddress := common.HexToAddress(erc20FreeGasAddressStr)

	code, err := client.CodeAt(ctx, erc20FreeGasAddress, nil)
	require.NoError(t, err)
	erc20ABI, err := abi.JSON(strings.NewReader(erc20ABIJson))
	require.NoError(t, err)
	if len(code) == 0 {
		// Fetch nonce
		nonce, err := client.PendingNonceAt(ctx, fromAddress)
		require.NoError(t, err)

		log.Infof("Nonce: %d", nonce)

		// Define gas parameters
		gasPrice, err := client.SuggestGasPrice(ctx)
		require.NoError(t, err)

		// Set up transaction options
		auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(195))
		require.NoError(t, err)

		auth.Nonce = big.NewInt(int64(nonce))
		auth.Value = big.NewInt(0)
		auth.GasLimit = uint64(3000000)
		auth.GasPrice = gasPrice

		// Deploy the contract
		erc20Bytecode, err := hex.DecodeString(erc20BytecodeStr)
		require.NoError(t, err)
		erc20Address, tx, _, err := bind.DeployContract(auth, erc20ABI, erc20Bytecode, client)
		require.NoError(t, err)

		log.Infof("Contract deployed at: %s, transaction hash: %s", erc20Address.Hex(), tx.Hash().Hex())

		// Wait for contract deployment to be mined
		bind.WaitDeployed(ctx, client, tx)
	}

	amount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)) // Adjust for token decimals (18 in this case)
	// Prepare transfer data
	data, err := erc20ABI.Pack("transfer", common.HexToAddress(blockAddress), amount)
	require.NoError(t, err)
	// Get the sender's nonce
	freeGasNonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	require.NoError(t, err)

	// Create the transaction with free gas
	freeGasTx := &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: freeGasNonce,
			To:    &erc20FreeGasAddress,
			Gas:   60000,
			Value: uint256.NewInt(0),
			Data:  data,
		},
		GasPrice: uint256.NewInt(0),
	}

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(freeGasTx, *signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
	receipt, err := client.TransactionReceipt(ctx, signedTx.Hash())
	require.NoError(t, err)
	log.Infof("receipt: %+v", receipt)

	// Send with gas price
	freeGasNonceWithGp, err := client.PendingNonceAt(context.Background(), fromAddress)
	require.NoError(t, err)
	freeGasTxWithGp := &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: freeGasNonceWithGp,
			To:    &erc20FreeGasAddress,
			Gas:   60000,
			Value: uint256.NewInt(0),
			Data:  data,
		},
		GasPrice: uint256.NewInt(100),
	}

	signerWithGp := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTxWithGp, err := types.SignTx(freeGasTxWithGp, *signerWithGp, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTxWithGp)
	require.NoError(t, err)
	err = operations.WaitTxToBeMined(ctx, client, signedTxWithGp, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
	receiptWithGp, err := client.TransactionReceipt(ctx, signedTx.Hash())
	require.NoError(t, err)
	log.Infof("receipt: %+v", receiptWithGp)

	// send to non specific project sender
	privateKeyNon, err := crypto.HexToECDSA(nonSpecificProjectSenderPrivateKay)
	require.NoError(t, err)

	publicKeyNon := privateKeyNon.Public()
	publicKeyECDSANon, ok := publicKeyNon.(*ecdsa.PublicKey)
	require.True(t, ok)
	fromAddressNon := crypto.PubkeyToAddress(*publicKeyECDSANon)

	transTokenWithFrom(t, ctx, client, "0x"+tmpSenderPrivateKey,
		new(uint256.Int).Mul(uint256.NewInt(100), uint256.NewInt(1e18)),
		fromAddressNon.String())

	// not allowed from address
	freeGasNonceNon, err := client.PendingNonceAt(context.Background(), fromAddressNon)
	require.NoError(t, err)
	freeGasTxNon := &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: freeGasNonceNon,
			To:    &erc20FreeGasAddress,
			Gas:   60000,
			Value: uint256.NewInt(0),
			Data:  data,
		},
		GasPrice: uint256.NewInt(100),
	}

	signerNon := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTxNon, err := types.SignTx(freeGasTxNon, *signerNon, privateKeyNon)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTxNon)
	require.ErrorContains(t, err, "FEE_TOO_LOW")
}
