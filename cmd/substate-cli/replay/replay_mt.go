package replay

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/big"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"

	fuzz "github.com/ethereum/go-ethereum/cmd/substate-cli/fuzz"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/research"
	cli "gopkg.in/urfave/cli.v1"
)

// this commit is to-be-done
var ReplaySICommand = cli.Command{
	Action:    replaySIAction,
	Name:      "replay-SI",
	Usage:     "executes full state transitions and check state consistency",
	ArgsUsage: "<blockNumFirst> <blockNumLast> --dappDir <dappFolderPath> --substateDir <substatePath>",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipEnvFlag,
		research.SkipTodFlag,
		research.SkipManiFlag,
		research.SkipHookFlag,
		research.RichInfoFlag,
		research.SubstateDirFlag,
		research.DappDirFlag,
	},
	Description: `
The substate-cli replay-mt command requires four arguments:
<blockNumFirst> <blockNumLast> <dappFolderPath> <substatePath>

<blockNumFirst> and <blockNumLast> are the first and
last block of the inclusive range of blocks to replay transactions.

<dappFolderPath> and <substatePath> are the path of dapp data folder 
and the path of substate that previously replay`,
}

var (
	tenEther        *big.Int
	errorLogFile    *os.File
	bugLogFile      *os.File
	icyStateLogFile *os.File
	pastBlocks      []uint64
)

type SIbug struct {
	BugType          string                   `json:"BugType"`
	InputAlloc       research.SubstateAlloc   `json:"inputAlloc"`
	OutputAlloc      research.SubstateAlloc   `json:"outputAlloc"`
	InputMessage     research.SubstateMessage `json:"inputMessage"`
	AdditMessageFrom string                   `json:"additMessageFrom"`
	AdditMessageTo   string                   `json:"additMessageTo"`
	AdditMessageData string                   `json:"additMessageData"`
	OriAlloc         research.SubstateAlloc   `json:"oriAlloc"`
	MutAlloc         research.SubstateAlloc   `json:"mutAlloc"`
}

// record-replay: func replayAction for replay command
func replaySIAction(ctx *cli.Context) error {
	var err error

	if len(ctx.Args()) != 2 {
		return fmt.Errorf("substate-cli replay-mt command requires exactly 2 arguments")
	}

	first, ferr := strconv.ParseInt(ctx.Args().Get(0), 10, 64)
	last, lerr := strconv.ParseInt(ctx.Args().Get(1), 10, 64)
	if ferr != nil || lerr != nil {
		return fmt.Errorf("substate-cli replay-mt: error in parsing parameters: block number not an integer")
	}
	if first < 0 || last < 0 {
		return fmt.Errorf("substate-cli replay-mt: error: block number must be greater than 0")
	}
	if first > last {
		return fmt.Errorf("substate-cli replay-mt: error: first block has larger number than last block")
	}

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	taskPool := research.NewSubstateTaskPool("substate-cli replay-SI", replaySITask, uint64(first), uint64(last), ctx)
	initGlobalEnv(ctx, taskPool)
	err = taskPool.Execute()
	return err
}

