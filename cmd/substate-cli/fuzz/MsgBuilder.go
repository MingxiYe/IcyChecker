// this package is used to generate function call based global seed
//
// this is the entry of this package, the main function is MsgBuilder
package fuzz

import (
	"log"

	abi_gen "github.com/ethereum/go-ethereum/cmd/substate-cli/abi"
)

/*
 * generate function calls for each targetedContracts
 * local variables (related to the transaction being fuzzed):
 * timestamp, localUsers, localContracts
 */
func MsgBuilder(targetedContracts []string, timestamp uint64, localUsers []string, localContracts []string) ([]string, []string, []string, error) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			printCallStackIfError()
		}
	}()

	var (
		addressResults []string
		msgResults     []string
		msgStrings     []string
		data           []byte
		abi            *ABI
		err            error
	)

	for i := 0; i < len(targetedContracts); i++ {
		path := GlobalABIPath + targetedContracts[i] + ".json"
		if data, err = readFile(path); err != nil {
			continue
		}
		if abi, err = newAbi(data); err != nil {
			continue
		}

		for _, fun := range ([]*Function)(*abi) {
			if fun.Type != "function" ||
				fun.Constant == true ||
				fun.Statemutability == "pure" ||
				fun.Statemutability == "view" {
				continue
			}

			for j := 0; j < RAND_CASE_SCALE; j++ {
				if len(fun.Inputs) <= 0 {
					if hex_str, suberr := abi_gen.Parse_GenMsg(fun.Sig()); suberr == nil {
						addressResults = append(addressResults, targetedContracts[i])
						msgResults = append(msgResults, hex_str)
						msgStrings = append(msgStrings, fun.Sig())
					}
					break
				}
				if ret, err := fun.Inputs.fuzz(timestamp, localUsers, localContracts); err == nil {
					temp := fun.Sig() + ":[" + ret.(string) + "]"
					if hex_str, suberr := abi_gen.Parse_GenMsg(temp); suberr == nil {
						addressResults = append(addressResults, targetedContracts[i])
						msgResults = append(msgResults, hex_str)
						msgStrings = append(msgStrings, temp)
					}
				} else {
					addressResults = append(addressResults, targetedContracts[i])
					msgResults = append(msgResults, "0xcaffee")
					msgStrings = append(msgStrings, "0xcaffee")
				}
			}
		}
	}

	return addressResults, msgResults, msgStrings, nil
}

/*
 * generate function calls for each targeted contracts
 * given stroage index that are expected to be interfered
 */
func MsgBuilder2(targetedContract2Function map[string][]string, timestamp uint64, localUsers []string, localContracts []string) ([]string, []string, []string, error) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			printCallStackIfError()
		}
	}()

	var (
		addressResults []string
		msgResults     []string
		msgStrings     []string
		data           []byte
		abi            *ABI
		err            error
	)

	for contract, signatureList := range targetedContract2Function {
		path := GlobalABIPath + contract + ".json"
		if data, err = readFile(path); err != nil {
			continue
		}
		if abi, err = newAbi(data); err != nil {
			continue
		}

		for _, fun := range ([]*Function)(*abi) {
			if fun.Type != "function" ||
				fun.Constant == true ||
				!containByList(signatureList, fun.Sig()) ||
				fun.Statemutability == "pure" ||
				fun.Statemutability == "view" {
				continue
			}

			for j := 0; j < RAND_CASE_SCALE; j++ {
				if len(fun.Inputs) <= 0 {
					if hex_str, suberr := abi_gen.Parse_GenMsg(fun.Sig()); suberr == nil {
						addressResults = append(addressResults, contract)
						msgResults = append(msgResults, hex_str)
						msgStrings = append(msgStrings, fun.Sig())
					}
					break
				}
				if ret, err := fun.Inputs.fuzz(timestamp, localUsers, localContracts); err == nil {
					temp := fun.Sig() + ":[" + ret.(string) + "]"
					if hex_str, suberr := abi_gen.Parse_GenMsg(temp); suberr == nil {
						addressResults = append(addressResults, contract)
						msgResults = append(msgResults, hex_str)
						msgStrings = append(msgStrings, temp)
					}
				} else {
					addressResults = append(addressResults, contract)
					msgResults = append(msgResults, "0xcaffee")
					msgStrings = append(msgStrings, "0xcaffee")
				}
			}
			break
		}
	}

	return addressResults, msgResults, msgStrings, nil
}

func containByList(list []string, item string) bool {
	for _, listItem := range list {
		if item == listItem {
			return true
		}
	}
	return false
}
