package utils

import (
	jsoniter "github.com/json-iterator/go"
	"net/url"
	"strings"
	"syscall"
	"unicode"
)

func Blob(char byte, len int) []byte {
	b := make([]byte, len)
	for index := range b {
		b[index] = char
	}
	return b
}

func GetHostFromUrl(u string) string {
	if strings.Index(u, "://") == -1 {
		u = "http://" + u
	}
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return p.Host
}

func CheckFdLimit() {
	const min = 8192
	// Warn if ulimit is too low for production sites
	rlimit := &syscall.Rlimit{}
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, rlimit)
	if err == nil && rlimit.Cur < min {
		log.Warnf("WARNING: File descriptor limit %d is too low for production servers. "+
			"At least %d is recommended. Fix with `ulimit -n %d`.\n", rlimit.Cur, min, min)
	}
}

func IsASCII(s []byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func StrSliceContains(slice []string, item string) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

func StrSliceContainsI(slice []string, item string) bool {
	for _, i := range slice {
		if strings.EqualFold(i, item) {
			return true
		}
	}
	return false
}

func CastToStruct(from interface{}, to interface{}) (err error) {
	var raw []byte
	if raw, err = jsoniter.Marshal(from); err != nil {
		return
	}
	if err = jsoniter.Unmarshal(raw, to); err != nil {
		return
	}
	return nil
}
