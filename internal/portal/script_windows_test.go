//go:build windows

package portal

import (
	"testing"

	"golang.org/x/text/encoding/japanese"
)

func TestDecodeConsoleOutputSmoke(t *testing.T) {
	// Encode "イーサネット" (Ethernet, katakana) to genuine Shift-JIS bytes and
	// splice it into otherwise-ASCII text, mimicking the real ipconfig output
	// that mixed English labels with a still-CP932-encoded adapter name.
	const ethernetJa = "イーサネット"
	sjisBytes, err := japanese.ShiftJIS.NewEncoder().String(ethernetJa)
	if err != nil {
		t.Fatalf("failed to encode test fixture: %v", err)
	}
	mixed := "Ethernet adapter " + sjisBytes + ":"
	want := "Ethernet adapter " + ethernetJa + ":"
	if got := decodeConsoleOutput(mixed); got != want {
		t.Fatalf("decodeConsoleOutput mismatch:\n got:  %q\n want: %q", got, want)
	}

	// Already-valid UTF-8 (e.g. plain ASCII, or genuinely UTF-8 PowerShell output)
	// must pass through unchanged.
	ascii := "Windows IP Configuration"
	if got := decodeConsoleOutput(ascii); got != ascii {
		t.Fatalf("valid UTF-8/ASCII input was altered: got %q, want %q", got, ascii)
	}
}