// replayTask replays a transaction substate based on metamorphic relations
func replaySITask(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool) error {
	var (
		errorstrings   []string
		localUsers     []string
		localContracts []string
		err            error
	)

	// return if toAddr not in inner
	if !containByList(fuzz.GetInnerValueList(),
		strings.ToLower(substate.Message.To.String())) {
		return fmt.Errorf("not inner")
	}

	// rich InputAlloc if richInfoFlag is true
	if taskPool.RichInfo {
		// add previous alloc to InputAlloc & OutputAlloc
		for _, pastBlock := range pastBlocks {
			if pastBlock > block {
				continue
			}
			var keys []int
			pastBlockSubstate := taskPool.DB.GetBlockSubstates(pastBlock)
			for tx, substate := range pastBlockSubstate {
				msgToAddr := substate.Message.To.String()
				if !containByList(fuzz.GetInnerValueList(), msgToAddr) {
					continue
				}
				keys = append(keys, tx)
			}
			sort.Slice(keys, func(i, j int) bool { return keys[i] > keys[j] })
			for _, key := range keys {
				if key >= tx {
					continue
				}
				research.UpdateSubstatePlusInner(
					&(substate.InputAlloc),
					pastBlockSubstate[key].OutputAlloc,
					false,
					false,
					fuzz.GetInnerValueList())
				research.UpdateSubstatePlusInner(
					&(substate.OutputAlloc),
					pastBlockSubstate[key].OutputAlloc,
					false,
					false,
					fuzz.GetInnerValueList())
			}
		}
	}

	for add, acc := range substate.InputAlloc {
		acc0, exist := substate.OutputAlloc[add]
		if exist == false {
			continue
		}
		acc.Balance.Add(tenEther, acc.Balance)
		acc0.Balance.Add(tenEther, acc0.Balance)

		if acc.Code == nil {
			localUsers = append(localUsers, strings.ToLower(add.String()))
			if containByList(fuzz.GetUserValueList(), add.String()) {
				continue
			}
			fuzz.GlobalUserSeed = append(
				fuzz.GlobalUserSeed,
				fuzz.SeedItem{strings.ToLower(add.String()), block})
		} else {
			if !containByList(fuzz.GetInnerValueList(), add.String()) &&
				!containByList(fuzz.GetOuterValueList(), add.String()) {
				fuzz.GlobalOuterSeed = append(
					fuzz.GlobalOuterSeed,
					fuzz.SeedItem{strings.ToLower(add.String()), block})
				localContracts = append(localContracts, strings.ToLower(add.String()))
			} else if !containByList(
				fuzz.ConvertStringSlice2InterfaceSlice(localContracts),
				add.String()) {
				localContracts = append(localContracts, strings.ToLower(add.String()))
			}
		}
	}

	if !taskPool.SkipEnv {
		err = replayWithEnvMR(block, tx, substate, taskPool)
		if err != nil &&
			strings.Index(err.Error(), "inconsistent output") == -1 &&
			strings.Index(err.Error(), "insufficient funds") == -1 {
			errorstrings = append(errorstrings, err.Error())
		}
	}

	if !taskPool.SkipTod {
		err = replayWithTodMR(block, tx, substate, taskPool, localUsers, localContracts)
		if err != nil &&
			strings.Index(err.Error(), "inconsistent output") == -1 &&
			strings.Index(err.Error(), "insufficient funds") == -1 {
			errorstrings = append(errorstrings, err.Error())
		}
	}

	if !taskPool.SkipMani {
		err = replayWithManiMR(block, tx, substate, taskPool, localUsers, localContracts)
		if err != nil &&
			strings.Index(err.Error(), "inconsistent output") == -1 &&
			strings.Index(err.Error(), "insufficient funds") == -1 {
			errorstrings = append(errorstrings, err.Error())
		}
	}

	if !taskPool.SkipHook {
		err = replayWithHook(block, tx, substate, taskPool, localUsers, localContracts)
		if err != nil &&
			strings.Index(err.Error(), "inconsistent output") == -1 &&
			strings.Index(err.Error(), "insufficient funds") == -1 {
			errorstrings = append(errorstrings, err.Error())
		}
	}

	if len(errorstrings) == 0 {
		return nil
	} else {
		log.SetOutput(errorLogFile)
		log.SetPrefix("[ErrorLog]")
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.LUTC)
		log.Printf(strings.Join(errorstrings, "\n"))
		return nil
	}
}

func replayWithEnvMR(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool) error {

	inputAlloc := substate.InputAlloc
	inputMessage := substate.Message
	var (
		oriAlloc research.SubstateAlloc
		mutAlloc research.SubstateAlloc
		err      error
	)

	oriEnv := substate.Env
	mutEnv := &research.SubstateEnv{
		Coinbase:    oriEnv.Coinbase,
		Difficulty:  new(big.Int).Add(oriEnv.Difficulty, big.NewInt(rand.Int63()%100)),
		GasLimit:    oriEnv.GasLimit,
		Number:      oriEnv.Number,
		Timestamp:   oriEnv.Timestamp + rand.Uint64()%100,
		BlockHashes: oriEnv.BlockHashes,
		BaseFee:     new(big.Int).SetUint64(oriEnv.BaseFee.Uint64()),
	}

	if oriAlloc, err = replayRegularMsgs(block, tx, inputAlloc, *oriEnv, inputMessage.AsMessage()); err != nil {
		return err
	}
	if mutAlloc, err = replayRegularMsgs(block, tx, inputAlloc, *mutEnv, inputMessage.AsMessage()); err != nil {
		return err
	}

	if addr, a := oriAlloc.InnerStateEqual(mutAlloc); !a {
		// write bug detailed information
		bugFile := taskPool.DappDir + "output/" + addr + "_" +
			strconv.FormatUint(oriEnv.Number, 10) + ".json"
		bugDetails := &SIbug{
			BugType:          "ENV",
			InputAlloc:       substate.InputAlloc,
			OutputAlloc:      substate.OutputAlloc,
			InputMessage:     *inputMessage,
			AdditMessageFrom: "",
			AdditMessageTo:   "",
			AdditMessageData: "",
			OriAlloc:         oriAlloc,
			MutAlloc:         mutAlloc,
		}
		data, err := json.MarshalIndent(bugDetails, "", " ")
		checkError(err)
		err = ioutil.WriteFile(bugFile, data, 0777)
		checkError(err)
		// write to log file
		log.SetOutput(bugLogFile)
		log.SetPrefix("[SIBugLog]")
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.LUTC)
		log.Printf("alloc differ under ENV in \n%s\nin %d\n", addr, block)
	}
	return nil
}

