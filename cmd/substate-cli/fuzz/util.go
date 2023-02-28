package fuzz

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"time"
)

var (
	count = 0
)
var (
	fuzz_rand = rand.Intn(time.Now().Nanosecond())
	s         = rand.NewSource(time.Now().UnixNano() + int64(fuzz_rand))
	r         = rand.New(s)
)

func randBint(max, min big.Int) *big.Int {
	count += 1
	fuzz_rand = rand.Intn(count)
	s = rand.NewSource(time.Now().UnixNano() + int64(fuzz_rand))
	r = rand.New(s)
	return new(big.Int).Add(new(big.Int).Rand(r, new(big.Int).Sub(&max, &min)), &min)
}
func toJsonStr(v interface{}) []byte {
	buf, _ := json.Marshal(v)
	return buf
}
func readFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fileinfo, _ := file.Stat()
	Size := fileinfo.Size()
	out := make([]byte, Size)
	if _, err := file.Read(out); err == nil {
		return out, nil
	} else {
		return nil, err
	}
}
func readDir(path string) ([]string, error) {
	fileinfos, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(fileinfos))
	for _, fileinfo := range fileinfos {
		file := fileinfo.Name()
		files = append(files, file)
	}
	return files, nil
}
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func randintOne(max, min int) int {
	count += 1
	fuzz_rand = rand.Intn(count)
	s = rand.NewSource(time.Now().UnixNano() + int64(fuzz_rand))
	r = rand.New(s)
	if max-min <= 0 {
		return Max(max, min)
	}
	tmp := r.Intn(max - min)
	count += tmp
	return tmp + min
}
func randIntN(max, min big.Int, size int, out []big.Int) {
	var bInt big.Int
	for i := 0; i < size; i++ {
		n := new(big.Int)
		n.Set(bInt.Add(bInt.Rand(r, bInt.Sub(&max, &min)), &min))
		out[i] = *n
	}
}

func randintN(max, min int, size int, out []int) {
	for i := 0; i < size; i++ {
		n := r.Intn(max-min) + min
		out[i] = n
	}
}

type BigInt big.Int

func (bInt BigInt) String() string {
	var myInt big.Int
	var bint = big.Int(bInt)
	sign := bint.Sign()
	if sign == 0 {
		return "-0x80"
	} else if sign == 1 {
		return fmt.Sprintf("0x%s", bint.Text(16))
	} else {
		abs := myInt.Abs(&bint)
		return fmt.Sprintf("-0x%s", abs.Text(16))
	}
}

type BigUint big.Int

func (bUint BigUint) String() string {
	var myUint big.Int
	var bint = big.Int(bUint)
	sign := bint.Sign()
	if sign == 0 {
		return "0x0"
	} else if sign == 1 {
		return fmt.Sprintf("0x%s", bint.Text(16))
	} else {
		// unreachable location
		abs := myUint.Abs(&bint)
		return fmt.Sprintf("-0x%s", abs.Text(16))
	}
}

// const(
// 	ERR_ZERO_SIZED_SLICE = iota+101
// 	ERR_OVER_RANDOM_LIMIT
// 	ERR_UNKNOWN_COMPLEX_TYPE
// 	ERR_TYPE_NOT_FOUND
// 	ERR_FUZZ_TYPE_FAILED
// )
var errorCodeMap = map[int]string{
	101: "Zero-sized slice to handle",
	102: "Try over random time limit",
	103: "Cannot understand the complex type",
	104: "Cannot found type",
	105: "Fuzz type failed",
	106: "Abi fuzz failed out of unknown error",
	107: "Random select failed out of unknown error",
}

type Error struct {
	errorCode int
}

func NewError(code int) Error {
	return Error{errorCode: code}
}
func (err Error) Error() string {
	return fmt.Sprintf("%d:%s", err.errorCode, errorCodeMap[err.errorCode])
}

