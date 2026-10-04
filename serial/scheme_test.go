package serial

import (
	"context"
	"strings"
	"testing"

	"github.com/connordoman/escpos"
)

func TestSchemeRegistered(t *testing.T) {
	_, _, err := escpos.Open(context.Background(), "serial:/dev/does-not-exist?baud=9600")
	if err == nil || strings.Contains(err.Error(), "unknown connection scheme") {
		t.Errorf("serial scheme not registered: %v", err)
	}
	for _, bad := range []string{"serial:", "serial:/dev/x?baud=fast", "serial:/dev/x?parity=mark"} {
		if _, _, err := escpos.Open(context.Background(), bad); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}
