package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"dragz-admin/backend/internal/app"
	pkgi18n "dragz-admin/backend/internal/pkg/i18n"
	"dragz-admin/backend/internal/pkg/translate"
	"dragz-admin/backend/internal/store"
)

// The translation management page: the preset-data texts of every language the
// product ships, edited side by side.
//
// Scope is preset data only — menu names, permission button labels, roles,
// departments, positions, jobs and storage configs, i.e. the rows migrations
// and plugin seeds write. Interface copy (buttons, form labels, hints) is *not*
// here: it ships with the binary as message packs (locale/*.json plus the
// admin's locales), so changing it is a code change and every install gets it.
//
// One row is one entity, listed with the Chinese text its owning table holds and
// the primary target language from sys_translation (pkg/i18n.TargetLanguage).
// Both columns are editable in the same save, so the page no longer follows the
// interface language: an English operator sees the Chinese source too, and can
// fix it without switching the UI. Chinese writes the owning column, the target
// writes a sys_translation row, and clearing the target deletes that row so the
// entity falls back to Chinese.

type saveTranslationRequest struct {
	EntityKey string `json:"entityKey"`
	Lang      string `json:"lang"`
	// Source is the Chinese text of the same entity. When the field is present
	// (even empty) the request is a two-column save from the translation page:
	// the source is written to the owning table and Value to the primary target
	// language in one call. Absent means the older single-language save, where
	// the request language decides which column is written.
	Source *string `json:"source"`
	Value  string  `json:"value"`
}

// isSourceLang reports the language preset data is authored in. Those records
// are not rows of sys_translation: they are the owning table's text column.
func isSourceLang(lang string) bool {
	return lang == pkgi18n.SourceLanguage
}

// translationLang decides which language a translation request is about: the
// lang the client names when it names one, otherwise the language the request
// itself is written in. That is the same rule the entity pages use — the text
// you submit belongs to the language you are looking at — so the admin page
// needs no language picker of its own: it follows the interface language.
func translationLang(r *http.Request, raw string) (string, string) {
	lang := strings.TrimSpace(raw)
	if lang == "" {
		lang = pkgi18n.Lang(r)
	}
	code, ok := pkgi18n.Resolve(lang)
	if !ok {
		return "", "errors.translation_lang_unsupported"
	}
	return code, ""
}

// translationPageLang is the language the translation management page edits:
// the primary target, never the interface language. The page shows the Chinese
// source next to this language, so an operator working in Chinese who adds an
// English text must not have "zh" decide which column is written — that would
// overwrite the source instead of creating a translation.
func translationPageLang(raw string) (string, string) {
	lang := strings.TrimSpace(raw)
	if lang == "" {
		lang = pkgi18n.TargetLanguage()
	}
	if lang == "" {
		return "", "errors.translation_lang_unsupported"
	}
	code, ok := pkgi18n.Resolve(lang)
	if !ok {
		return "", "errors.translation_lang_unsupported"
	}
	return code, ""
}

// saveSourceText writes one Chinese record: the entity is addressed by its key,
// and an unknown or already deleted entity is reported the way a missing
// translation record is.
func saveSourceText(w http.ResponseWriter, r *http.Request, ctx *app.Context, req saveTranslationRequest, value string) {
	if req.EntityKey == "" {
		fail(w, r, http.StatusBadRequest, 40000, "errors.translation_invalid_request")
		return
	}
	if err := ctx.Auth.SetSourceText(r.Context(), req.EntityKey, value); err != nil {
		switch {
		case errors.Is(err, store.ErrUnknownEntityKey), errors.Is(err, store.ErrSourceEntityNotFound):
			fail(w, r, http.StatusNotFound, 40400, "errors.translation_not_found")
		default:
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
		}
		return
	}
	ok(w, r, nil)
}

// saveEntityTexts writes both halves of one entity in a single request: the
// Chinese text into the owning table and the primary target language into
// sys_translation. The page edits them side by side, so a save must not land
// half-way — the source goes first and a failed translation write is reported
// instead of leaving the two columns out of sync. Clearing the target text
// deletes its record, which is how an entity falls back to Chinese.
func saveEntityTexts(w http.ResponseWriter, r *http.Request, ctx *app.Context, req saveTranslationRequest) {
	if req.EntityKey == "" {
		fail(w, r, http.StatusBadRequest, 40000, "errors.translation_invalid_request")
		return
	}
	source := strings.TrimSpace(*req.Source)
	if source == "" {
		fail(w, r, http.StatusBadRequest, 40000, "errors.translation_source_required")
		return
	}
	if err := ctx.Auth.SetSourceText(r.Context(), req.EntityKey, source); err != nil {
		switch {
		case errors.Is(err, store.ErrUnknownEntityKey), errors.Is(err, store.ErrSourceEntityNotFound):
			fail(w, r, http.StatusNotFound, 40400, "errors.translation_not_found")
		default:
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
		}
		return
	}
	lang := pkgi18n.TargetLanguage()
	value := strings.TrimSpace(req.Value)
	if value == "" {
		// An empty target text clears the record; removing nothing is fine here,
		// because the entity simply has no translation in this language.
		if _, err := ctx.Auth.DeleteTranslationForEntity(r.Context(), req.EntityKey, lang); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, nil)
		return
	}
	if err := ctx.Auth.CreateTranslation(r.Context(), req.EntityKey, lang, value); err != nil {
		fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
		return
	}
	ok(w, r, nil)
}

