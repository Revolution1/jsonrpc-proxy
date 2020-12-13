package main

import (
	log "github.com/sirupsen/logrus"
	"net/url"
	"strings"
	"syscall"
	"unicode"
)

type fnv64a struct{}

const (
	// offset64 FNVa offset basis. See https://en.wikipedia.org/wiki/Fowler–Noll–Vo_hash_function#FNV-1a_hash
	offset64 = 14695981039346656037
	// prime64 FNVa prime value. See https://en.wikipedia.org/wiki/Fowler–Noll–Vo_hash_function#FNV-1a_hash
	prime64 = 1099511628211
)

// Sum64 gets the string and returns its uint64 hash value.
func (f fnv64a) Sum64(key string) uint64 {
	var hash uint64 = offset64
	for i := 0; i < len(key); i++ {
		hash ^= uint64(key[i])
		hash *= prime64
	}

	return hash
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

func isASCII(s []byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}
