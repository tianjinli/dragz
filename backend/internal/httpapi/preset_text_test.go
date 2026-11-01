package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"dragz-admin/backend/internal/store"
)

// fakePresetTexts records the store calls a preset save makes.
type fakePresetTexts struct {
	row     store.PresetSourceRow
	rekeyed []presetKeyPair
	saved   []presetSavedText
	deleted []string
}

type presetKeyPair struct{ oldKey, newKey string }
type presetSavedText struct {
	entityKey string
	lang      string
	value     string
}

func (f *fakePresetTexts) PresetSourceRow(_ context.Context, namespace string, id uint64) (store.PresetSourceRow, error) {
	if namespace == "" || id == 0 {
		return store.PresetSourceRow{}, store.ErrSourceEntityNotFound
	}
	return f.row, nil
}

func (f *fakePresetTexts) RekeyEntityTranslations(_ context.Context, oldKey, newKey string) error {
	f.rekeyed = append(f.rekeyed, presetKeyPair{oldKey: oldKey, newKey: newKey})
	return nil
}

func (f *fakePresetTexts) CreateTranslation(_ context.Context, entityKey, lang, value string) error {
	f.saved = append(f.saved, presetSavedText{entityKey: entityKey, lang: lang, value: value})
	return nil
}

func (f *fakePresetTexts) DeleteTranslationForEntity(_ context.Context, entityKey, lang string) (bool, error) {
	f.deleted = append(f.deleted, entityKey+"|"+lang)
	return true, nil
}

// presetRequest is a save made in one language: the store routing reads the
// language off the request, exactly like the entity endpoints do.
func presetRequest(language string) *http.Request {
	r := httptest.NewRequest(http.MethodPut, "/api/system/roles/7", nil)
	r.Header.Set("Accept-Language", language)
	return r
}

// roleEdit is the translatable half of a role save under test.
func roleEdit(id uint64, code, name, remark string) presetTextEdit {
	return presetTextEdit{
		Namespace: "role", Prefix: "role", ID: id,
		Code: code, Name: name, Remark: remark, HasRemark: true,
	}
}

func TestPresetSaveInAnotherLanguageKeepsSourceColumns(t *testing.T) {
	auth := &fakePresetTexts{row: store.PresetSourceRow{Code: "admin", Name: "管理员", Remark: "内置角色"}}
	save, err := preparePresetSave(auth, presetRequest("en-US"), roleEdit(7, "admin", "Administrator", ""))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if save.SourceName() != "管理员" {
		t.Fatalf("SourceName() = %q, want the Chinese text the row already held", save.SourceName())
	}
	if save.SourceRemark() != "内置角色" {
		t.Fatalf("SourceRemark() = %q, want the stored remark", save.SourceRemark())
	}
	if err := save.Apply(context.Background()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(auth.rekeyed) != 0 {
		t.Fatalf("unexpected rekey with an unchanged code: %v", auth.rekeyed)
	}
	want := presetSavedText{entityKey: "role.admin", lang: "en", value: "Administrator"}
	if len(auth.saved) != 1 || auth.saved[0] != want {
		t.Fatalf("saved = %v, want just %+v", auth.saved, want)
	}
	// The remark was cleared, so its record goes and the Chinese shows again.
	if len(auth.deleted) != 1 || auth.deleted[0] != "role.admin.remark|en" {
		t.Fatalf("deleted = %v, want the English remark", auth.deleted)
	}
}

func TestPresetSaveInChineseWritesOnlyColumns(t *testing.T) {
	auth := &fakePresetTexts{row: store.PresetSourceRow{Code: "admin", Name: "Administrator", Remark: "Built in"}}
	save, err := preparePresetSave(auth, presetRequest("zh-CN"), roleEdit(7, "admin", "管理员", "内置角色"))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if save.SourceName() != "管理员" || save.SourceRemark() != "内置角色" {
		t.Fatalf("columns = %q, %q; want what the operator typed", save.SourceName(), save.SourceRemark())
	}
	if err := save.Apply(context.Background()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(auth.saved) != 0 || len(auth.deleted) != 0 {
		t.Fatalf("a source-language save must not touch sys_translation: %v %v", auth.saved, auth.deleted)
	}
}

func TestPresetSaveRefilesRecordsWhenCodeChanges(t *testing.T) {
	auth := &fakePresetTexts{row: store.PresetSourceRow{Code: "admin", Name: "管理员"}}
	save, err := preparePresetSave(auth, presetRequest("en-US"), roleEdit(7, "superadmin", "Super Admin", ""))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := save.Apply(context.Background()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := []presetKeyPair{
		{oldKey: "role.admin", newKey: "role.superadmin"},
		{oldKey: "role.admin.remark", newKey: "role.superadmin.remark"},
	}
	if len(auth.rekeyed) != len(want) {
		t.Fatalf("rekeyed = %v, want %v", auth.rekeyed, want)
	}
	for i, pair := range want {
		if auth.rekeyed[i] != pair {
			t.Fatalf("rekeyed[%d] = %v, want %v", i, auth.rekeyed[i], pair)
		}
	}
	// The new text is filed under the new code, not the one it started under.
	if len(auth.saved) != 1 || auth.saved[0].entityKey != "role.superadmin" {
		t.Fatalf("saved = %v, want the text under role.superadmin", auth.saved)
	}
}

func TestPresetSaveOnCreateKeepsTypedTextInColumns(t *testing.T) {
	auth := &fakePresetTexts{}
	// A create has no row to read, so the columns hold what was typed and the
	// translation record is written next to it: both languages say something.
	save, err := preparePresetSave(auth, presetRequest("en-US"), roleEdit(0, "auditor", "Auditor", ""))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if save.SourceName() != "Auditor" {
		t.Fatalf("SourceName() = %q, want the submitted text on a create", save.SourceName())
	}
	if err := save.Apply(context.Background()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(auth.rekeyed) != 0 {
		t.Fatalf("a create has no previous key: %v", auth.rekeyed)
	}
	want := presetSavedText{entityKey: "role.auditor", lang: "en", value: "Auditor"}
	if len(auth.saved) != 1 || auth.saved[0] != want {
		t.Fatalf("saved = %v, want %+v", auth.saved, want)
	}
}

func TestPresetSaveWithoutRemarkTouchesOneKey(t *testing.T) {
	auth := &fakePresetTexts{row: store.PresetSourceRow{Code: "root", Name: "总部"}}
	edit := presetTextEdit{Namespace: "dept", Prefix: "dept", ID: 3, Code: "head", Name: "Head Office"}
	save, err := preparePresetSave(auth, presetRequest("en-US"), edit)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := save.Apply(context.Background()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(auth.rekeyed) != 1 || auth.rekeyed[0] != (presetKeyPair{oldKey: "dept.root", newKey: "dept.head"}) {
		t.Fatalf("rekeyed = %v, want only dept.root moved", auth.rekeyed)
	}
	if len(auth.saved) != 1 || auth.saved[0].entityKey != "dept.head" {
		t.Fatalf("saved = %v, want one text under dept.head", auth.saved)
	}
}
