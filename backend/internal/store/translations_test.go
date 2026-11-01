package store

import "testing"

func TestSplitTranslationKey(t *testing.T) {
	tests := []struct {
		entityKey string
		namespace string
		code      string
		remark    bool
		ok        bool
	}{
		{entityKey: "menu.dashboard", namespace: "menu", code: "dashboard", ok: true},
		{entityKey: "perm.system:user:create", namespace: "perm", code: "system:user:create", ok: true},
		{entityKey: "role.admin", namespace: "role", code: "admin", ok: true},
		{entityKey: "role.admin.remark", namespace: "role", code: "admin", remark: true, ok: true},
		{entityKey: "dept.root", namespace: "dept", code: "root", ok: true},
		{entityKey: "post.dev.remark", namespace: "post", code: "dev", remark: true, ok: true},
		// Codes may hold dots of their own; only the trailing .remark is special.
		{entityKey: "job.report.daily", namespace: "job", code: "report.daily", ok: true},
		{entityKey: "storage.s3", namespace: "storage", code: "s3", ok: true},
		// A code may read like a suffix; only a trailing one after a code counts.
		{entityKey: "role.remark", namespace: "role", code: "remark", ok: true},
		// Unknown namespaces split fine; writeSourceText is what refuses them.
		{entityKey: "role.", ok: false},
		{entityKey: ".admin", ok: false},
		{entityKey: "nodothere", ok: false},
	}
	for _, tt := range tests {
		namespace, code, remark, ok := splitTranslationKey(tt.entityKey)
		if namespace != tt.namespace || code != tt.code || remark != tt.remark || ok != tt.ok {
			t.Fatalf("splitTranslationKey(%q) = %q, %q, %v, %v; want %q, %q, %v, %v",
				tt.entityKey, namespace, code, remark, ok, tt.namespace, tt.code, tt.remark, tt.ok)
		}
	}
}
