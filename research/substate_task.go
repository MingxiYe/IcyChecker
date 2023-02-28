package research

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	cli "gopkg.in/urfave/cli.v1"
)

var (
	WorkersFlag = cli.IntFlag{
		Name:  "workers",
		Usage: "Number of worker threads that execute in parallel",
		Value: 4,
	}
	SkipTransferTxsFlag = cli.BoolFlag{
		Name:  "skip-transfer-txs",
		Usage: "Skip executing transactions that only transfer ETH",
	}
	SkipCallTxsFlag = cli.BoolFlag{
		Name:  "skip-call-txs",
		Usage: "Skip executing CALL transactions to accounts with contract bytecode",
	}
	SkipCreateTxsFlag = cli.BoolFlag{
		Name:  "skip-create-txs",
		Usage: "Skip executing CREATE transactions",
	}
	SkipEnvFlag = cli.BoolFlag{
		Name:  "skip-env",
		Usage: "Skip ENV MR",
	}
	SkipTodFlag = cli.BoolFlag{
		Name:  "skip-tod",
		Usage: "Skip TOD MR",
	}
	SkipManiFlag = cli.BoolFlag{
		Name:  "skip-mani",
		Usage: "Skip MANI MR",
	}
	SkipHookFlag = cli.BoolFlag{
		Name:  "skip-hook",
		Usage: "Skip HOOK MR",
	}
	RichInfoFlag = cli.BoolFlag{
		Name:  "rich-info",
		Usage: "Rich Substate",
	}
	DappDirFlag = cli.StringFlag{
		Name:  "dappDir",
		Usage: "the path for targeted dapp data",
	}
)

type SubstateTaskFunc func(block uint64, tx int, substate *Substate, taskPool *SubstateTaskPool) error

type SubstateTaskPool struct {
	Name     string
	TaskFunc SubstateTaskFunc

	First uint64
	Last  uint64

	Workers  int
	RichInfo bool
	SkipEnv  bool
	SkipTod  bool
	SkipMani bool
	SkipHook bool

	DappDir string

	Ctx *cli.Context // CLI context required to read additional flags

	DB *SubstateDB
}

func NewSubstateTaskPool(name string, taskFunc SubstateTaskFunc, first, last uint64, ctx *cli.Context) *SubstateTaskPool {
	return &SubstateTaskPool{
		Name:     name,
		TaskFunc: taskFunc,

		First: first,
		Last:  last,

		Workers:  ctx.Int(WorkersFlag.Name),
		SkipEnv:  ctx.Bool(SkipEnvFlag.Name),
		SkipTod:  ctx.Bool(SkipTodFlag.Name),
		SkipMani: ctx.Bool(SkipManiFlag.Name),
		SkipHook: ctx.Bool(SkipHookFlag.Name),
		RichInfo: ctx.Bool(RichInfoFlag.Name),

		DappDir: ctx.String(DappDirFlag.Name),

		Ctx: ctx,

		DB: staticSubstateDB,
	}
}

// ExecuteBlock function iterates on substates of a given block call TaskFunc
func (pool *SubstateTaskPool) ExecuteBlock(block uint64) (numTx int64, err error) {
	for tx, substate := range pool.DB.GetBlockSubstates(block) {
		alloc := substate.InputAlloc
		msg := substate.Message

		to := msg.To
		if to != nil {
			// skip regular transactions (ETH transfer)
			if account, exist := alloc[*to]; !exist || len(account.Code) == 0 {
				continue
			}
		} else {
			// skip CREATE transactions
			continue
		}

		err = pool.TaskFunc(block, tx, substate, pool)
		if err != nil && err.Error() == "not inner" {
			numTx--
		} else if err != nil {
			return numTx, fmt.Errorf("%s: %v_%v: %v", pool.Name, block, tx, err)
		}

		numTx++
	}

	return numTx, nil
}

