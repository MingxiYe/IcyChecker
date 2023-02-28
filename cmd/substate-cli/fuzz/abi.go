package fuzz

import (
	"encoding/json"
	"fmt"
)

type Element struct {
	Name string        `json:"name,omitempty"`
	Type string        `json:"type"`
	Out  []interface{} `json:"out,omitempty"`
}
type IOput []Element

func newIOput(jsondata []byte) (*IOput, error) {
	var ioput = new(IOput)
	if err := json.Unmarshal(jsondata, ioput); err != nil {
		return nil, JSON_UNMARSHAL_ERROR(err)
	}
	return ioput, nil
}

func (ioput *IOput) String() string {
	buf, _ := json.Marshal(ioput)
	return string(buf)
}

// we only fuzz input
// return one generated input each time
// through random.go, the generated input is different with the previous one
// In cartesianProductOne, only the first generated inputs are used
func (input *IOput) fuzz(timestamp uint64, localUsers []string, localContracts []string) (interface{}, error) {
	for i, _ := range *input {
		elem := &(*input)[i]
		out, err := fuzz(elem.Type, timestamp, localUsers, localContracts)
		if err != nil {
			return nil, err
		}
		if out == nil {
			return nil, fmt.Errorf("nil out")
		}
		elem.Out = out
	}

	var outs = make([][]interface{}, 0, 0)
	for _, elem := range *input {
		outs = append(outs, elem.Out)
	}

	val := cartesianProductOne(outs)
	return val, nil
}

type Function struct {
	Name            string `json:"name,omitempty"`
	Type            string `json:"type"`
	Inputs          IOput  `json:"inputs,omitempty"`
	Outputs         IOput  `json:"outputs,omitempty"`
	Payable         bool   `json:"payable"`
	Statemutability string `json:"stateMutability,omitempty`
	Constant        bool   `json:"constant,omitempty"`
}

func (fun *Function) Sig() string {
	var elems = ([]Element)(fun.Inputs)
	sig := fun.Name + "("
	for i, elem := range elems {
		if i == 0 {
			sig += elem.Type
		} else {
			sig += "," + elem.Type
		}
	}
	sig = sig + ")"
	return sig
}

func (fun *Function) Values() []interface{} {
	var elems = ([]Element)(fun.Inputs)
	var outs = make([][]interface{}, 0, 0)

	for _, elem := range elems {
		outs = append(outs, elem.Out)
	}

	var vals = make([]interface{}, 0, 0)
	if len(outs) == 0 {
		return nil
	} else {
		for _, v := range outs[0] {
			vals = append(vals, stringify(v))
		}
		for i := 1; i < len(outs); i++ {
			if i > 3 && len(outs[i]) > 2 {
				c := randintOne(len(outs[i]), 0)
				outs[i][0] = outs[i][c]
				c = randintOne(len(outs[i]), 0)
				outs[i][1] = outs[i][c]
				outs[i] = outs[i][:2]
			}
			vals = product(vals, outs[i])
		}
	}
	return vals
}

type ABI []*Function

func newAbi(jsondata []byte) (*ABI, error) {
	var abi = new(ABI)
	if err := json.Unmarshal(jsondata, abi); err != nil {
		return nil, JSON_UNMARSHAL_ERROR(err)
	}
	return abi, nil
}

func (abi *ABI) String() string {
	buf, _ := json.Marshal(abi)
	return string(buf)
}
