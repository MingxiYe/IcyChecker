package fuzz

type solidityBool Type

func (self solidityBool) name() string {
	return typeToString[Type(self)]
}

func (self solidityBool) fuzz() ([]interface{}, error) {
	v := []interface{}{
		true,
		false,
	}
	ret, err := boolRand.RandomSelect(v)
	return []interface{}{ret}, err
}
