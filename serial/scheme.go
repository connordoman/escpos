package serial

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/connordoman/escpos"
)

// Importing this package lets escpos.Open handle connection strings such as
// serial:/dev/ttyUSB0?baud=19200&databits=8&parity=none&stopbits=1 (or
// serial:COM3 on Windows).
func init() {
	escpos.RegisterScheme("serial", openSpec)
}

func openSpec(_ context.Context, spec escpos.Spec) (io.WriteCloser, escpos.Connection, error) {
	c := escpos.Connection{Kind: "serial", Target: spec.Target}
	if spec.Target == "" {
		return nil, c, fmt.Errorf("serial: connection needs a port, as in serial:/dev/ttyUSB0")
	}
	var opts Options
	ints := []struct {
		key string
		dst *int
	}{{"baud", &opts.BaudRate}, {"databits", &opts.DataBits}, {"stopbits", &opts.StopBits}}
	for _, f := range ints {
		if v := spec.Query.Get(f.key); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				return nil, c, fmt.Errorf("serial: %s=%q is not a positive integer", f.key, v)
			}
			*f.dst = n
		}
	}
	switch p := strings.ToLower(spec.Query.Get("parity")); p {
	case "", "none", "n":
	case "odd", "o":
		opts.Parity = OddParity
	case "even", "e":
		opts.Parity = EvenParity
	default:
		return nil, c, fmt.Errorf("serial: parity=%q is not none, odd or even", p)
	}
	conn, err := Open(spec.Target, opts)
	return conn, c, err
}
