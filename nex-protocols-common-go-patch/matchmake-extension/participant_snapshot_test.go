package matchmake_extension

import "testing"

func TestNewParticipantSnapshotEvent(t *testing.T) {
	event := newParticipantSnapshotEvent(42, 654321, 123456, "")

	if got := uint64(event.PIDSource); got != 654321 {
		t.Fatalf("PIDSource = %d, want joining PID 654321", got)
	}
	if got := uint32(event.Param1); got != 42 {
		t.Fatalf("Param1 = %d, want gathering ID 42", got)
	}
	if got := uint32(event.Param2); got != 123456 {
		t.Fatalf("Param2 = %d, want participant PID 123456", got)
	}
	if got := uint32(event.Param3); got != 1 {
		t.Fatalf("Param3 = %d, want one participant", got)
	}
}