// Execute function spawns worker goroutines and schedule tasks.
func (pool *SubstateTaskPool) Execute() error {
	start := time.Now()

	var totalNumBlock, totalNumTx int64
	defer func() {
		duration := time.Since(start) + 1*time.Nanosecond
		sec := duration.Seconds()

		nb, nt := atomic.LoadInt64(&totalNumBlock), atomic.LoadInt64(&totalNumTx)
		blkPerSec := float64(nb) / sec
		txPerSec := float64(nt) / sec
		fmt.Printf("%s: block range = %v %v\n", pool.Name, pool.First, pool.Last)
		fmt.Printf("%s: total #block = %v\n", pool.Name, nb)
		fmt.Printf("%s: total #tx    = %v\n", pool.Name, nt)
		fmt.Printf("%s: %.2f blk/s, %.2f tx/s\n", pool.Name, blkPerSec, txPerSec)
		fmt.Printf("%s done in %v\n", pool.Name, duration.Round(1*time.Millisecond))
	}()

	// numProcs = numWorker + work producer (1) + main thread (1)
	numProcs := pool.Workers + 2
	if goMaxProcs := runtime.GOMAXPROCS(0); goMaxProcs < numProcs {
		runtime.GOMAXPROCS(numProcs)
	}

	fmt.Printf("%s: block range = %v %v\n", pool.Name, pool.First, pool.Last)
	fmt.Printf("%s: #CPU = %v, #worker = %v\n", pool.Name, runtime.NumCPU(), pool.Workers)

	workChan := make(chan uint64, pool.Workers*10)
	doneChan := make(chan interface{}, pool.Workers*10)
	stopChan := make(chan struct{}, pool.Workers)
	wg := sync.WaitGroup{}
	defer func() {
		// stop all workers
		for i := 0; i < pool.Workers; i++ {
			stopChan <- struct{}{}
		}
		// stop work producer (1)
		stopChan <- struct{}{}

		wg.Wait()
		close(workChan)
		close(doneChan)
	}()
	// dynamically schedule one block per worker
	for i := 0; i < pool.Workers; i++ {
		wg.Add(1)
		// worker goroutine
		go func() {
			defer wg.Done()

			for {
				select {

				case block := <-workChan:
					nt, err := pool.ExecuteBlock(block)
					atomic.AddInt64(&totalNumTx, nt)
					atomic.AddInt64(&totalNumBlock, 1)
					if err != nil {
						doneChan <- err
					} else {
						doneChan <- block
					}

				case <-stopChan:
					return

				}
			}
		}()
	}

	// wait until all workers finish all tasks
	wg.Add(1)
	go func() {
		defer wg.Done()

		for block := pool.First; block <= pool.Last; block++ {
			select {

			case workChan <- block:
				continue

			case <-stopChan:
				return

			}
		}
	}()

	// Count finished blocks in order and report execution speed
	var lastSec float64
	var lastNumBlock, lastNumTx int64
	waitMap := make(map[uint64]struct{})
	for block := pool.First; block <= pool.Last; {

		// Count finshed blocks from waitMap in order
		if _, ok := waitMap[block]; ok {
			delete(waitMap, block)

			block++
			continue
		}

		duration := time.Since(start) + 1*time.Nanosecond
		sec := duration.Seconds()
		if block == pool.Last ||
			(block%10000 == 0 && sec > lastSec+5) ||
			(block%1000 == 0 && sec > lastSec+10) ||
			(block%100 == 0 && sec > lastSec+20) ||
			(block%10 == 0 && sec > lastSec+40) ||
			(sec > lastSec+60) {
			nb, nt := atomic.LoadInt64(&totalNumBlock), atomic.LoadInt64(&totalNumTx)
			blkPerSec := float64(nb-lastNumBlock) / (sec - lastSec)
			txPerSec := float64(nt-lastNumTx) / (sec - lastSec)
			fmt.Printf("%s: elapsed time: %v, number = %v\n", pool.Name, duration.Round(1*time.Millisecond), block)
			fmt.Printf("%s: %.2f blk/s, %.2f tx/s\n", pool.Name, blkPerSec, txPerSec)

			lastSec, lastNumBlock, lastNumTx = sec, nb, nt
		}

		data := <-doneChan
		switch t := data.(type) {

		case uint64:
			waitMap[data.(uint64)] = struct{}{}

		case error:
			err := data.(error)
			return err

		default:
			panic(fmt.Errorf("%s: unknown type %T value from doneChan", pool.Name, t))

		}
	}

	return nil
}

// // obtain a rich substate for taskPool
// func (pool *SubstateTaskPool) InitRichSubstates(_addr2blocks map[string]([]uint64)) error {
// 	Addr2Block = make(map[string][]uint64)
// 	for _addr, _block := range _addr2blocks {
// 		Addr2Block[_addr] = _block
// 	}

// 	// RichSubstate = make(map[uint64]map[int]*Substate)
// 	// latestAlloc := make(SubstateAlloc)
// 	// for _, block := range blocks {
// 	// 	RichSubstate[block] = make(map[int]*Substate)

// 	// 	for tx, substate := range pool.DB.GetBlockSubstates(block) {
// 	// 		outAlloc := substate.OutputAlloc

// 	// 		// update inAlloc & outAlloc + latestAlloc to richSubstate
// 	// 		if block > pool.First {
// 	// 			RichSubstate[block][tx] = NewSubstate(substate.InputAlloc,
// 	// 				substate.OutputAlloc,
// 	// 				substate.Env,
// 	// 				substate.Message,
// 	// 				substate.Result)
// 	// 			UpdateSubstate(&(RichSubstate[block][tx].InputAlloc), latestAlloc, false)
// 	// 			UpdateSubstate(&(RichSubstate[block][tx].OutputAlloc), latestAlloc, false)
// 	// 		}
// 	// 		// update outAlloc to latestAlloc
// 	// 		UpdateSubstate(&latestAlloc, outAlloc, true)
// 	// 	}
// 	// }

// 	return nil
// }

// update substate by alloc, replace := replace or not
func UpdateSubstate(allocPointer *SubstateAlloc, alloc SubstateAlloc, replace bool, add bool) {
	for addr, account := range alloc {
		if _, exist := (*allocPointer)[addr]; exist == false {
			if add {
				(*allocPointer)[addr] = account
			}
		} else {
			for hash, storage := range account.Storage {
				if _, exist := (*allocPointer)[addr].Storage[hash]; exist == false {
					(*allocPointer)[addr].Storage[hash] = storage
				} else if replace {
					(*allocPointer)[addr].Storage[hash] = storage
				}
			}
		}
	}
}

func UpdateSubstatePlusInner(allocPointer *SubstateAlloc, alloc SubstateAlloc, replace bool, add bool, dappInner []interface{}) {
	for addr, account := range alloc {
		if _, exist := (*allocPointer)[addr]; exist == false {
			if containByList(dappInner, addr) || add {
				(*allocPointer)[addr] = account
			}
		} else {
			for hash, storage := range account.Storage {
				if _, exist := (*allocPointer)[addr].Storage[hash]; exist == false {
					(*allocPointer)[addr].Storage[hash] = storage
				} else if replace {
					(*allocPointer)[addr].Storage[hash] = storage
				}
			}
		}
	}
}