func replayWithTodMR(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool, localUsers []string, localContracts []string) error {
	// collect original information
	env := substate.Env
	inputAlloc := substate.InputAlloc
	originalMessage := substate.Message

	// generate additional messages
	var (
		addrs []string
		msgs  []string
		rets  []string
		err   error
	)
	if addrs, msgs, rets, err = fuzz.MsgBuilder(
		fuzz.ConvertInterfaceSlice2StringSlice(fuzz.GetInnerValueList()),
		block,
		localUsers,
		localContracts); err != nil {
		return fmt.Errorf("error in generating msgs")
	}

	// replay original & additional messages
	for index, msg := range msgs {
		var (
			tempAlloc     research.SubstateAlloc
			fromAddress   common.Address
			toAddress     common.Address
			tempEnv       research.SubstateEnv
			originalMsg   types.Message
			additionalMsg types.Message
			err           error
			obverseAlloc  research.SubstateAlloc
			reverseAlloc  research.SubstateAlloc
		)
		msgData, _ := hex.DecodeString(msg[2:])

		if len(localUsers) <= 1 {
			Users := fuzz.ConvertInterfaceSlice2StringSlice(fuzz.GetUserValueList())
			fromAddress = common.HexToAddress(Users[int(rand.Uint64()/2)%len(Users)])
		} else {
			fromAddress = common.HexToAddress(localUsers[int(rand.Uint64()/2)%len(localUsers)])
		}
		toAddress = common.HexToAddress(addrs[index])
		// if from address does not exist, generate one
		if fromAccount, exist := inputAlloc[fromAddress]; exist != true {
			fromAccount = &research.SubstateAccount{}
			fromAccount.Balance = new(big.Int).SetUint64(math.MaxUint64)
			inputAlloc[fromAddress] = fromAccount
		}

		// (original, additional)
		tempAlloc = inputAlloc.Copy()
		tempEnv = *env
		originalMsg = types.NewMessage(
			originalMessage.From,
			originalMessage.To,
			tempAlloc[originalMessage.From].Nonce,
			originalMessage.Value,
			originalMessage.Gas,
			originalMessage.GasPrice,
			originalMessage.GasFeeCap,
			originalMessage.GasTipCap,
			originalMessage.Data,
			originalMessage.AccessList,
			false,
		)
		// execute original msg
		if obverseAlloc, err = replayRegularMsgs(block, tx, tempAlloc, tempEnv, originalMsg); err != nil {
			return err
		}
		research.UpdateSubstate(&obverseAlloc, tempAlloc, false, true)

		tempAlloc = obverseAlloc.Copy()
		tempEnv = *env
		additionalMsg = types.NewMessage(
			fromAddress,
			&toAddress,
			tempAlloc[fromAddress].Nonce,
			new(big.Int),
			env.GasLimit-originalMessage.Gas,
			originalMessage.GasPrice,
			originalMessage.GasFeeCap,
			originalMessage.GasTipCap,
			msgData,
			originalMessage.AccessList,
			false,
		)
		// execute additional msg
		if obverseAlloc, err = replayRegularMsgs(block, tx+1, tempAlloc, tempEnv, additionalMsg); err != nil {
			return err
		}
		research.UpdateSubstate(&obverseAlloc, tempAlloc, false, true)

		// (additional, original)
		tempAlloc = inputAlloc.Copy()
		tempEnv = *env
		additionalMsg = types.NewMessage(
			fromAddress,
			&toAddress,
			tempAlloc[fromAddress].Nonce,
			new(big.Int),
			env.GasLimit-originalMessage.Gas,
			originalMessage.GasPrice,
			originalMessage.GasFeeCap,
			originalMessage.GasTipCap,
			msgData,
			originalMessage.AccessList,
			false,
		)
		// execute additional msg
		if reverseAlloc, err = replayRegularMsgs(block, tx, tempAlloc, tempEnv, additionalMsg); err != nil {
			return err
		}
		research.UpdateSubstate(&reverseAlloc, tempAlloc, false, true)

		// additional check if additional msg is useless
		if _, flag := reverseAlloc.AllStateEqual(inputAlloc); flag == true {
			continue
		}

		tempAlloc = reverseAlloc.Copy()
		originalMsg = types.NewMessage(
			originalMessage.From,
			originalMessage.To,
			tempAlloc[originalMessage.From].Nonce,
			originalMessage.Value,
			originalMessage.Gas,
			originalMessage.GasPrice,
			originalMessage.GasFeeCap,
			originalMessage.GasTipCap,
			originalMessage.Data,
			originalMessage.AccessList,
			false,
		)
		// execute original msg
		if reverseAlloc, err = replayRegularMsgs(block, tx+1, tempAlloc, tempEnv, originalMsg); err != nil {
			return err
		}
		research.UpdateSubstate(&reverseAlloc, tempAlloc, false, true)

		if addr, a := obverseAlloc.InnerStateEqual(reverseAlloc); !a {
			// write bug information
			bugFile := taskPool.DappDir + "output/" + addr + "_" +
				strconv.FormatUint(env.Number, 10) + ".json"
			bugDetails := &SIbug{
				BugType:          "TOD",
				InputAlloc:       substate.InputAlloc,
				OutputAlloc:      substate.OutputAlloc,
				InputMessage:     *originalMessage,
				AdditMessageFrom: additionalMsg.From().String(),
				AdditMessageTo:   additionalMsg.To().String(),
				AdditMessageData: rets[index],
				OriAlloc:         obverseAlloc,
				MutAlloc:         reverseAlloc,
			}
			data, err := json.MarshalIndent(bugDetails, "", " ")
			checkError(err)
			err = ioutil.WriteFile(bugFile, data, 0777)
			checkError(err)
			//writh to bug log file
			log.SetOutput(bugLogFile)
			log.SetPrefix("[SIBugLog]")
			log.SetFlags(log.LstdFlags | log.Lshortfile | log.LUTC)
			log.Printf("alloc differ under TOD in \n%s\nin %d\n", addr, block)
		}
	}

	return nil
}

