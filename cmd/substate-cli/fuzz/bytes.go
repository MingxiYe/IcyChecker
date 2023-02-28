package fuzz

type solidityBytes Type

func (self solidityBytes) name() string {
	return typeToString[Type(self)]
}

func (self solidityBytes) fuzz(timestamp uint64) ([]interface{}, error) {
	var result []interface{}
	for _, seedItem := range GlobalBytesSeed {
		if seedItem.Timestamp < timestamp {
			result = append(result, seedItem.Value)
		}
	}
	if len(result) == 0 {
		for i := 1; i <= 32; i++ {
			result = append(result, ByteMax[i])
		}
		result = append(result, ByteMin[1])
	}
	ret, err := bytesRand.RandomSelect(result)
	return []interface{}{ret}, err
}
