package tui

import "testing"

func TestMsgDetectingHardware(t *testing.T) {
	if MsgDetectingHardware != "Detectando hardware…" {
		t.Errorf("MsgDetectingHardware = %q, want %q", MsgDetectingHardware, "Detectando hardware…")
	}
}