func replayWithManiMR(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool, localUsers []string, localContracts []string) error {
	// collect original information
	env := substate.Env
	inputAlloc := substate.InputAlloc
	originalMessage := substate.Message

	// generate additional messages
	var (
		addrs          []string
		msgs           []string
		rets           []string
		localDappOuter []string
		err            error
	)
	// init target contracts
	for _, localContract := range localContracts {
		if !containByList(
			fuzz.GetInnerValueList(),
			localContract) {
			localDappOuter = append(localDappOuter, localContract)
		}
	}
	if len(localContracts) == 0 {
		localDappOuter = fuzz.ConvertInterfaceSlice2StringSlice(
			fuzz.GetOuterValueList())
	}
	// generate msg
	if addrs, msgs, rets, err = fuzz.MsgBuilder(
		localDappOuter,
		block,
		localUsers,
		localContracts); err != nil {
		return fmt.Errorf("error in generating msgs")
	}

	// replay original & additional messages
	for index, msg := range msgs {
		var (
			tempAlloc     research.SubstateAlloc
			fromAddress   common.Address
			toAddress     common.Address
			tempEnv       research.SubstateEnv
			originalMsg   types.Message
			additionalMsg types.Message
			err           error
			obverseAlloc  research.SubstateAlloc
			reverseAlloc  research.SubstateAlloc
		)
		msgData, _ := hex.DecodeString(msg[2:])
		fromAddress = originalMessage.From
		toAddress = common.HexToAddress(addrs[index])
		// if from address does not exist, generate one (dead code for now)
		if fromAccount, exist := inputAlloc[fromAddress]; exist != true {
			fromAccount = &research.SubstateAccount{}
			fromAccount.Balance = new(big.Int).SetUint64(math.MaxUint64)
			inputAlloc[fromAddress] = fromAccount
		}

		// (original, additional)
		tempAlloc = inputAlloc.Copy()
		tempEnv = *env
		originalMsg = types.NewMessage(
			originalMessage.From,
			originalMessage.To,
			tempAlloc[originalMessage.From].Nonce,
			originalMessage.Value,
			originalMessage.Gas,
			originalMessage.GasPrice,
			originalMessage.GasFeeCap,
			originalMessage.GasTipCap,
			originalMessage.Data,
			originalMessage.AccessList,
			false,
		)
		// execute original msg
		if obverseAlloc, err = replayRegularMsgs(block, tx, tempAlloc, tempEnv, originalMsg); err != nil {
			return err
		}
		research.UpdateSubstate(&obverseAlloc, tempAlloc, false, true)

		tempAlloc = obverseAlloc.Copy()
		tempEnv = *env
		additionalMsg = types.NewMessage(
			fromAddress,
			&toAddress,
			tempAlloc[fromAddress].Nonce,
			new(big.Int),
			env.GasLimit-originalMessage.Gas,
			originalMessage.GasPrice,
			originalMessage.GasFeeCap,
			originalMessage.GasTipCap,
			msgData,
			originalMessage.AccessList,
			false,
		)
		// execute additional msg
		if obverseAlloc, err = replayRegularMsgs(block, tx+1, tempAlloc, tempEnv, additionalMsg); err != nil {
			return err
		}
		research.UpdateSubstate(&obverseAlloc, tempAlloc, false, true)

		// (additional, original)
		tempAlloc = inputAlloc.Copy()
		tempEnv = *env
		additionalMsg = types.NewMessage(
			fromAddress,
			&toAddress,
			tempAlloc[fromAddress].Nonce,
			new(big.Int),
			env.GasLimit-originalMessage.Gas,
			originalMessage.GasPrice,
			originalMessage.GasFeeCap,
			originalMessage.GasTipCap,
			msgData,
			originalMessage.AccessList,
			false,
		)
		if reverseAlloc, err = replayRegularMsgs(block, tx, tempAlloc, tempEnv, additionalMsg); err != nil {
			return err
		}
		research.UpdateSubstate(&reverseAlloc, tempAlloc, false, true)

		// additional check if additional msg is useless
		if _, flag := reverseAlloc.AllStateEqual(inputAlloc); flag == true {
			continue
		}

		tempAlloc = reverseAlloc.Copy()
		originalMsg = types.NewMessage(
			originalMessage.From,
			originalMessage.To,
			tempAlloc[originalMessage.From].Nonce,
			originalMessage.Value,
			originalMessage.Gas,
			originalMessage.GasPrice,
			originalMessage.GasFeeCap,
			originalMessage.GasTipCap,
			originalMessage.Data,
			originalMessage.AccessList,
			false,
		)
		if reverseAlloc, err = replayRegularMsgs(block, tx+1, tempAlloc, tempEnv, originalMsg); err != nil {
			return err
		}
		research.UpdateSubstate(&reverseAlloc, tempAlloc, false, true)

		if addr, a := obverseAlloc.InnerStateEqual(reverseAlloc); !a {
			// write bug information
			bugFile := taskPool.DappDir + "output/" + addr + "_" +
				strconv.FormatUint(env.Number, 10) + ".json"
			bugDetails := &SIbug{
				BugType:          "MANI",
				InputAlloc:       substate.InputAlloc,
				OutputAlloc:      substate.OutputAlloc,
				InputMessage:     *originalMessage,
				AdditMessageFrom: additionalMsg.From().String(),
				AdditMessageTo:   additionalMsg.To().String(),
				AdditMessageData: rets[index],
				OriAlloc:         obverseAlloc,
				MutAlloc:         reverseAlloc,
			}
			data, err := json.MarshalIndent(bugDetails, "", " ")
			checkError(err)
			err = ioutil.WriteFile(bugFile, data, 0777)
			checkError(err)
			//writh to log file
			log.SetOutput(bugLogFile)
			log.SetPrefix("[SIBugLog]")
			log.SetFlags(log.LstdFlags | log.Lshortfile | log.LUTC)
			log.Printf("alloc differ under MANI in \n%s\nin %d\n", addr, block)
		}
	}

	return nil
}

