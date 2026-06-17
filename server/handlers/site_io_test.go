package handlers

// 本文件：site_io.go 的单元测试。
// 覆盖 ReadSites / WriteSites / FindSiteByName / FilterSitesByQuery。
//
// TDD 流程：写测试（RED）→ 跑测试确认失败 → 写 site_io.go（GREEN）→ 跑测试确认通过。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ----- ReadSites -----

// 1. 文件不存在 → 返空 slice + nil error
func TestReadSitesFileMissing(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "target_sites.json")

	sites, err := ReadSites(full)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(sites) != 0 {
		t.Errorf("want empty slice, got %v", sites)
	}
}

// 2. 文件存在但为空 → 返空 slice + nil error
func TestReadSitesFileEmpty(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "target_sites.json")
	if err := os.WriteFile(full, []byte(""), 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}

	sites, err := ReadSites(full)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(sites) != 0 {
		t.Errorf("want empty slice, got %v", sites)
	}
}

// 3. 文件内容是合法 JSON 数组 → 解析并返回
func TestReadSitesValid(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "target_sites.json")
	content := `[
		{"name": "SET上市公司名录", "url": "https://example.com/set", "country": "泰国", "industry": "综合", "type": "download"},
		{"name": "BOI 投资促进局", "url": "https://boi.go.th", "country": "泰国", "industry": "政府", "type": "portal"}
	]`
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sites, err := ReadSites(full)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("want 2 sites, got %d: %v", len(sites), sites)
	}
	if sites[0].Name != "SET上市公司名录" {
		t.Errorf("want first name=SET上市公司名录, got %q", sites[0].Name)
	}
	if sites[0].URL != "https://example.com/set" {
		t.Errorf("want first url=https://example.com/set, got %q", sites[0].URL)
	}
	if sites[0].Country != "泰国" {
		t.Errorf("want first country=泰国, got %q", sites[0].Country)
	}
	if sites[0].Industry != "综合" {
		t.Errorf("want first industry=综合, got %q", sites[0].Industry)
	}
	if sites[0].Type != "download" {
		t.Errorf("want first type=download, got %q", sites[0].Type)
	}
	if sites[1].Name != "BOI 投资促进局" {
		t.Errorf("want second name=BOI 投资促进局, got %q", sites[1].Name)
	}
}

