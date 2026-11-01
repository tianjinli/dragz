package httpapi

import (
	"net/url"
	"testing"
)

func TestExtensionAllowed(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		allowed   string
		want      bool
	}{
		{name: "empty allow list permits everything", extension: "exe", allowed: "", want: true},
		{name: "whitespace allow list permits everything", extension: "exe", allowed: "   ", want: true},
		{name: "exact match", extension: "png", allowed: "jpg,png", want: true},
		{name: "case insensitive", extension: "png", allowed: "JPG,PNG", want: true},
		{name: "leading dot and spaces", extension: "png", allowed: " .jpg , .png ", want: true},
		{name: "not allowed", extension: "exe", allowed: "jpg,png", want: false},
		{name: "empty extension rejected when allow list set", extension: "", allowed: "jpg,png", want: false},
		{name: "single entry", extension: "pdf", allowed: "pdf", want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := extensionAllowed(test.extension, test.allowed); got != test.want {
				t.Fatalf("extensionAllowed(%q, %q) = %v, want %v", test.extension, test.allowed, got, test.want)
			}
		})
	}
}

func TestFileListFilterFromQuery(t *testing.T) {
	tests := []struct {
		name         string
		rawQuery     string
		page         int
		pageSize     int
		wantStatus   string
		wantExclude  string
		wantKeyword  string
		wantMime     string
		wantUploader uint64
		wantPage     int
		wantPageSize int
	}{
		{
			name: "no filters", page: 1, pageSize: 20,
			wantPage: 1, wantPageSize: 20,
		},
		{
			name: "recycled true lists only recycled", rawQuery: "recycled=true",
			page: 1, pageSize: 20, wantStatus: "recycled", wantPage: 1, wantPageSize: 20,
		},
		{
			name: "recycled false excludes recycled", rawQuery: "recycled=false",
			page: 1, pageSize: 20, wantExclude: "recycled", wantPage: 1, wantPageSize: 20,
		},
		{
			name: "invalid recycled ignored", rawQuery: "recycled=maybe",
			page: 1, pageSize: 20, wantPage: 1, wantPageSize: 20,
		},
		{
			name: "explicit status preserved", rawQuery: "status=ready",
			page: 1, pageSize: 20, wantStatus: "ready", wantPage: 1, wantPageSize: 20,
		},
		{
			name: "recycled overrides status", rawQuery: "status=ready&recycled=true",
			page: 1, pageSize: 20, wantStatus: "recycled", wantPage: 1, wantPageSize: 20,
		},
		{
			name: "all filters", rawQuery: "keyword=logo&mimeType=image/&uploaderId=7&recycled=false",
			page: 2, pageSize: 50, wantKeyword: "logo", wantMime: "image/", wantUploader: 7,
			wantExclude: "recycled", wantPage: 2, wantPageSize: 50,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values, err := url.ParseQuery(test.rawQuery)
			if err != nil {
				t.Fatal(err)
			}
			filter := fileListFilterFromQuery(values, test.page, test.pageSize)
			if filter.Status != test.wantStatus {
				t.Errorf("Status = %q, want %q", filter.Status, test.wantStatus)
			}
			if filter.ExcludeStatus != test.wantExclude {
				t.Errorf("ExcludeStatus = %q, want %q", filter.ExcludeStatus, test.wantExclude)
			}
			if filter.Keyword != test.wantKeyword {
				t.Errorf("Keyword = %q, want %q", filter.Keyword, test.wantKeyword)
			}
			if filter.MimeType != test.wantMime {
				t.Errorf("MimeType = %q, want %q", filter.MimeType, test.wantMime)
			}
			if filter.UploaderID != test.wantUploader {
				t.Errorf("UploaderID = %d, want %d", filter.UploaderID, test.wantUploader)
			}
			if filter.Page != test.wantPage {
				t.Errorf("Page = %d, want %d", filter.Page, test.wantPage)
			}
			if filter.PageSize != test.wantPageSize {
				t.Errorf("PageSize = %d, want %d", filter.PageSize, test.wantPageSize)
			}
		})
	}
}
