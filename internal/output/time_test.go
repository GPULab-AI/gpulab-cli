package output

import (
	"testing"
	"time"
)

func TestParseTimestamp_MillisEpoch(t *testing.T) {
	// 1_700_000_000_000 ms = 2023-11-14T…; must NOT be read as a seconds epoch
	// (which would land in the year ~55600).
	got, ok := ParseTimestamp("1700000000000")
	if !ok {
		t.Fatal("failed to parse ms epoch")
	}
	if got.Year() != 2023 {
		t.Fatalf("ms epoch parsed to wrong year: %d", got.Year())
	}
}

func TestParseTimestamp_SecondsEpochAndISO(t *testing.T) {
	sec, ok := ParseTimestamp("1700000000")
	if !ok || sec.Year() != 2023 {
		t.Fatalf("seconds epoch parse: ok=%v year=%d", ok, sec.Year())
	}
	iso, ok := ParseTimestamp("2026-07-03T11:34:27+05:30")
	if !ok || iso.UTC().Hour() != 6 {
		t.Fatalf("ISO with offset parse: ok=%v hour=%d", ok, iso.UTC().Hour())
	}
}

func TestParseFlexibleDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"10m":  10 * time.Minute,
		"1h":   time.Hour,
		"2d":   48 * time.Hour,
		"1d6h": 30 * time.Hour,
		"90s":  90 * time.Second,
	}
	for in, want := range cases {
		got, err := ParseFlexibleDuration(in)
		if err != nil || got != want {
			t.Fatalf("%s: got %v err %v, want %v", in, got, err, want)
		}
	}
}

func TestHumanDuration(t *testing.T) {
	if got := HumanDuration(2*24*time.Hour + 8*time.Hour); got != "2d8h" {
		t.Fatalf("want 2d8h got %s", got)
	}
	if got := HumanDuration(45 * time.Minute); got != "45m" {
		t.Fatalf("want 45m got %s", got)
	}
}