func replayWithHook(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool, localUsers []string, localContracts []string) error {

	inputAlloc := substate.InputAlloc
	inputEnv := substate.Env
	inputMessage := substate.Message

	var (
		vmConfig    vm.Config
		chainConfig *params.ChainConfig
		getTracerFn func(txIndex int, txHash common.Hash) (tracer vm.EVMLogger, err error)
	)
	vmConfig = vm.Config{}
	chainConfig = &params.ChainConfig{}
	*chainConfig = *params.MainnetChainConfig
	// disable DAOForkSupport, otherwise account states will be overwritten
	chainConfig.DAOForkSupport = false
	getTracerFn = func(txIndex int, txHash common.Hash) (tracer vm.EVMLogger, err error) {
		return nil, nil
	}
	var hashError error
	getHash := func(num uint64) common.Hash {
		if inputEnv.BlockHashes == nil {
			hashError = fmt.Errorf("getHash(%d) invoked, no blockhashes provided", num)
			return common.Hash{}
		}
		h, ok := inputEnv.BlockHashes[num]
		if !ok {
			hashError = fmt.Errorf("getHash(%d) invoked, blockhash for that block not provided", num)
		}
		return h
	}

	var (
		targetedAddress []string
		msgs            []string
		rets            []string
		err             error
	)
	targetedAddress, msgs, rets, err = fuzz.MsgBuilder(
		fuzz.ConvertInterfaceSlice2StringSlice(fuzz.GetInnerValueList()),
		block,
		localUsers,
		localContracts)
	if err != nil {
		return fmt.Errorf("error in generating msgs")
	}

	for index, msg := range msgs {
		var (
			hookAlloc research.SubstateAlloc
			outAlloc  research.SubstateAlloc
		)
		// Apply message without hook
		tempAlloc := inputAlloc.Copy()
		tempEnv := *inputEnv
		originalMsg := types.NewMessage(
			inputMessage.From,
			inputMessage.To,
			tempAlloc[inputMessage.From].Nonce,
			inputMessage.Value,
			inputMessage.Gas,
			inputMessage.GasPrice,
			inputMessage.GasFeeCap,
			inputMessage.GasTipCap,
			inputMessage.Data,
			inputMessage.AccessList,
			false,
		)
		if outAlloc, err = replayRegularMsgs(block, tx, tempAlloc, tempEnv, originalMsg); err != nil {
			return err
		}
		research.UpdateSubstate(&outAlloc, tempAlloc, false, true)

		tempAlloc = outAlloc.Copy()
		tempEnv = *inputEnv
		toAddress := common.HexToAddress(targetedAddress[index])
		data, _ := hex.DecodeString(msg[2:])
		additionalMsg := types.NewMessage(
			inputMessage.From,
			&toAddress,
			tempAlloc[inputMessage.From].Nonce,
			new(big.Int),
			tempEnv.GasLimit-inputMessage.Gas,
			inputMessage.GasPrice,
			inputMessage.GasFeeCap,
			inputMessage.GasTipCap,
			data,
			inputMessage.AccessList,
			false,
		)
		if outAlloc, err = replayRegularMsgs(block, tx+1, tempAlloc, tempEnv, additionalMsg); err != nil {
			return err
		}
		research.UpdateSubstate(&outAlloc, tempAlloc, false, true)

		// Apply Message with hook
		var (
			statedb = MakeOffTheChainStateDB(inputAlloc)
			gaspool = new(core.GasPool)
			txHash  = common.Hash{0x02}
			txIndex = tx
		)
		gaspool.AddGas(inputEnv.GasLimit)
		blockCtx := vm.BlockContext{
			CanTransfer: core.CanTransfer,
			Transfer:    core.Transfer,
			Coinbase:    inputEnv.Coinbase,
			BlockNumber: new(big.Int).SetUint64(inputEnv.Number),
			Time:        new(big.Int).SetUint64(inputEnv.Timestamp),
			Difficulty:  inputEnv.Difficulty,
			GasLimit:    inputEnv.GasLimit,
			GetHash:     getHash,
		}
		// If currentBaseFee is defined, add it to the vmContext.
		if inputEnv.BaseFee != nil {
			blockCtx.BaseFee = new(big.Int).Set(inputEnv.BaseFee)
		}
		temp := inputMessage.Gas
		inputMessage.Gas = inputEnv.GasLimit
		inputMsg := inputMessage.AsMessage()
		inputMessage.Gas = temp
		tracer, err := getTracerFn(txIndex, txHash)
		if err != nil {
			return err
		}
		vmConfig.Tracer = tracer
		vmConfig.Debug = (tracer != nil)
		statedb.Prepare(txHash, txIndex)
		txCtx := vm.TxContext{
			GasPrice: inputMsg.GasPrice(),
			Origin:   inputMsg.From(),
		}
		evm := vm.NewEVM(blockCtx, txCtx, statedb, chainConfig, vmConfig)
		snapshot := statedb.Snapshot()
		msg = msg[2:]
		hookResult, err := core.ApplyMessageWithHook(
			evm,
			inputMsg,
			gaspool,
			fuzz.ConvertInterfaceSlice2StringSlice(fuzz.GetInnerValueList()),
			targetedAddress[index],
			msg)

		if err != nil {
			statedb.RevertToSnapshot(snapshot)
			return err
		} else if hashError != nil {
			return hashError
		}
		if chainConfig.IsByzantium(blockCtx.BlockNumber) {
			statedb.Finalise(true)
		} else {
			statedb.IntermediateRoot(chainConfig.IsEIP158(blockCtx.BlockNumber))
		}
		hookAlloc = statedb.ResearchPostAlloc

		if addr, a := outAlloc.InnerStateEqual(hookAlloc); !a && hookResult.Err == nil {
			// write bug information
			bugDir := taskPool.DappDir + "output/" + addr + "_" +
				strconv.FormatUint(inputEnv.Number, 10) + ".json"
			bugDetails := &SIbug{
				BugType:          "HOOK",
				InputAlloc:       substate.InputAlloc,
				OutputAlloc:      substate.OutputAlloc,
				InputMessage:     *inputMessage,
				AdditMessageFrom: additionalMsg.From().String(),
				AdditMessageTo:   additionalMsg.To().String(),
				AdditMessageData: rets[index],
				OriAlloc:         outAlloc,
				MutAlloc:         hookAlloc,
			}
			data, err := json.MarshalIndent(bugDetails, "", " ")
			checkError(err)
			err = ioutil.WriteFile(bugDir, data, 0777)
			checkError(err)
			//writh to log file
			log.SetOutput(bugLogFile)
			log.SetPrefix("[SIBugLog]")
			log.SetFlags(log.LstdFlags | log.Lshortfile | log.LUTC)
			log.Printf("alloc differ under HOOK in \n%s\nin %d\n", addr, block)
		}
	}

	return nil
}