var (
	ERR_ZERO_SIZED_SLICE     = NewError(101)
	ERR_OVER_RANDOM_LIMIT    = NewError(102)
	ERR_UNKNOWN_COMPLEX_TYPE = NewError(103)
	ERR_TYPE_NOT_FOUND       = NewError(104)
	ERR_FUZZ_TYPE_FAILED     = NewError(105)
	ERR_ABI_FUZZ_FAILED      = NewError(106)
	ERR_RANDOM_SELECT_FAILED = NewError(107)
)

func printCallStackIfError() {
	for i := 0; i < 10; i++ {
		funcName, file, line, ok := runtime.Caller(i)
		if ok {
			log.Printf("frame %v:[func:%v,file:%v,line:%v]\n", i, runtime.FuncForPC(funcName).Name(), file, line)
		}
	}
}

func Convert2InterfaceSlice(slice interface{}) (ret []interface{}) {
	ret = make([]interface{}, 0)
	switch slice.(type) {
	case []string:
		for _, item := range slice.([]string) {
			ret = append(ret, item)
		}
		return ret
	case []int:
		for _, item := range slice.([]int) {
			ret = append(ret, item)
		}
		return ret
	case []bool:
		for _, item := range slice.([]bool) {
			ret = append(ret, item)
		}
		return ret
	default:
		return nil
	}
}

func ConvertStringSlice2InterfaceSlice(strSlice []string) []interface{} {
	var interSlice []interface{} = make([]interface{}, 0)
	for _, item := range strSlice {
		interSlice = append(interSlice, item)
	}
	return interSlice
}
func ConvertInterfaceSlice2StringSlice(interSlice []interface{}) []string {
	var strSlice []string = make([]string, 0)
	for _, item := range interSlice {
		strSlice = append(strSlice, item.(string))
	}
	return strSlice
}
func ConvertIntSlice2InterfaceSlice(strSlice []int) []interface{} {
	var interSlice []interface{} = make([]interface{}, 0)
	for _, item := range strSlice {
		// BigInt(elem.(big.Int)).String()
		interSlice = append(interSlice, BigInt(*new(big.Int).SetInt64(int64(item))).String())
	}
	return interSlice
}

func FILE_OPEN_ERROR(err error) error {
	return fmt.Errorf("file open error. %s", err)
}
func FILE_READ_ERROR(err error) error {
	return fmt.Errorf("file read error. %s", err)
}
func FILE_WRITE_ERROR(err error) error {
	return fmt.Errorf("file write error. %s", err)
}
func JSON_MARSHAL_ERROR(err error) error {
	return fmt.Errorf("json marshal error. %s", err)
}
func JSON_UNMARSHAL_ERROR(err error) error {
	return fmt.Errorf("json unmarshal error. %s", err)
}
func DYNAMIC_CAST_ERROR(ok bool) error {
	if ok != true {
		return fmt.Errorf("dynamic cast error. ")
	} else {
		return nil
	}
}
func SWICTH_DEFAULT_ERROR(err error) error {
	if err != nil {
		return fmt.Errorf("swictch default error. %s", err)
	} else {
		return fmt.Errorf("swictch default error. ")
	}
}

/*
 * outs[][], first dimension: parameter, second dimension: concrete value
 * stringify generated output in vals
 * only the first one is return now
 * this is for abi.go
 */
func stringify(val interface{}) string {
	_, ok := val.(string)
	if ok {
		if strings.Contains(val.(string), "\"") {
			return val.(string)
		} else {
			return "\"" + val.(string) + "\""
		}

	} else {
		data, _ := json.Marshal(val)
		return string(data)
	}
}
func product(A, B []interface{}) []interface{} {
	var rets = make([]interface{}, 0, 0)
	for _, a := range A {
		for _, b := range B {
			rets = append(rets, stringify(a)+","+stringify(b))
		}
	}
	return rets
}
func cartesianProductOne(outs [][]interface{}) string {
	var vals = make([]interface{}, 0)
	if len(outs) == 0 {
		return ""
	}
	for _, v := range outs[0] {
		vals = append(vals, stringify(v))
	}
	for i := 1; i < len(outs); i++ {
		vals = product(vals, outs[i])
	}
	return vals[0].(string)
}
