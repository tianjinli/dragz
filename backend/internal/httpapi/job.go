package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/jobs"
	"dragz-admin/backend/internal/store"
)

var jobCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{2,127}$`)

type saveJobRequest struct {
	Name           string          `json:"name"`
	Code           string          `json:"code"`
	Handler        string          `json:"handler"`
	CronExpression string          `json:"cronExpression"`
	Payload        json.RawMessage `json:"payload"`
	TimeoutSeconds int             `json:"timeoutSeconds"`
	MaxRetries     int             `json:"maxRetries"`
	Status         int8            `json:"status"`
}

func listJobsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := ctx.Auth.ListJobs(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.job_list_failed")
			return
		}
		items = localizePreset(r, ctx.Auth, items,
			func(j store.ScheduledJob) []string { return []string{"job." + j.Code} },
			func(j *store.ScheduledJob, v map[string]string) {
				if s, ok := v["job."+j.Code]; ok {
					j.Name = s
				}
			})
		ok(w, r, map[string]any{"items": items, "handlers": jobs.AllowedHandlers})
	}
}

func createJobHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveJobRequest
		if !decode(w, r, &req) {
			return
		}
		input, valid := validateJob(w, r, req)
		if !valid {
			return
		}
		save, err := preparePresetSave(ctx.Auth, r, jobPresetEdit(0, input))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		id, err := ctx.Auth.CreateJob(r.Context(), input)
		if err != nil {
			fail(w, r, http.StatusConflict, 40901, "errors.job_save_conflict")
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		_ = ctx.Jobs.RequestReconcile(r.Context())
		ok(w, r, map[string]uint64{"id": id})
	}
}

func updateJobHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.job_id_invalid")
			return
		}
		var req saveJobRequest
		if !decode(w, r, &req) {
			return
		}
		input, valid := validateJob(w, r, req)
		if !valid {
			return
		}
		save, err := preparePresetSave(ctx.Auth, r, jobPresetEdit(id, input))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		input.Name = save.SourceName()
		if err := ctx.Auth.UpdateJob(r.Context(), id, input); err != nil {
			fail(w, r, http.StatusConflict, 40901, "errors.job_update_conflict")
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		_ = ctx.Jobs.RequestReconcile(r.Context())
		ok(w, r, map[string]bool{"success": true})
	}
}

// jobPresetEdit describes a scheduled job save: only its name is translatable.
func jobPresetEdit(id uint64, input store.SaveJobInput) presetTextEdit {
	return presetTextEdit{
		Namespace: "job", Prefix: "job", ID: id, Code: input.Code, Name: input.Name,
	}
}

func deleteJobHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.job_id_invalid")
			return
		}
		if err := ctx.Auth.DeleteJob(r.Context(), id); err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.job_not_found")
			return
		}
		_ = ctx.Jobs.RequestReconcile(r.Context())
		ok(w, r, map[string]bool{"success": true})
	}
}

func runJobHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.job_id_invalid")
			return
		}
		result, err := ctx.Jobs.RunNow(r.Context(), id)
		if err != nil {
			if errors.Is(err, jobs.ErrNoWorker) {
				fail(w, r, http.StatusServiceUnavailable, 50301, "errors.job_runner_unavailable")
				return
			}
			// A disabled scheduler is a configuration state, not a conflict.
			if errors.Is(err, jobs.ErrSchedulerDisabled) {
				fail(w, r, http.StatusServiceUnavailable, 50301, "errors.job_scheduler_disabled")
				return
			}
			fail(w, r, http.StatusConflict, 40901, "errors.job_run_failed")
			return
		}
		ok(w, r, result)
	}
}

func listJobLogsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.job_id_invalid")
			return
		}
		query := r.URL.Query()
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		items, total, err := ctx.Auth.ListJobLogs(r.Context(), id, page, pageSize)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.job_logs_failed")
			return
		}
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

func validateJob(w http.ResponseWriter, r *http.Request, req saveJobRequest) (store.SaveJobInput, bool) {
	req.Name, req.Code, req.Handler = strings.TrimSpace(req.Name), strings.TrimSpace(req.Code), strings.TrimSpace(req.Handler)
	if req.Name == "" || len(req.Name) > 128 || !jobCodePattern.MatchString(req.Code) || !jobs.HandlerAllowed(req.Handler) {
		fail(w, r, http.StatusBadRequest, 40001, "errors.job_fields_invalid")
		return store.SaveJobInput{}, false
	}
	schedule, err := cron.ParseStandard(strings.TrimSpace(req.CronExpression))
	if err != nil {
		fail(w, r, http.StatusBadRequest, 40002, "errors.job_cron_invalid")
		return store.SaveJobInput{}, false
	}
	if len(req.Payload) == 0 || string(req.Payload) == "null" {
		req.Payload = json.RawMessage(`{}`)
	}
	var payload map[string]any
	if !json.Valid(req.Payload) || json.Unmarshal(req.Payload, &payload) != nil {
		fail(w, r, http.StatusBadRequest, 40003, "errors.job_params_invalid")
		return store.SaveJobInput{}, false
	}
	if req.TimeoutSeconds < 1 || req.TimeoutSeconds > 3600 || req.MaxRetries < 0 || req.MaxRetries > 10 || (req.Status != 0 && req.Status != 1) {
		fail(w, r, http.StatusBadRequest, 40004, "errors.job_options_invalid")
		return store.SaveJobInput{}, false
	}
	var nextRunAt *time.Time
	if req.Status == 1 {
		value := schedule.Next(time.Now())
		nextRunAt = &value
	}
	return store.SaveJobInput{
		Name: req.Name, Code: req.Code, Handler: req.Handler,
		CronExpression: strings.TrimSpace(req.CronExpression), Payload: req.Payload,
		TimeoutSeconds: req.TimeoutSeconds, MaxRetries: req.MaxRetries,
		Status: req.Status, NextRunAt: nextRunAt,
	}, true
}
