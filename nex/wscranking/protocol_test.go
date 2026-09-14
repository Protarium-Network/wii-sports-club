package wscranking

import (
	"encoding/hex"
	"testing"

	nex "github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	rankingtypes "github.com/PretendoNetwork/nex-protocols-go/v2/ranking/types"
)

func testStreamConfiguration() (*nex.LibraryVersions, *nex.ByteStreamSettings) {
	versions := nex.NewLibraryVersions()
	versions.SetDefault(nex.NewLibraryVersion(3, 4, 7))
	settings := nex.NewByteStreamSettings()
	settings.UseStructureHeader = false
	return versions, settings
}

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return decoded
}

func TestDecodeOfficialGlobalRequest(t *testing.T) {
	versions, settings := testStreamConfiguration()
	raw := mustDecodeHex(t, "00080000006b000000690000006a0000006c0000006d0000006e0000006700000068000000000900000000ff00020000000001000000000000000000000000")

	request, err := decodeRequest(raw, versions, settings)
	if err != nil {
		t.Fatalf("decode official request: %v", err)
	}

	if request.RankingMode != 0 {
		t.Fatalf("ranking mode = %d, want 0", request.RankingMode)
	}
	wantCategories := []uint32{107, 105, 106, 108, 109, 110, 103, 104}
	if len(request.Categories) != len(wantCategories) {
		t.Fatalf("category count = %d, want %d", len(request.Categories), len(wantCategories))
	}
	for index, want := range wantCategories {
		if uint32(request.Categories[index]) != want {
			t.Fatalf("category[%d] = %d, want %d", index, request.Categories[index], want)
		}
	}
	if uint8(request.OrderParam.OrderCalculation) != 0 || uint8(request.OrderParam.GroupIndex) != 255 || request.OrderParam.TimeScope != 2 || request.OrderParam.Offset != 0 || request.OrderParam.Length != 1 {
		t.Fatalf("unexpected order parameter: %s", request.OrderParam.String())
	}
	if request.UniqueID != 0 || request.PrincipalID != 0 {
		t.Fatalf("unexpected target: uniqueID=%d PID=%d", request.UniqueID, request.PrincipalID)
	}
	if settings.UseStructureHeader {
		t.Fatal("decodeRequest mutated the server's global byte-stream settings")
	}
}

func TestEncodeMatchesOfficialEmptyUserResponse(t *testing.T) {
	versions, settings := testStreamConfiguration()
	results := types.NewList[rankingtypes.RankingResult]()
	for range 8 {
		results = append(results, rankingtypes.NewRankingResult())
	}

	got := encodeResults(results, versions, settings)
	want := mustDecodeHex(t, "08000000001000000000000000000000000000000000000000001000000000000000000000000000000000000000001000000000000000000000000000000000000000001000000000000000000000000000000000000000001000000000000000000000000000000000000000001000000000000000000000000000000000000000001000000000000000000000000000000000000000001000000000000000000000000000000000000000")

	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("encoded response differs from Nintendo capture\n got: %x\nwant: %x", got, want)
	}
	if settings.UseStructureHeader {
		t.Fatal("encodeResults mutated the server's global byte-stream settings")
	}
}

func TestDecodeOfficialGlobalResponse(t *testing.T) {
	versions, settings := testStreamConfiguration()
	raw := mustDecodeHex(t, "08000000003f00000001000000002a000000fcbe476b0000000000000000010000006b00000098080000020000000100a04c0f00000000000000000027000000000042401f000000003f00000001000000002a00000008be476b00000000000000000100000069000000fc210000020000000100ed4c0f00000000000000000018000000000042401f000000003f00000001000000002a00000008be476b0000000000000000010000006a000000f0230000020000000100ed4c0f0000000000000000001e000000000042401f000000003f00000001000000002a00000003b8476b0000000000000000010000006c000000f4330000020000000300ed4e0f00000000000000000019000000000042401f000000003f00000001000000002a000000ddbe476b0000000000000000010000006d000000d80e0000020000000100ae4c0f0000000000000000000f000000000042401f000000003f00000001000000002a00000005bf476b0000000000000000010000006e00000098080000020000000400ac4d0f00000000000000000013000000000042401f000000003f00000001000000002a00000005bf476b000000000000000001000000670000001c250000020000000400ac4d0f00000000000000000016000000000042401f000000003f00000001000000002a000000fcbe476b0000000000000000010000006800000000190000020000000600a04c0f0000000000000000000f000000000042401f000000")

	stream := nex.NewByteStreamIn(raw, versions, structureHeaderSettings(settings))
	var results types.List[rankingtypes.RankingResult]
	if err := results.ExtractFrom(stream); err != nil {
		t.Fatalf("decode official response: %v", err)
	}
	if stream.Remaining() != 0 || len(results) != 8 {
		t.Fatalf("remaining=%d result count=%d", stream.Remaining(), len(results))
	}
	wantScores := []uint32{2200, 8700, 9200, 13300, 3800, 2200, 9500, 6400}
	for index, wantScore := range wantScores {
		if len(results[index].RankDataList) != 1 {
			t.Fatalf("result[%d] has %d rows", index, len(results[index].RankDataList))
		}
		if got := uint32(results[index].RankDataList[0].Score); got != wantScore {
			t.Fatalf("result[%d] score=%d, want %d", index, got, wantScore)
		}
	}
	if results[0].TotalCount != 39 || results[0].RankDataList[0].Category != 107 {
		t.Fatalf("unexpected first result: %s", results[0].String())
	}
}