func replayRegularMsgs(block uint64, tx int, inputAlloc research.SubstateAlloc, inputEnv research.SubstateEnv, message types.Message) (research.SubstateAlloc, error) {
	//Set up Executing Environment
	var (
		vmConfig    vm.Config
		chainConfig *params.ChainConfig
		getTracerFn func(txIndex int, txHash common.Hash) (tracer vm.EVMLogger, err error)
	)
	vmConfig = vm.Config{}
	chainConfig = &params.ChainConfig{}
	*chainConfig = *params.MainnetChainConfig
	// disable DAOForkSupport, otherwise account states will be overwritten
	chainConfig.DAOForkSupport = false
	getTracerFn = func(txIndex int, txHash common.Hash) (tracer vm.EVMLogger, err error) {
		return nil, nil
	}
	var hashError error
	getHash := func(num uint64) common.Hash {
		if inputEnv.BlockHashes == nil {
			hashError = fmt.Errorf("getHash(%d) invoked, no blockhashes provided", num)
			return common.Hash{}
		}
		h, ok := inputEnv.BlockHashes[num]
		if !ok {
			hashError = fmt.Errorf("getHash(%d) invoked, blockhash for that block not provided", num)
		}
		return h
	}

	// Apply Message
	var (
		statedb = MakeOffTheChainStateDB(inputAlloc)
		gaspool = new(core.GasPool)
		txHash  = common.Hash{0x02}
		txIndex = tx
	)
	gaspool.AddGas(inputEnv.GasLimit)
	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		Coinbase:    inputEnv.Coinbase,
		BlockNumber: new(big.Int).SetUint64(inputEnv.Number),
		Time:        new(big.Int).SetUint64(inputEnv.Timestamp),
		Difficulty:  inputEnv.Difficulty,
		GasLimit:    inputEnv.GasLimit,
		GetHash:     getHash,
	}
	// If currentBaseFee is defined, add it to the vmContext.
	if inputEnv.BaseFee != nil {
		blockCtx.BaseFee = new(big.Int).Set(inputEnv.BaseFee)
	}
	tracer, err := getTracerFn(txIndex, txHash)
	if err != nil {
		return nil, err
	}
	vmConfig.Tracer = tracer
	vmConfig.Debug = (tracer != nil)
	statedb.Prepare(txHash, txIndex)
	txCtx := vm.TxContext{
		GasPrice: message.GasPrice(),
		Origin:   message.From(),
	}
	evm := vm.NewEVM(blockCtx, txCtx, statedb, chainConfig, vmConfig)
	snapshot := statedb.Snapshot()
	_, err = core.ApplyMessage(evm, message, gaspool)

	if err != nil {
		statedb.RevertToSnapshot(snapshot)
		return nil, err
	}

	if hashError != nil {
		return nil, hashError
	}

	if chainConfig.IsByzantium(blockCtx.BlockNumber) {
		statedb.Finalise(true)
	} else {
		statedb.IntermediateRoot(chainConfig.IsEIP158(blockCtx.BlockNumber))
	}

	evmAlloc := statedb.ResearchPostAlloc

	return evmAlloc, nil
}

