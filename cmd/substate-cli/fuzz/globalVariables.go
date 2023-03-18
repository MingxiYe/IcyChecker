package fuzz

const fuzz_Scale = 5

var (
	G_current_contract     interface{}
	G_current_fun          interface{}
	G_current_bin_fun_sigs = make(map[string][]string, 0)
	G_current_abi_sigs     = make(map[string]string, 0)
	RAND_CASE_SCALE        = 10
)

// each seed is store in hex (string) with 0x prefix
type SeedItem struct {
	Value     string
	Timestamp uint64
}

var (
	GlobalUserSeed   []SeedItem
	GlobalInnerSeed  []SeedItem
	GlobalOuterSeed  []SeedItem
	GlobalIntSeed    []SeedItem
	GlobalUintSeed   []SeedItem
	GlobalStringSeed []SeedItem
	GlobalByteSeed   []SeedItem
	GlobalBytesSeed  []SeedItem
	GlobalABIPath    string
)

var error_log = "./error-line.log"

func GetUserValueList() []interface{} {
	var ret []interface{}
	for _, item := range GlobalUserSeed {
		ret = append(ret, item.Value)
	}
	return ret
}
func GetInnerValueList() []interface{} {
	var ret []interface{}
	for _, item := range GlobalInnerSeed {
		ret = append(ret, item.Value)
	}
	return ret
}
func GetOuterValueList() []interface{} {
	var ret []interface{}
	for _, item := range GlobalOuterSeed {
		ret = append(ret, item.Value)
	}
	return ret
}

func UpdateGlobalSeed(data []byte, block uint64) {
	GlobalIntSeed = append(GlobalIntSeed, SeedItem{"0x" + string(data), block})
	GlobalUintSeed = append(GlobalUintSeed, SeedItem{"0x" + string(data), block})
	// get rid of prefix '0'
	for index, byteItem := range data {
		if byteItem != 0 {
			data = data[index:]
			break
		}
	}
	GlobalStringSeed = append(GlobalStringSeed, SeedItem{string(data), block})
	GlobalByteSeed = append(GlobalByteSeed, SeedItem{"0x" + string(data), block})
	GlobalBytesSeed = append(GlobalBytesSeed, SeedItem{"0x" + string(data), block})
}

func UpdateGlobalSeeds(data []byte, block uint64) {
	var parameterList []byte
	if len(data) <= 4 {
		return
	}
	parameterList = data[4:]
	for i := 0; i+32 <= len(parameterList); i += 32 {
		parameter := parameterList[i : i+32]
		UpdateGlobalSeed(parameter, block)
	}
}
