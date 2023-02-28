package fuzz

import (
	"math/big"
	"strings"
)

var (
	IntMax = map[int]string{
		1:  "0x7f",
		2:  "0x7fff",
		3:  "0x7fffff",
		4:  "0x7fffffff",
		5:  "0x7fffffffff",
		6:  "0x7fffffffffff",
		7:  "0x7fffffffffffff",
		8:  "0x7fffffffffffffff",
		9:  "0x7fffffffffffffffff",
		10: "0x7fffffffffffffffffff",
		11: "0x7fffffffffffffffffffff",
		12: "0x7fffffffffffffffffffffff",
		13: "0x7fffffffffffffffffffffffff",
		14: "0x7fffffffffffffffffffffffffff",
		15: "0x7fffffffffffffffffffffffffffff",
		16: "0x7fffffffffffffffffffffffffffffff",
		17: "0x7fffffffffffffffffffffffffffffffff",
		18: "0x7fffffffffffffffffffffffffffffffffff",
		19: "0x7fffffffffffffffffffffffffffffffffffff",
		20: "0x7fffffffffffffffffffffffffffffffffffffff",
		21: "0x7fffffffffffffffffffffffffffffffffffffffff",
		22: "0x7fffffffffffffffffffffffffffffffffffffffffff",
		23: "0x7fffffffffffffffffffffffffffffffffffffffffffff",
		24: "0x7fffffffffffffffffffffffffffffffffffffffffffffff",
		25: "0x7fffffffffffffffffffffffffffffffffffffffffffffffff",
		26: "0x7fffffffffffffffffffffffffffffffffffffffffffffffffff",
		27: "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffff",
		28: "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		29: "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		30: "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		31: "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		32: "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}
	IntMin = map[int]string{
		1:  "-0x80",
		2:  "-0x8000",
		3:  "-0x800000",
		4:  "-0x80000000",
		5:  "-0x8000000000",
		6:  "-0x800000000000",
		7:  "-0x80000000000000",
		8:  "-0x8000000000000000",
		9:  "-0x800000000000000000",
		10: "-0x80000000000000000000",
		11: "-0x8000000000000000000000",
		12: "-0x800000000000000000000000",
		13: "-0x80000000000000000000000000",
		14: "-0x8000000000000000000000000000",
		15: "-0x800000000000000000000000000000",
		16: "-0x80000000000000000000000000000000",
		17: "-0x8000000000000000000000000000000000",
		18: "-0x800000000000000000000000000000000000",
		19: "-0x80000000000000000000000000000000000000",
		20: "-0x8000000000000000000000000000000000000000",
		21: "-0x800000000000000000000000000000000000000000",
		22: "-0x80000000000000000000000000000000000000000000",
		23: "-0x8000000000000000000000000000000000000000000000",
		24: "-0x800000000000000000000000000000000000000000000000",
		25: "-0x80000000000000000000000000000000000000000000000000",
		26: "-0x8000000000000000000000000000000000000000000000000000",
		27: "-0x800000000000000000000000000000000000000000000000000000",
		28: "-0x80000000000000000000000000000000000000000000000000000000",
		29: "-0x8000000000000000000000000000000000000000000000000000000000",
		30: "-0x800000000000000000000000000000000000000000000000000000000000",
		31: "-0x80000000000000000000000000000000000000000000000000000000000000",
		32: "-0x8000000000000000000000000000000000000000000000000000000000000000",
	}
)

type solidityInt Type

func (self solidityInt) String() string {
	return typeToString[Type(self)]
}
func (self solidityInt) size() uint32 {
	return uint32(self) - uint32(Int8) + 1
}

func (self solidityInt) getBigInt(value string) big.Int {
	Max := new(big.Int)
	v := new(big.Int)
	Max.SetString(IntMax[int(self.size())], 0)
	if strings.Contains(value, "0x") {
		v.SetString(value, 0)
	} else {
		v.SetString(value, 16)
	}
	v = v.And(v, Max)
	return *v
}

func (self solidityInt) fuzz(timestamp uint64) ([]interface{}, error) {
	var result []interface{}
	for _, seedItem := range GlobalIntSeed {
		if seedItem.Timestamp < timestamp {
			result = append(result, self.getBigInt(seedItem.Value))
		}
	}
	if len(result) == 0 {
		for i := 1; i <= int(self.size()); i++ {
			result = append(result, self.getBigInt(IntMax[i]))
		}
		result = append(result, self.getBigInt(IntMin[1]))
	}
	ret, err := intRand.RandomSelect(result)
	return []interface{}{ret}, err
}