// translateRequest asks for one machine translation of a preset text.
type translateRequest struct {
	// Target overrides the language to translate into; empty means the primary
	// target language the page edits ("en").
	Target string `json:"target"`
	// Source overrides the language the text is written in; empty means the
	// language preset data is authored in ("zh"). The page has a translate
	// button on both columns, so either side can be the one being typed.
	Source string `json:"source"`
	// Text is the text to translate, in Source.
	Text string `json:"text"`
}

// translateTextHandler fills one language column from another so an operator
// only writes each text once.
//
// It is a separate endpoint rather than a server-side step of create/update on
// purpose: showing the result in an editable column keeps the correction step,
// and doing it inside the save would make every write wait on a third-party
// service.
//
// Both directions go through here: the page's Chinese column translates into the
// target language and the target column translates back into Chinese, so a
// translator can start from whichever side they already have.
//
// Failures are reported as 502: the request was valid, a dependency was not. The
// page shows that as "type the translation yourself", never as an empty column.
func translateTextHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req translateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			fail(w, r, http.StatusBadRequest, 40000, "errors.translation_invalid_request")
			return
		}
		target, langErr := translationPageLang(req.Target)
		if langErr != "" {
			fail(w, r, http.StatusBadRequest, 40000, langErr)
			return
		}
		source := strings.TrimSpace(req.Source)
		if source == "" {
			source = pkgi18n.SourceLanguage
		}
		resolved, supported := pkgi18n.Resolve(source)
		if !supported {
			fail(w, r, http.StatusBadRequest, 40000, "errors.translation_lang_unsupported")
			return
		}
		source = resolved
		// Translating a language into itself would only echo the text back as if
		// it were a result, so the caller has to mean it to send the pair.
		if source == target {
			fail(w, r, http.StatusBadRequest, 40000, "errors.translation_invalid_request")
			return
		}
		if ctx.Translate == nil {
			fail(w, r, http.StatusBadGateway, 50200, "errors.translation_service_unavailable")
			return
		}
		result, err := ctx.Translate.Translate(r.Context(), req.Text, source, target)
		switch {
		case errors.Is(err, translate.ErrEmptyText):
			fail(w, r, http.StatusBadRequest, 40000, "errors.translation_source_required")
		case errors.Is(err, translate.ErrTooLong):
			fail(w, r, http.StatusBadRequest, 40000, "errors.translation_text_too_long")
		case err != nil:
			logx.Errorw("machine translation failed",
				logx.Field("source", source),
				logx.Field("target", target),
				logx.Field("error", err.Error()),
			)
			fail(w, r, http.StatusBadGateway, 50200, "errors.translation_service_unavailable")
		default:
			ok(w, r, map[string]any{
				"text":     result.Text,
				"provider": result.Provider,
				"cached":   result.Cached,
			})
		}
	}
}

func listTranslationsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		// One row per preset entity, Chinese source next to the primary target
		// language. The interface language deliberately plays no part: the page
		// edits both columns, so it must list both regardless of who is logged in.
		lang := pkgi18n.TargetLanguage()
		items, total, err := ctx.Auth.ListEntityTranslations(
			r.Context(), lang, query.Get("keyword"), page, pageSize)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_list_failed")
			return
		}
		ok(w, r, map[string]any{
			"items": items, "total": total, "page": page, "pageSize": pageSize,
			"targetLang": lang,
		})
	}
}

// listUntranslatedEntitiesHandler feeds the "add translation" form: it offers
// the preset entities that have no record yet in the primary target language.
// It deliberately ignores the interface language — the form always adds a
// translation, never a Chinese source (every entity already has one).
func listUntranslatedEntitiesHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		lang, langErr := translationPageLang(query.Get("lang"))
		if langErr != "" {
			fail(w, r, http.StatusBadRequest, 40000, langErr)
			return
		}
		items, err := ctx.Auth.UntranslatedEntities(r.Context(), lang, query.Get("keyword"))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_list_failed")
			return
		}
		ok(w, r, items)
	}
}

