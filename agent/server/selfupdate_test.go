package server

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		want []int
		ok   bool
	}{
		{"1.0.6", []int{1, 0, 6}, true},
		{"1.0.5", []int{1, 0, 5}, true},
		{"2.10.0", []int{2, 10, 0}, true},
		{"1.0", []int{1, 0}, true},
		{"", nil, false},
		{"1.0.5x", nil, false}, // 畸形段不得被宽松解析为 5
		{"1..5", nil, false},
		{"1.0.-1", nil, false},
		{"v1.0.6", []int{1, 0, 6}, true},
		{"vv1.0.6", nil, false},
		{"v1.0.6x", nil, false},
		{"v", nil, false},
	}
	for _, c := range cases {
		got, ok := parseVersion(c.in)
		if ok != c.ok {
			t.Errorf("parseVersion(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("parseVersion(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("parseVersion(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.5", "1.0.6", true},
		{"1.0.6", "1.0.5", false},
		{"1.0.6", "1.0.6", false},
		{"1.0.9", "1.0.10", true},      // 数值比较，非字符串
		{"1.0", "1.0.1", true},         // 前缀更短
		{"1.9.9", "1.10.0", true},      // 数值比较
		{"1.0.5x", "1.0.6", true},      // 畸形版本按字符串比较："1.0.5x" < "1.0.6"（'5'<'6'）
		{"garbage", "1.0.6", false},    // 畸形目标按字符串比较不小于 → 拒绝升级（安全默认）
		{"snapshot-1", "1.0.6", false}, // 同上
	}
	for _, c := range cases {
		if got := versionLess(c.a, c.b); got != c.want {
			t.Errorf("versionLess(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	got := assetName()
	if got == "" {
		t.Fatal("assetName() is empty")
	}
	// 格式：komari-agent-<os>-<arch>
	if len(got) < len("komari-agent--") {
		t.Fatalf("assetName() = %q, unexpected shape", got)
	}
}
