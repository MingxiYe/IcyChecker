package fuzz

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var fixReg = regexp.MustCompile("^(.*)\\[([\\d]+)\\]+$")
var DynReg = regexp.MustCompile("^(.*)\\[\\].*$")

const (
	Cfundemental uint32 = iota
	CfixedArray
	CdynamicArray
)

func getInfo(typestr string) (uint32, error) {
	typestr = strings.TrimSpace(typestr)

	if match := fixReg.MatchString(typestr); match == true {
		return CfixedArray, nil
	} else if match := DynReg.MatchString(typestr); match == true {
		return CdynamicArray, nil
	} else if v, err := strToType(typestr); err == nil {
		return Cfundemental, nil
	} else {
		return uint32(v), err
	}
}

type FixedArray struct {
	ArrayElem Type          `json:"element_type"`
	Size      uint32        `json:"size"`
	Str       string        `json:"description"`
	Out       []interface{} `json:"fuzz_out"`
	Ostream   io.Writer     `json:"-"`
}

func newFixedArray(str string) *FixedArray {
	f := new(FixedArray)
	f.Str = str
	match := fixReg.FindStringSubmatch(f.Str)
	if len(match) != 0 {
		elemstr := match[1]
		size, _ := strconv.Atoi(match[2])
		elem, err := strToType(elemstr)
		if err != nil {
			fmt.Errorf("%s", err)
		}
		f.ArrayElem = elem
		f.Size = uint32(size)
		f.Out = make([]interface{}, 0)
	}
	return f
}
func (f *FixedArray) String() string {
	buf, _ := json.Marshal(f)
	return string(buf)
}

// generate one array item once.
func (f *FixedArray) fuzz(timestamp uint64, users []string, contracts []string) ([]interface{}, error) {
	var (
		out  = make([]interface{}, 0)
		size = f.Size
	)
	for i := uint32(0); i < size; i++ {
		if m_out, err := f.ArrayElem.fuzz(timestamp, users, contracts); err == nil {
			out = append(out, m_out[0])
		} else {
			return nil, err
		}
	}
	return []interface{}{out}, nil
}
func (f *FixedArray) SetOstream(file string) {
	if ostream, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666); err != nil {
		fmt.Printf("%s", FILE_OPEN_ERROR(err))
	} else {
		f.Ostream = io.Writer(ostream)
	}
}
func (f *FixedArray) Write(data []byte) {
	f.Ostream.Write(data)
}

type DynamicArray struct {
	ArrayElem Type          `json:"element_type"`
	Str       string        `json:"description"`
	Out       []interface{} `json:"fuzz_out"`
}

func newDynamicArray(str string) *DynamicArray {
	d := new(DynamicArray)
	d.Str = str
	match := DynReg.FindStringSubmatch(d.Str)
	if len(match) != 0 {
		elemstr := match[1]
		elem, err := strToType(elemstr)
		if err != nil {
			fmt.Errorf("%s", err)
		}
		d.ArrayElem = elem
		d.Out = make([]interface{}, 0)
	}
	return d
}
func (d *DynamicArray) fuzz(timestamp uint64, users []string, contracts []string) ([]interface{}, error) {
	const ARRAY_SIZE_LIMIT = 10
	size := randintOne(1, ARRAY_SIZE_LIMIT)
	str_fixArray := fmt.Sprintf("%s[%d]", typeToString[d.ArrayElem], size)
	fixArray := newFixedArray(str_fixArray)
	out, err := fixArray.fuzz(timestamp, users, contracts)
	return out, err
}
func (d *DynamicArray) String() string {
	buf, _ := json.Marshal(d)
	return string(buf)
}

// entry function for array/fundemental type
func fuzz(typeStr string, timestamp uint64, users []string, contracts []string) ([]interface{}, error) {
	v, err := getInfo(typeStr)

	if err != nil {
		return nil, err
	} else {
		switch v {
		case Cfundemental:
			{
				f, _ := strToType(typeStr)
				out, _ := f.fuzz(timestamp, users, contracts)
				return out, nil
			}
		case CfixedArray:
			{
				f := newFixedArray(typeStr)
				out, _ := f.fuzz(timestamp, users, contracts)
				return out, nil
			}
		case CdynamicArray:
			{
				d := newDynamicArray(typeStr)
				out, _ := d.fuzz(timestamp, users, contracts)
				return out, nil
			}
		default:
			return nil, ERR_UNKNOWN_COMPLEX_TYPE
		}
	}

}