// automaticTranslation renders a Chinese source into the primary target
// language. It returns the translated text and an empty message key on success,
// or an empty text and the locale key the caller should fail with.
//
// A blank source returns "no error, no text": the caller's own validation
// reports that case, and this helper must not invent a message for it.
func automaticTranslation(r *http.Request, ctx *app.Context, source string) (string, string) {
	text := strings.TrimSpace(source)
	if text == "" {
		return "", ""
	}
	if ctx.Translate == nil {
		return "", "errors.translation_service_unavailable"
	}
	result, err := ctx.Translate.Translate(r.Context(), text, pkgi18n.SourceLanguage, pkgi18n.TargetLanguage())
	switch {
	case errors.Is(err, translate.ErrTooLong):
		return "", "errors.translation_text_too_long"
	case err != nil:
		logx.Errorw("automatic translation failed on save", logx.Field("error", err.Error()))
		return "", "errors.translation_service_unavailable"
	}
	return result.Text, ""
}

func createTranslationHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveTranslationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.EntityKey == "" {
			fail(w, r, http.StatusBadRequest, 40000, "errors.translation_invalid_request")
			return
		}
		if req.Source != nil {
			// A create can arrive with the target column still empty: the operator
			// typed the source and saved before the automatic translation came
			// back. Fill it here — storing a blank translation would mean "no
			// translation" and silently discard the record being asked for.
			// Editing an existing row keeps the blank-is-delete semantics, so the
			// fallback is create-only.
			if strings.TrimSpace(req.Value) == "" {
				translated, messageKey := automaticTranslation(r, ctx, *req.Source)
				if messageKey != "" {
					status, code := http.StatusBadGateway, 50200
					if messageKey == "errors.translation_text_too_long" {
						status, code = http.StatusBadRequest, 40000
					}
					fail(w, r, status, code, messageKey)
					return
				}
				req.Value = translated
			}
			saveEntityTexts(w, r, ctx, req)
			return
		}
		// The form adds a translation to an existing entity, so an unnamed
		// language is the primary target rather than the interface language.
		lang, langErr := translationPageLang(req.Lang)
		if langErr != "" {
			fail(w, r, http.StatusBadRequest, 40000, langErr)
			return
		}
		value := strings.TrimSpace(req.Value)
		if value == "" {
			fail(w, r, http.StatusBadRequest, 40000, "errors.translation_value_required")
			return
		}
		if isSourceLang(lang) {
			saveSourceText(w, r, ctx, req, value)
			return
		}
		if err := ctx.Auth.CreateTranslation(r.Context(), req.EntityKey, lang, value); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, nil)
	}
}

func updateTranslationHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawID, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40000, "errors.translation_invalid_request")
			return
		}
		id := int64(rawID)
		var req saveTranslationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			fail(w, r, http.StatusBadRequest, 40000, "errors.translation_invalid_request")
			return
		}
		if req.Source != nil {
			// Two-column save: the id is the list's display key, and the entity
			// is addressed by entityKey, so nothing here depends on it.
			saveEntityTexts(w, r, ctx, req)
			return
		}
		lang, langErr := translationLang(r, req.Lang)
		if langErr != "" {
			fail(w, r, http.StatusBadRequest, 40000, langErr)
			return
		}
		value := strings.TrimSpace(req.Value)
		if isSourceLang(lang) {
			// The id of a Chinese row is a display key: the entity itself is
			// addressed by entityKey, and its text may never be blanked.
			if value == "" {
				fail(w, r, http.StatusBadRequest, 40000, "errors.translation_source_required")
				return
			}
			saveSourceText(w, r, ctx, req, value)
			return
		}
		err := ctx.Auth.UpdateTranslation(r.Context(), id, lang, value)
		switch {
		case errors.Is(err, store.ErrTranslationNotFound):
			fail(w, r, http.StatusNotFound, 40400, "errors.translation_not_found")
		case errors.Is(err, store.ErrTranslationConflict):
			// Unreachable with English as the only target (a record cannot
			// move onto its own language) but kept so a second registered
			// language gets a clear 409 instead of a 500.
			fail(w, r, http.StatusConflict, 40900, "errors.translation_lang_conflict")
		case err != nil:
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
		default:
			ok(w, r, nil)
		}
	}
}

func deleteTranslationHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// The page lists one row per preset entity and its id is a display key
		// (ListSourceEntities), so a delete names the entity instead of the
		// record: nothing in the grid carries a sys_translation id, which is why
		// there is no id-addressed branch here. It removes the translation only:
		// the entity falls back to its Chinese source, and the source itself is
		// the entity, so this page never deletes preset data.
		key := strings.TrimSpace(r.URL.Query().Get("entityKey"))
		if key == "" {
			fail(w, r, http.StatusBadRequest, 40000, "errors.translation_invalid_request")
			return
		}
		removed, err := ctx.Auth.DeleteTranslationForEntity(
			r.Context(), key, pkgi18n.TargetLanguage())
		switch {
		case err != nil:
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
		case !removed:
			// A stale row (another operator cleared it, or a code change
			// rekeyed the entity) must not report success: the caller would
			// keep showing a translation that no longer exists.
			fail(w, r, http.StatusNotFound, 40400, "errors.translation_not_found")
		default:
			ok(w, r, nil)
		}
	}
}
