package config

import (
	"strconv"
	"strings"
)

var version = "1.1.0"

func FullVersion() string {
	return version
}

func getSubVersion(v string, position int) int64 {
	arr := strings.Split(v, ".")
	if len(arr) < 3 {
		return 0
	}
	res, _ := strconv.ParseInt(arr[position], 10, 64)
	return res
}

func ProtoVersion(v string) int64 {
	return getSubVersion(v, 0)
}

func MajorVersion(v string) int64 {
	return getSubVersion(v, 1)
}

func MinorVersion(v string) int64 {
	return getSubVersion(v, 2)
}