// 4. 文件内容是合法 JSON 但不是数组 → 返 error
func TestReadSitesNotArray(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "target_sites.json")
	if err := os.WriteFile(full, []byte(`{"name":"oops"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sites, err := ReadSites(full)
	if err == nil {
		t.Fatalf("want error, got nil (sites=%v)", sites)
	}
}

// 5. 文件内容是损坏的 JSON → 返 error
func TestReadSitesCorrupt(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "target_sites.json")
	if err := os.WriteFile(full, []byte(`{this is not JSON`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sites, err := ReadSites(full)
	if err == nil {
		t.Fatalf("want error, got nil (sites=%v)", sites)
	}
}

// 6. 空数组 JSON "[]" → 返空 slice + nil error
func TestReadSitesEmptyArray(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "target_sites.json")
	if err := os.WriteFile(full, []byte("[]"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sites, err := ReadSites(full)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if len(sites) != 0 {
		t.Errorf("want empty slice, got %v", sites)
	}
}

// ----- WriteSites -----

// 7. 写后读回 round-trip（中文 + 特殊字符）
func TestWriteSitesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "target_sites.json")
	in := []TargetSite{
		{Name: "SET上市公司名录", URL: "https://example.com/set?a=1&b=2", Country: "泰国", Industry: "综合", Type: "download"},
		{Name: "BOI 投资促进局", URL: "https://boi.go.th", Country: "泰国", Industry: "政府", Type: "portal"},
		{Name: "quotes\"and\\backslash", URL: "https://x.example", Country: "USA", Industry: "fin", Type: "api"},
	}

	if err := WriteSites(full, in); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, err := ReadSites(full)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("want %d sites, got %d", len(in), len(out))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("index %d: want %+v, got %+v", i, in[i], out[i])
		}
	}
}

// 8. 写后 .tmp 不残留
func TestWriteSitesLeavesNoTmp(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "target_sites.json")
	if err := WriteSites(full, []TargetSite{{Name: "a", URL: "https://a"}}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := os.Stat(full + ".tmp"); !os.IsNotExist(err) {
		t.Errorf(".tmp should not exist, stat err=%v", err)
	}
	if _, err := os.Stat(full); err != nil {
		t.Errorf("target file should exist: %v", err)
	}
}

// 9. 写入空 slice → 文件内容是 "[]\n"
func TestWriteSitesEmpty(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "target_sites.json")
	if err := WriteSites(full, []TargetSite{}); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := strings.TrimSpace(string(got))
	if s != "[]" {
		t.Errorf("want '[]', got %q", s)
	}
}

// ----- FindSiteByName -----

// 10. 名字存在 → 返非 nil 指针，且字段一致
func TestFindSiteByNameExists(t *testing.T) {
	sites := []TargetSite{
		{Name: "alpha", URL: "https://a"},
		{Name: "beta", URL: "https://b"},
		{Name: "gamma", URL: "https://g"},
	}
	got := FindSiteByName(sites, "beta")
	if got == nil {
		t.Fatalf("want non-nil, got nil")
	}
	if got.URL != "https://b" {
		t.Errorf("want URL=https://b, got %q", got.URL)
	}
}

// 11. 名字不存在 → 返 nil
func TestFindSiteByNameMissing(t *testing.T) {
	sites := []TargetSite{
		{Name: "alpha", URL: "https://a"},
	}
	if got := FindSiteByName(sites, "delta"); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

// 12. 大小写敏感：小写 "alpha" 找不到 "Alpha"
func TestFindSiteByNameCaseSensitive(t *testing.T) {
	sites := []TargetSite{{Name: "Alpha", URL: "https://a"}}
	if got := FindSiteByName(sites, "alpha"); got != nil {
		t.Errorf("case-sensitive: want nil, got %+v", got)
	}
}

// 13. 空 slice → 返 nil
func TestFindSiteByNameEmpty(t *testing.T) {
	if got := FindSiteByName(nil, "anything"); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

// ----- FilterSitesByQuery -----

// 14. 空 query → 返全部
func TestFilterSitesByQueryEmpty(t *testing.T) {
	sites := []TargetSite{
		{Name: "alpha", URL: "https://a"},
		{Name: "beta", URL: "https://b"},
	}
	got := FilterSitesByQuery(sites, "")
	if len(got) != 2 {
		t.Errorf("want 2 (all), got %d", len(got))
	}
}

// 14b. 空 query + nil/空 sites → 返非 nil 空 slice（避免 JSON 序列化为 null）
func TestFilterSitesByQueryEmptyReturnsNonNil(t *testing.T) {
	got := FilterSitesByQuery(nil, "")
	if got == nil {
		t.Errorf("want non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("want 0, got %d", len(got))
	}
}

// 15. 模糊子串匹配（中英文都试）
func TestFilterSitesByQuerySubstring(t *testing.T) {
	sites := []TargetSite{
		{Name: "SET上市公司名录", URL: "https://set"},
		{Name: "BOI 投资促进局", URL: "https://boi"},
		{Name: "Tokyo Stock Exchange", URL: "https://tse"},
	}
	// 中文模糊
	got := FilterSitesByQuery(sites, "上市公司")
	if len(got) != 1 || got[0].Name != "SET上市公司名录" {
		t.Errorf("chinese fuzzy: want 1 SET, got %v", got)
	}
	// 英文模糊
	got = FilterSitesByQuery(sites, "Stock")
	if len(got) != 1 || got[0].Name != "Tokyo Stock Exchange" {
		t.Errorf("english fuzzy: want 1 Tokyo, got %v", got)
	}
}

// 16. 多条匹配 → 全部返回
func TestFilterSitesByQueryMulti(t *testing.T) {
	sites := []TargetSite{
		{Name: "SET-泰国", URL: "x"},
		{Name: "SET-日本", URL: "y"},
		{Name: "BOI", URL: "z"},
	}
	got := FilterSitesByQuery(sites, "SET")
	if len(got) != 2 {
		t.Errorf("want 2, got %d: %v", len(got), got)
	}
}

// 17. 无匹配 → 返空
func TestFilterSitesByQueryNoMatch(t *testing.T) {
	sites := []TargetSite{{Name: "alpha"}}
	got := FilterSitesByQuery(sites, "zzzzz")
	if len(got) != 0 {
		t.Errorf("want empty, got %v", got)
	}
}
