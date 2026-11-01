package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// Department is one sys_department row. Ancestors is the materialised path of parent IDs, which is what makes subtree queries a prefix match.
type Department struct {
	ID           uint64    `json:"id"`
	ParentID     uint64    `json:"parentId"`
	Ancestors    string    `json:"ancestors"`
	AncestorPath string    `json:"ancestorPath"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	LeaderUserID *uint64   `json:"leaderUserId"`
	LeaderName   string    `json:"leaderName"`
	Sort         int       `json:"sort"`
	Status       int8      `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

// SaveDepartmentInput is the payload shared by create and update.
type SaveDepartmentInput struct {
	ParentID     uint64
	Code         string
	Name         string
	LeaderUserID *uint64
	Sort         int
	Status       int8
}

// Position is one sys_position row together with its user count.
type Position struct {
	ID        uint64    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Sort      int       `json:"sort"`
	Status    int8      `json:"status"`
	Remark    string    `json:"remark"`
	UserCount int64     `json:"userCount"`
	CreatedAt time.Time `json:"createdAt"`
}

// SavePositionInput is the payload shared by create and update.
type SavePositionInput struct {
	Code   string
	Name   string
	Sort   int
	Status int8
	Remark string
}

// ListDepartments returns the whole department tree flat, with a human-readable ancestor path for the tree column.
func (r *AuthRepository) ListDepartments(ctx context.Context, keyword string, status *int8) ([]Department, error) {
	rows, err := r.queries.ListDepartmentsFiltered(ctx, sqlcstore.ListDepartmentsFilteredParams{
		KeywordPattern: optionalPattern(keyword), Status: optionalInt16(status),
	})
	if err != nil {
		return nil, err
	}
	items := make([]Department, 0, len(rows))
	for _, row := range rows {
		items = append(items, Department{
			ID: uint64(row.ID), ParentID: row.ParentID, Ancestors: row.Ancestors, Code: row.Code, Name: row.Name,
			LeaderUserID: nullInt64Ptr(row.LeaderUserID), LeaderName: row.LeaderName,
			Sort: int(row.Sort), Status: int8(row.Status), CreatedAt: row.CreatedAt,
		})
	}
	nameByID, err := r.departmentNameIndex(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].AncestorPath = formatDepartmentAncestorPath(items[i].Ancestors, nameByID)
	}
	return items, nil
}

// CreateDepartment derives the ancestors path from the parent and inserts the row.
func (r *AuthRepository) CreateDepartment(ctx context.Context, input SaveDepartmentInput) (uint64, error) {
	ancestors, err := r.departmentAncestors(ctx, input.ParentID)
	if err != nil {
		return 0, err
	}
	id, err := r.queries.CreateDepartment(ctx, sqlcstore.CreateDepartmentParams{
		ParentID: input.ParentID, Ancestors: ancestors, Code: input.Code, Name: input.Name,
		LeaderUserID: ptrUint64Null(input.LeaderUserID), Sort: int32(input.Sort), Status: int16(input.Status),
	})
	return uint64(id), wrapDuplicate(err)
}

