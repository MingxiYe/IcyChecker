package fuzz

type solidityAddress Type

func (self solidityAddress) name() string {
	return typeToString[Type(self)]
}

func (self solidityAddress) fuzz(timestamp uint64, localUsers []string, localContracts []string) ([]interface{}, error) {
	var result []interface{}
	for _, localUser := range localUsers {
		result = append(result, localUser)
	}
	for _, localContract := range localContracts {
		result = append(result, localContract)
	}

	if len(result) == 0 {
		for _, seedItem := range GlobalUserSeed {
			if seedItem.Timestamp < timestamp {
				result = append(result, seedItem.Value)
			}
		}
		for _, seedItem := range GlobalInnerSeed {
			if seedItem.Timestamp < timestamp {
				result = append(result, seedItem.Value)
			}
		}
		for _, seedItem := range GlobalOuterSeed {
			if seedItem.Timestamp < timestamp {
				result = append(result, seedItem.Value)
			}
		}
	}

	if ret, err := addressRand.RandomSelect(result); err == nil {
		return []interface{}{ret}, nil
	} else {
		return []interface{}{}, err
	}
}