// parsing address varients from addressdir
func initGlobalEnv(ctx *cli.Context, taskPool *research.SubstateTaskPool) error {
	// set up global variables (i.e. files)
	var err error
	errorLogFile, err = os.OpenFile(taskPool.DappDir+"/errorLog.txt",
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0766)
	checkError(err)
	bugLogFile, err = os.OpenFile(taskPool.DappDir+"/SIbugLog.txt",
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0766)
	checkError(err)
	// set up a global variable
	tenEther, _ = new(big.Int).SetString("10000000000000000000", 0)
	// readline from address.txt
	addressDir := taskPool.DappDir + "/address.txt"
	fmt.Printf("record-replay: --addressdir=%s\n", addressDir)

	addrFile, err := os.OpenFile(addressDir, os.O_RDWR, 0444)
	checkError(err)
	defer addrFile.Close()
	_, err = addrFile.Stat()
	checkError(err)

	buf := bufio.NewReader(addrFile)
	for {
		line, err := buf.ReadString('\n')
		line = strings.TrimSpace(line)
		if err == io.EOF {
			break
		} else if err != nil && err != io.EOF {
			return err
		}
		fuzz.GlobalInnerSeed = append(
			fuzz.GlobalInnerSeed,
			fuzz.SeedItem{strings.ToLower(line), math.MaxUint32})
	}

	fuzz.GlobalABIPath = taskPool.DappDir + "/abi/"

	// read from richInfo
	if taskPool.RichInfo {
		blockInfo := taskPool.DappDir + "/blockInfo.txt"
		fmt.Printf("record-replay: --blockInfoDir=%s\n", blockInfo)
		infoFile, err := os.OpenFile(blockInfo, os.O_RDWR, 0444)
		checkError(err)
		defer infoFile.Close()
		_, err = infoFile.Stat()
		checkError(err)

		buf := bufio.NewReader(infoFile)
		for {
			line, err := buf.ReadString('\n')
			line = strings.TrimSpace(line)
			if err == io.EOF {
				break
			} else if err != nil && err != io.EOF {
				return err
			}
			block, err := strconv.ParseUint(line, 10, 64)
			checkError(err)
			pastBlocks = append(pastBlocks, block)
		}

		pastBlocks = removeRepByMap(pastBlocks)
		sort.Slice(pastBlocks, func(i, j int) bool { return pastBlocks[i] > pastBlocks[j] })
		// add previous msg data to global seed
		for _, pastBlock := range pastBlocks {
			pastBlockSubstate := taskPool.DB.GetBlockSubstates(pastBlock)
			for _, substate := range pastBlockSubstate {
				if substate.Message.To == nil ||
					!containByList(
						fuzz.GetInnerValueList(),
						strings.ToLower(substate.Message.To.String())) {
					continue
				}
				fuzz.UpdateGlobalSeeds(substate.Message.Data, pastBlock)
			}
		}
	}
	return nil
}

func containByList(list []interface{}, item interface{}) bool {
	for _, listItem := range list {
		if item == listItem {
			return true
		}
	}
	return false
}

func removeRepByMap(List []uint64) []uint64 {
	result := []uint64{}
	tempMap := map[uint64]byte{}
	for _, element := range List {
		l := len(tempMap)
		tempMap[element] = 0
		if len(tempMap) != l {
			result = append(result, element)
		}
	}
	return result
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}
