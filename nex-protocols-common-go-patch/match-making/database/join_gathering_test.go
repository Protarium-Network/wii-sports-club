package database

import (
	"slices"
	"testing"
)

func TestNewParticipantEventCount(t *testing.T) {
	tests := []struct {
		name               string
		vacantParticipants uint16
		want               uint32
	}{
		{name: "legacy zero means caller", vacantParticipants: 0, want: 1},
		{name: "one joining player", vacantParticipants: 1, want: 1},
		{name: "three local participants", vacantParticipants: 3, want: 3},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := newParticipantEventCount(test.vacantParticipants)
			if got != test.want {
				t.Fatalf("newParticipantEventCount(%d) = %d, want %d", test.vacantParticipants, got, test.want)
			}
		})
	}
}

func TestAppendParticipantsPreservesHostFirst(t *testing.T) {
	existing := []uint64{100, 200}
	joining := []uint64{300, 300}

	got := appendParticipants(existing, joining)
	want := []uint64{100, 200, 300, 300}
	if !slices.Equal(got, want) {
		t.Fatalf("appendParticipants() = %v, want %v", got, want)
	}

	got[0] = 999
	if existing[0] != 100 {
		t.Fatalf("appendParticipants mutated existing slice: %v", existing)
	}
}
