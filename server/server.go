package server

type ID string
type Type string

type Config map[string]interface{}

func (c Config) id() ID {
	n, ok := c["id"]
	if !ok {
		panic("missing required server config field 'id'")
	}
	return n.(ID)
}

func (c Config) typ() Type {
	n, ok := c["id"]
	if !ok {
		panic("missing required server config field 'type'")
	}
	return n.(Type)
}

type Server interface {
	Serve() error
	Stop() error
}