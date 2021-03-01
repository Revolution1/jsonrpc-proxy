package types

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"net/url"
	"strings"
)

func (u *UpstreamType) UnmarshalJSON(bytes []byte) error {
	var s string
	if err := jsoniter.Unmarshal(bytes, &s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	case "http":
		*u = HTTPUpstream
	case "tcp":
		*u = TCPUpstream
	default:
		return errors.Errorf("unknown upstreamType %v", s)
	}
	return nil
}

func (u UpstreamType) MarshalJSON() ([]byte, error) {
	switch u {
	case HTTPUpstream:
		return []byte(`"HTTP"`), nil
	case TCPUpstream:
		return []byte(`"TCP"`), nil
	default:
		return nil, errors.New("unknown UpstreamType")
	}
}

func (e *Endpoint) Parse(u string) error {
	if u, err := url.Parse(u); err != nil {
		return err
	} else {
		*e = Endpoint(*u)
	}
	return nil
}

func (e *Endpoint) UnmarshalJSON(bytes []byte) error {
	var s string
	if err := jsoniter.Unmarshal(bytes, &s); err != nil {
		return err
	}
	if u, err := url.Parse(s); err != nil {
		return err
	} else {
		*e = Endpoint(*u)
	}
	return nil
}

func (e *Endpoint) MarshalJSON() ([]byte, error) {
	u := url.URL(*e)
	return []byte(fmt.Sprintf(`"%s"`, u.String())), nil
}
