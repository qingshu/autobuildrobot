package tool

import (
	"unsafe"
)

//负责人：廖友平

//设置位标记
func Tool_BitSet(num int, pos uint) int {
	if pos <= 0 || pos > uint(unsafe.Sizeof(pos))*8 {
		return num
	}
	return (num | (0x00000001 << (pos - 1)))
}

//位检测
func Tool_BitTest(num int, pos uint) bool {
	if pos <= 0 || pos > uint(unsafe.Sizeof(pos))*8 {
		return false
	}
	return ((num >> (pos - 1)) & 0x00000001) == 1
}

//位清除
func Tool_BitClear(num int, pos uint) int {
	if pos <= 0 || pos > uint(unsafe.Sizeof(pos))*8 {
		return num
	}
	return (num &^ (0x00000001 << (pos - 1)))
}
