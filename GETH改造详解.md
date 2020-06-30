##### GETH改造详解

1. 目的

   通过eth_getTransactionReceipt接口可以获取到内部交易的信息

2. 修改点

   - [ ] 新增ethereum/go-ethereum/core/types/internal_tx.go文件，增加InternalTx类型

   ```go
   package types
   
   import (
   	"math/big"
   
   	"github.com/ethereum/go-ethereum/common"
   )
   
   // InternalTx represents contract internla txs.
   type InternalTx struct {
   	From  common.Address `json:"from"`
   	To    common.Address `json:"to"`
   	Value *big.Int       `json:"value,omitempty"`
   	Depth uint64         `json:"depth,omitempty"`
   }
   
   ```

   - [ ] 修改github.com/ethereum/go-ethereum/core/types/receipt.go文件，在Receipt结构体中增加InternalTxs属性用于存储内部交易（具体修改请在此文件中搜索InternalTxs关键字）

   ```go
   // Receipt represents the results of a transaction.
   type Receipt struct {
   	// Consensus fields: These fields are defined by the Yellow Paper
   	PostState         []byte `json:"root"`
   	Status            uint64 `json:"status"`
   	CumulativeGasUsed uint64 `json:"cumulativeGasUsed" gencodec:"required"`
   	Bloom             Bloom  `json:"logsBloom"         gencodec:"required"`
   	Logs              []*Log `json:"logs"              gencodec:"required"`
   
   	// Implementation fields: These fields are added by geth when processing a transaction.
   	// They are stored in the chain database.
   	TxHash          common.Hash    `json:"transactionHash" gencodec:"required"`
   	ContractAddress common.Address `json:"contractAddress"`
   	GasUsed         uint64         `json:"gasUsed" gencodec:"required"`
   
   	// Inclusion information: These fields provide information about the inclusion of the
   	// transaction corresponding to this receipt.
   	BlockHash        common.Hash `json:"blockHash,omitempty"`
   	BlockNumber      *big.Int    `json:"blockNumber,omitempty"`
   	TransactionIndex uint        `json:"transactionIndex"`
   
   	InternalTxs []*InternalTx `json:"internalTx"`   // geth改造新增部分
   }
   ```

   - [ ] 修改github.com/ethereum/go-ethereum/core/vm/evm.go文件，在EVM结构体中增加InternalTxs属性，用于保存单笔中的所有内部交易信息；增加SetInternalTx方法，每次发生转账或余额变动的时候会记录一条内部交易信息到EVM.InternalTxs，该函数还会判断回退，将无效的内部交易删除

   ```go
   type EVM struct {
   	// Context provides auxiliary blockchain related information
   	Context
   	// StateDB gives access to the underlying state
   	StateDB StateDB
   	// Depth is the current call stack
   	depth int
   
   	// chainConfig contains information about the current chain
   	chainConfig *params.ChainConfig
   	// chain rules contains the chain rules for the current epoch
   	chainRules params.Rules
   	// virtual machine configuration options used to initialise the
   	// evm.
   	vmConfig Config
   	// global (to this context) ethereum virtual machine
   	// used throughout the execution of the tx.
   	interpreters []Interpreter
   
   	InternalTxs []*types.InternalTx  // GETH改造新增部分
   
   	interpreter Interpreter
   	// abort is used to abort the EVM calling operations
   	// NOTE: must be set atomically
   	abort int32
   	// callGasTemp holds the gas available for the current call. This is needed because the
   	// available gas is calculated in gasCall* according to the 63/64 rule and later
   	// applied in opCall*.
   	callGasTemp uint64
   }
   ```

   ```go
   func SetInternalTx(evm *EVM, from common.Address, to common.Address, value *big.Int, depth uint64, revert bool) {
   	if revert {
   		length := len(evm.InternalTxs)
   		if length >= 1 {
   			stop := length
   			for i := length - 1; i >= 0; i-- {
   				if evm.InternalTxs[i].Depth > uint64(evm.depth) {
   					stop = i
   				} else {
   					break
   				}
   			}
   			evm.InternalTxs = evm.InternalTxs[:stop]
   		}
   	} else {
   		itx := &types.InternalTx{From: from, To: to, Value: new(big.Int).Set(value), Depth: depth}
   		evm.InternalTxs = append(evm.InternalTxs, itx)
   	}
   }
   ```

   - [ ] 修改github.com/ethereum/go-ethereum/core/vm/instructions.go文件，对于suicide类型也要做一次SetInternalTx

   ```go
   func opSuicide(pc *uint64, interpreter *EVMInterpreter, callContext *callCtx) ([]byte, error) {
   	balance := interpreter.evm.StateDB.GetBalance(callContext.contract.Address())
   
   	rewardAddress := common.BigToAddress(callContext.stack.pop())
   	interpreter.evm.StateDB.AddBalance(rewardAddress, balance)
   
   	interpreter.evm.StateDB.Suicide(callContext.contract.Address())
   
   	SetInternalTx(interpreter.evm, callContext.contract.Address(), rewardAddress, balance, uint64(interpreter.evm.depth), false)   // GETH 改造新增部分
   	return nil, nil
   }
   
   ```

   - [ ] 修改github.com/ethereum/go-ethereum/core/state_processor.go文件，将交易执行完成后EVM中保存的内部交易写入receipt中

   ```go
   	for _, itx := range vmenv.InternalTxs {
   		if itx.Depth == 0 {
   			continue
   		}
   		if itx.Value.Cmp(big.NewInt(0)) == 0 {
   			continue
   		}
   
   		receipt.InternalTxs = append(receipt.InternalTxs, itx)
   	}
   ```

   

3. 总结

   geth修改后，可以直接通过eth_getTransactionReceipt返回值中的internalTx字段来分析内部交易，且此时获取的内部交易都是正确执行的（可以和etherscan对比），注意InternalTx不包括挖矿和外部交易。