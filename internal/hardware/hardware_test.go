package hardware

import "testing"

func TestBytesToGB(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  float64
	}{
		{0, 0},
		{1073741824, 1.0},       // 1 GB
		{8589934592, 8.0},       // 8 GB
		{17179869184, 16.0},     // 16 GB
		{549755813888, 512.0},   // 512 GB
		{536870912, 0.5},        // 512 MB
		{1099511627776, 1024.0}, // 1 TB
	}
	for _, tc := range tests {
		got := bytesToGB(tc.bytes)
		if got != tc.want {
			t.Errorf("bytesToGB(%d) = %v, want %v", tc.bytes, got, tc.want)
		}
	}
}

func TestGPUKindString(t *testing.T) {
	tests := []struct {
		kind GPUKind
		want string
	}{
		{GPUKindNone, "none"},
		{GPUKindNVIDIA, "nvidia"},
		{GPUKindAMD, "amd"},
		{GPUKindApple, "apple"},
		{GPUKindIntel, "intel"},
	}
	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			got := tc.kind.String()
			if got != tc.want {
				t.Errorf("%q.String() = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestParseAppleVRAM(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"1536 MB", 1.5},
		{"4096 MB", 4.0},
		{"8 GB", 8.0},
		{"16 GB", 16.0},
		{"2 gb", 2.0},   // case insensitive
		{"512 mb", 0.5}, // case insensitive
		{"", 0},
		{"bad", 0},
		{"  ", 0},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := parseAppleVRAM(tc.input)
			if got != tc.want {
				t.Errorf("parseAppleVRAM(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
