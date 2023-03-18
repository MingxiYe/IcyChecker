package fuzz

type solidityByte Type

func (self solidityByte) String() string {
	return typeToString[Type(self)]
}
func (self solidityByte) size() uint32 {
	return uint32(self) - uint32(Bytes1) + 1
}

var (
	ByteMax = map[int]string{
		1:  "0xff",
		2:  "0xffff",
		3:  "0xffffff",
		4:  "0xffffffff",
		5:  "0xffffffffff",
		6:  "0xffffffffffff",
		7:  "0xffffffffffffff",
		8:  "0xffffffffffffffff",
		9:  "0xffffffffffffffffff",
		10: "0xffffffffffffffffffff",
		11: "0xffffffffffffffffffffff",
		12: "0xffffffffffffffffffffffff",
		13: "0xffffffffffffffffffffffffff",
		14: "0xffffffffffffffffffffffffffff",
		15: "0xffffffffffffffffffffffffffffff",
		16: "0xffffffffffffffffffffffffffffffff",
		17: "0xffffffffffffffffffffffffffffffffff",
		18: "0xffffffffffffffffffffffffffffffffffff",
		19: "0xffffffffffffffffffffffffffffffffffffff",
		20: "0xffffffffffffffffffffffffffffffffffffffff",
		21: "0xffffffffffffffffffffffffffffffffffffffffff",
		22: "0xffffffffffffffffffffffffffffffffffffffffffff",
		23: "0xffffffffffffffffffffffffffffffffffffffffffffff",
		24: "0xffffffffffffffffffffffffffffffffffffffffffffffff",
		25: "0xffffffffffffffffffffffffffffffffffffffffffffffffff",
		26: "0xffffffffffffffffffffffffffffffffffffffffffffffffffff",
		27: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		28: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		29: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		30: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		31: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		32: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}
	ByteMin = map[int]string{
		1:  "0x0",
		2:  "0x0",
		3:  "0x0",
		4:  "0x0",
		5:  "0x0",
		6:  "0x0",
		7:  "0x0",
		8:  "0x0",
		9:  "0x0",
		10: "0x0",
		11: "0x0",
		12: "0x0",
		13: "0x0",
		14: "0x0",
		15: "0x0",
		16: "0x0",
		17: "0x0",
		18: "0x0",
		19: "0x0",
		20: "0x0",
		21: "0x0",
		22: "0x0",
		23: "0x0",
		24: "0x0",
		25: "0x0",
		26: "0x0",
		27: "0x0",
		28: "0x0",
		29: "0x0",
		30: "0x0",
		31: "0x0",
		32: "0x0",
	}
)

func (self solidityByte) getSlice(value interface{}) interface{} {
	valueByte := []byte(value.(string))
	if len(valueByte) > int(self.size()) {
		valueByte = valueByte[len(valueByte)-int(self.size()):]
	}
	return string(valueByte)
}

func (self solidityByte) fuzz(timestamp uint64) ([]interface{}, error) {
	var result []interface{}
	for _, seedItem := range GlobalByteSeed {
		if seedItem.Timestamp < timestamp {
			result = append(result, self.getSlice(seedItem.Value))
		}
	}
	if len(result) == 0 {
		for i := 1; i <= int(self.size()); i++ {
			result = append(result, ByteMax[i])
		}
		result = append(result, ByteMin[1])
	}
	ret, err := byteRand.RandomSelect(result)
	return []interface{}{ret}, err
}
