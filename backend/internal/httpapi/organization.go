package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/store"
)

type saveDepartmentRequest struct {
	ParentID     uint64  `json:"parentId"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	LeaderUserID *uint64 `json:"leaderUserId"`
	Sort         int     `json:"sort"`
	Status       int8    `json:"status"`
}

type savePositionRequest struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Sort   int    `json:"sort"`
	Status int8   `json:"status"`
	Remark string `json:"remark"`
}

func listDepartmentsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, valid := optionalStatus(r.URL.Query().Get("status"))
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dept_status_invalid")
			return
		}
		items, err := ctx.Auth.ListDepartments(r.Context(), r.URL.Query().Get("keyword"), status)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.dept_list_failed")
			return
		}
		items = localizePreset(r, ctx.Auth, items,
			func(d store.Department) []string { return []string{"dept." + d.Code} },
			func(d *store.Department, v map[string]string) {
				if s, ok := v["dept."+d.Code]; ok {
					d.Name = s
				}
			})
		ok(w, r, items)
	}
}

func createDepartmentHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveDepartmentRequest
		if !decode(w, r, &req) || !validateDepartment(w, r, req) {
			return
		}
		input := departmentInput(req)
		save, err := preparePresetSave(ctx.Auth, r, presetTextEdit{
			Namespace: "dept", Prefix: "dept", Code: input.Code, Name: input.Name,
		})
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		id, err := ctx.Auth.CreateDepartment(r.Context(), input)
		if err != nil {
			writeOrganizationError(w, r, err, "subjects.department")
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, map[string]uint64{"id": id})
	}
}

func updateDepartmentHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dept_id_invalid")
			return
		}
		var req saveDepartmentRequest
		if !decode(w, r, &req) || !validateDepartment(w, r, req) {
			return
		}
		input := departmentInput(req)
		save, err := preparePresetSave(ctx.Auth, r, presetTextEdit{
			Namespace: "dept", Prefix: "dept", ID: id, Code: input.Code, Name: input.Name,
		})
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		input.Name = save.SourceName()
		if err := ctx.Auth.UpdateDepartment(r.Context(), id, input); err != nil {
			writeOrganizationError(w, r, err, "subjects.department")
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func departmentInput(req saveDepartmentRequest) store.SaveDepartmentInput {
	return store.SaveDepartmentInput{
		ParentID: req.ParentID, Code: strings.TrimSpace(req.Code), Name: strings.TrimSpace(req.Name),
		LeaderUserID: req.LeaderUserID, Sort: req.Sort, Status: req.Status,
	}
}

func deleteDepartmentHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dept_id_invalid")
			return
		}
		if err := ctx.Auth.DeleteDepartment(r.Context(), id); err != nil {
			writeOrganizationError(w, r, err, "subjects.department")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func updateDepartmentStatusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dept_id_invalid")
			return
		}
		var req updateStatusRequest
		if !decode(w, r, &req) || !validStatus(req.Status) {
			fail(w, r, http.StatusBadRequest, 40001, "errors.dept_status_enum")
			return
		}
		if err := ctx.Auth.UpdateDepartmentStatus(r.Context(), id, req.Status); err != nil {
			writeOrganizationError(w, r, err, "subjects.department")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func listPositionsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		status, valid := optionalStatus(query.Get("status"))
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.position_status_invalid")
			return
		}
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		items, total, err := ctx.Auth.ListPositions(r.Context(), query.Get("keyword"), status, page, pageSize)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.position_list_failed")
			return
		}
		items = localizePreset(r, ctx.Auth, items,
			func(p store.Position) []string {
				return []string{"post." + p.Code, "post." + p.Code + ".remark"}
			},
			func(p *store.Position, v map[string]string) {
				if s, ok := v["post."+p.Code]; ok {
					p.Name = s
				}
				if s, ok := v["post."+p.Code+".remark"]; ok {
					p.Remark = s
				}
			})
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

func createPositionHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req savePositionRequest
		if !decode(w, r, &req) || !validatePosition(w, r, req) {
			return
		}
		input := positionInput(req)
		save, err := preparePresetSave(ctx.Auth, r, positionPresetEdit(0, input))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		id, err := ctx.Auth.CreatePosition(r.Context(), input)
		if err != nil {
			writeOrganizationError(w, r, err, "subjects.position")
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, map[string]uint64{"id": id})
	}
}

func updatePositionHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.position_id_invalid")
			return
		}
		var req savePositionRequest
		if !decode(w, r, &req) || !validatePosition(w, r, req) {
			return
		}
		input := positionInput(req)
		save, err := preparePresetSave(ctx.Auth, r, positionPresetEdit(id, input))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		input.Name, input.Remark = save.SourceName(), save.SourceRemark()
		if err := ctx.Auth.UpdatePosition(r.Context(), id, input); err != nil {
			writeOrganizationError(w, r, err, "subjects.position")
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

// positionPresetEdit describes a position save: like a role, it translates both
// its name and its remark.
func positionPresetEdit(id uint64, input store.SavePositionInput) presetTextEdit {
	return presetTextEdit{
		Namespace: "post", Prefix: "post", ID: id,
		Code: input.Code, Name: input.Name, Remark: input.Remark, HasRemark: true,
	}
}

func deletePositionHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.position_id_invalid")
			return
		}
		if err := ctx.Auth.DeletePosition(r.Context(), id); err != nil {
			writeOrganizationError(w, r, err, "subjects.position")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func updatePositionStatusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.position_id_invalid")
			return
		}
		var req updateStatusRequest
		if !decode(w, r, &req) || !validStatus(req.Status) {
			fail(w, r, http.StatusBadRequest, 40001, "errors.position_status_enum")
			return
		}
		if err := ctx.Auth.UpdatePositionStatus(r.Context(), id, req.Status); err != nil {
			writeOrganizationError(w, r, err, "subjects.position")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func validateDepartment(w http.ResponseWriter, r *http.Request, req saveDepartmentRequest) bool {
	if strings.TrimSpace(req.Code) == "" || len(req.Code) > 64 || strings.TrimSpace(req.Name) == "" || len(req.Name) > 128 {
		fail(w, r, http.StatusBadRequest, 40001, "errors.dept_fields_invalid")
		return false
	}
	if req.Status != 0 && req.Status != 1 {
		fail(w, r, http.StatusBadRequest, 40001, "errors.dept_status_enum")
		return false
	}
	return true
}

func validatePosition(w http.ResponseWriter, r *http.Request, req savePositionRequest) bool {
	if strings.TrimSpace(req.Code) == "" || len(req.Code) > 64 || strings.TrimSpace(req.Name) == "" || len(req.Name) > 128 {
		fail(w, r, http.StatusBadRequest, 40001, "errors.position_fields_invalid")
		return false
	}
	if req.Status != 0 && req.Status != 1 {
		fail(w, r, http.StatusBadRequest, 40001, "errors.position_status_enum")
		return false
	}
	return true
}

func positionInput(req savePositionRequest) store.SavePositionInput {
	return store.SavePositionInput{
		Code: strings.TrimSpace(req.Code), Name: strings.TrimSpace(req.Name), Sort: req.Sort,
		Status: req.Status, Remark: strings.TrimSpace(req.Remark),
	}
}

func optionalStatus(value string) (*int8, bool) {
	if strings.TrimSpace(value) == "" {
		return nil, true
	}
	parsed, err := strconv.ParseInt(value, 10, 8)
	if err != nil || (parsed != 0 && parsed != 1) {
		return nil, false
	}
	status := int8(parsed)
	return &status, true
}

// writeOrganizationError maps a department or position error onto the org
// message family. Classification is by sentinel only: matching on the error
// text would break the moment a message is reworded.
func writeOrganizationError(w http.ResponseWriter, r *http.Request, err error, subjectKey string) {
	data := map[string]any{"Subject": localized(r, subjectKey)}
	switch {
	case errors.Is(err, store.ErrDuplicateEntry):
		fail(w, r, http.StatusConflict, 40903, "errors.org.code_exists", data)
	case errors.Is(err, store.ErrNotFound) || errors.Is(err, sql.ErrNoRows):
		fail(w, r, http.StatusNotFound, 40401, "errors.org.not_found", data)
	case errors.Is(err, store.ErrDepartmentInUse),
		errors.Is(err, store.ErrPositionInUse),
		errors.Is(err, store.ErrInUse),
		errors.Is(err, store.ErrHasChildren):
		fail(w, r, http.StatusConflict, 40904, "errors.org.in_use", data)
	case errors.Is(err, store.ErrDepartmentProtected),
		errors.Is(err, store.ErrDepartmentCycle),
		errors.Is(err, store.ErrDepartmentParentDisabled),
		errors.Is(err, store.ErrHasDescendant),
		errors.Is(err, store.ErrProtected):
		fail(w, r, http.StatusConflict, 40905, "errors.org.invalid_relation", data)
	default:
		fail(w, r, http.StatusInternalServerError, 50000, "errors.org.failed", data)
	}
}
