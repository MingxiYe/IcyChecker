package fuzz

type solidityString Type

func (self solidityString) name() string {
	return typeToString[Type(self)]
}

func (self solidityString) fuzz(timestamp uint64) ([]interface{}, error) {
	var result []interface{}
	for _, seedItem := range GlobalStringSeed {
		if seedItem.Timestamp < timestamp {
			result = append(result, seedItem.Value)
		}
	}
	if len(result) == 0 {
		result = append(result, "hello")
		result = append(result, "ethereum")
		result = append(result, "hello, ethereum")
	}
	ret, err := stringRand.RandomSelect(result)
	return []interface{}{ret}, err
}
