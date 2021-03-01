package types

import (
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
)

type Size uint64

func (s *Size) UnmarshalJSON(b []byte) (err error) {
	if b[0] == '"' {
		var nBytes uint64
		sd := string(b[1 : len(b)-1])
		nBytes, err = humanize.ParseBytes(sd)
		*s = Size(int(nBytes))
		return
	}
	nBytes, err := json.Number(b).Int64()
	*s = Size(nBytes)
	return
}

func (s *Size) MarshalJSON() (b []byte, err error) {
	return []byte(fmt.Sprintf(`"%s"`, humanize.IBytes(uint64(*s)))), nil
}
