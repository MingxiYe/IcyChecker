package fuzz

import (
	"log"
	"math/rand"
	"os"
	"time"
)

type FuzzerRand struct {
	seed       int
	lastChoice int
	r          *rand.Rand
}

func NewFuzzerRand() *FuzzerRand {
	return &FuzzerRand{
		seed:       time.Now().Nanosecond(),
		lastChoice: -1,
		r:          rand.New(rand.NewSource((int64)(time.Now().Nanosecond())))}
}

func (this *FuzzerRand) RandomSelect(bids []interface{}) (ret interface{}, err error) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			printCallStackIfError()
			ret = nil
			err = ERR_RANDOM_SELECT_FAILED
		}
	}()

	if len(bids) == 0 {
		log.Fatalf("Random_select: bids' length cannot be zero")
		printCallStackIfError()
		os.Exit(-1)
		return nil, ERR_ZERO_SIZED_SLICE
	}

	var (
		timeLimit = 10
		choice    = this.r.Intn(len(bids))
	)

	for ; this.lastChoice == choice && timeLimit > 0; choice = this.r.Intn(len(bids)) {
		timeLimit -= 1
	}

	if timeLimit > 0 {
		this.lastChoice = choice
		return bids[choice], nil
	} else {
		this.lastChoice = -1
		return nil, ERR_OVER_RANDOM_LIMIT
	}
}

var (
	intRand     = NewFuzzerRand()
	uintRand    = NewFuzzerRand()
	byteRand    = NewFuzzerRand()
	bytesRand   = NewFuzzerRand()
	stringRand  = NewFuzzerRand()
	addressRand = NewFuzzerRand()
	boolRand    = NewFuzzerRand()
)