// UpdateDepartment rewrites a department and, when it moved, rewrites the ancestors of its whole subtree in the same transaction. Disabling a department also disables its descendants, so no active child can hang under an inactive parent.
func (r *AuthRepository) UpdateDepartment(ctx context.Context, departmentID uint64, input SaveDepartmentInput) error {
	info, err := r.queries.GetDepartmentParentInfo(ctx, int64(departmentID))
	if err != nil {
		return err
	}
	oldParentID := info.ParentID
	oldAncestors := info.Ancestors
	if departmentID == 1 && input.ParentID != 0 {
		return ErrDepartmentProtected
	}
	if departmentID == input.ParentID {
		return ErrDepartmentCycle
	}
	newAncestors, err := r.departmentAncestors(ctx, input.ParentID)
	if err != nil {
		return err
	}
	if containsAncestor(newAncestors, departmentID) {
		return ErrDepartmentCycle
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	affected, err := q.UpdateDepartment(ctx, sqlcstore.UpdateDepartmentParams{
		ParentID: input.ParentID, Ancestors: newAncestors, Code: input.Code, Name: input.Name,
		LeaderUserID: ptrUint64Null(input.LeaderUserID), Sort: int32(input.Sort), Status: int16(input.Status),
		ID: int64(departmentID),
	})
	if err != nil {
		return wrapDuplicate(err)
	}
	if affected != 1 {
		return ErrNotFound
	}
	if oldParentID != input.ParentID {
		oldPrefix := fmt.Sprintf("%s,%d", oldAncestors, departmentID)
		newPrefix := fmt.Sprintf("%s,%d", newAncestors, departmentID)
		if err := q.RewriteDepartmentAncestorsPrefix(ctx, sqlcstore.RewriteDepartmentAncestorsPrefixParams{
			Column1: newPrefix, Column2: oldPrefix, Ancestors: oldPrefix, Column4: oldPrefix,
		}); err != nil {
			return err
		}
	}
	if input.Status != 1 {
		prefix := fmt.Sprintf("%s,%d", newAncestors, departmentID)
		if err := q.DisableDepartmentDescendants(ctx, sqlcstore.DisableDepartmentDescendantsParams{
			Ancestors: prefix, Column2: prefix,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpdateDepartmentStatus enables or disables a department. Disabling cascades to the subtree.
func (r *AuthRepository) UpdateDepartmentStatus(ctx context.Context, departmentID uint64, status int8) error {
	info, err := r.queries.GetDepartmentParentInfo(ctx, int64(departmentID))
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	affected, err := q.UpdateDepartmentStatus(ctx, sqlcstore.UpdateDepartmentStatusParams{
		Status: int16(status), ID: int64(departmentID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	if status != 1 {
		prefix := fmt.Sprintf("%s,%d", info.Ancestors, departmentID)
		if err := q.DisableDepartmentDescendants(ctx, sqlcstore.DisableDepartmentDescendantsParams{
			Ancestors: prefix, Column2: prefix,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteDepartment soft deletes a leaf department that has neither children nor users.
func (r *AuthRepository) DeleteDepartment(ctx context.Context, departmentID uint64) error {
	if departmentID == 1 {
		return ErrDepartmentProtected
	}
	counts, err := r.queries.CountDepartmentChildrenAndUsers(ctx, sqlcstore.CountDepartmentChildrenAndUsersParams{
		ParentID: departmentID, DepartmentID: sql.NullInt64{Int64: int64(departmentID), Valid: true},
	})
	if err != nil {
		return err
	}
	if counts.ChildCount > 0 || counts.UserCount > 0 {
		return ErrDepartmentInUse
	}
	affected, err := r.queries.SoftDeleteDepartment(ctx, int64(departmentID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

func (r *AuthRepository) departmentAncestors(ctx context.Context, parentID uint64) (string, error) {
	if parentID == 0 {
		return "0", nil
	}
	row, err := r.queries.GetDepartmentAncestorsStatus(ctx, int64(parentID))
	if err != nil {
		return "", err
	}
	if row.Status != 1 {
		return "", ErrDepartmentParentDisabled
	}
	return fmt.Sprintf("%s,%d", row.Ancestors, parentID), nil
}

func containsAncestor(ancestors string, departmentID uint64) bool {
	target := fmt.Sprintf("%d", departmentID)
	for _, value := range strings.Split(ancestors, ",") {
		if value == target {
			return true
		}
	}
	return false
}

func (r *AuthRepository) departmentNameIndex(ctx context.Context) (map[uint64]string, error) {
	rows, err := r.queries.ListDepartmentNames(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[uint64]string, len(rows))
	for _, row := range rows {
		names[uint64(row.ID)] = row.Name
	}
	return names, nil
}

func formatDepartmentAncestorPath(ancestors string, nameByID map[uint64]string) string {
	segments := strings.Split(ancestors, ",")
	names := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" || segment == "0" {
			continue
		}
		id, err := strconv.ParseUint(segment, 10, 64)
		if err != nil {
			continue
		}
		if name, ok := nameByID[id]; ok {
			names = append(names, name)
		}
	}
	return strings.Join(names, " / ")
}

// ListPositions returns one page of positions plus the total count.
func (r *AuthRepository) ListPositions(ctx context.Context, keyword string, status *int8, page, pageSize int) ([]Position, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	pattern := optionalPattern(keyword)
	st := optionalInt16(status)
	total, err := r.queries.CountPositionsFiltered(ctx, sqlcstore.CountPositionsFilteredParams{
		KeywordPattern: pattern, Status: st,
	})
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListPositionsFiltered(ctx, sqlcstore.ListPositionsFilteredParams{
		Limit: int32(pageSize), Offset: int32((page - 1) * pageSize),
		KeywordPattern: pattern, Status: st,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]Position, 0, len(rows))
	for _, row := range rows {
		items = append(items, Position{
			ID: uint64(row.ID), Code: row.Code, Name: row.Name, Sort: int(row.Sort),
			Status: int8(row.Status), Remark: row.Remark, UserCount: row.UserCount, CreatedAt: row.CreatedAt,
		})
	}
	return items, total, nil
}

// CreatePosition inserts a position and reports a code clash as ErrDuplicateEntry.
func (r *AuthRepository) CreatePosition(ctx context.Context, input SavePositionInput) (uint64, error) {
	id, err := r.queries.CreatePosition(ctx, sqlcstore.CreatePositionParams{
		Code: input.Code, Name: input.Name, Sort: int32(input.Sort), Status: int16(input.Status), Remark: input.Remark,
	})
	return uint64(id), wrapDuplicate(err)
}

// UpdatePosition rewrites a position and reports a code clash as ErrDuplicateEntry.
func (r *AuthRepository) UpdatePosition(ctx context.Context, positionID uint64, input SavePositionInput) error {
	affected, err := r.queries.UpdatePosition(ctx, sqlcstore.UpdatePositionParams{
		Code: input.Code, Name: input.Name, Sort: int32(input.Sort), Status: int16(input.Status),
		Remark: input.Remark, ID: int64(positionID),
	})
	if err != nil {
		return wrapDuplicate(err)
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// DeletePosition soft deletes a position that no user holds.
func (r *AuthRepository) DeletePosition(ctx context.Context, positionID uint64) error {
	count, err := r.queries.CountPositionUsers(ctx, positionID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrPositionInUse
	}
	affected, err := r.queries.SoftDeletePosition(ctx, int64(positionID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// UpdatePositionStatus enables or disables a position.
func (r *AuthRepository) UpdatePositionStatus(ctx context.Context, positionID uint64, status int8) error {
	affected, err := r.queries.UpdatePositionStatus(ctx, sqlcstore.UpdatePositionStatusParams{
		Status: int16(status), ID: int64(positionID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}
