import { useCallback, useEffect, useRef, useState } from "react";
import { TranslationOutlined } from "@ant-design/icons";
import { Button, Tag, Tooltip, Typography, message } from "antd";
import { useTranslation } from "react-i18next";
import { ResourceCrudPage } from "@/components/ResourceCrudPage";
import {
  deleteTranslationApi,
  listTranslationCandidatesApi,
  translatePresetTextApi,
} from "@/api/system";

/**
 * Translation management: one row per preset entity, Chinese source and the
 * primary target language side by side, both editable in the same save.
 *
 * Scope is preset data only — the menu names, permission button labels, roles,
 * departments, positions, jobs and storage configs that migrations and plugin
 * seeds write. Interface copy is deliberately absent: buttons and labels ship
 * with the code as message packs, so changing them is a release, not an admin
 * action.
 *
 * The page is deliberately independent of the interface language. It used to
 * list whichever language the operator was working in, which meant an English
 * UI showed the Chinese source as a read-only column and editing it required
 * switching the whole interface. Both halves are now always present: Chinese is
 * written back to the owning table, the target language to sys_translation, and
 * clearing the target deletes its record so the entity falls back to Chinese.
 *
 * Create and delete operate on translations, never on the preset data:
 *   - create picks an entity that has no translation yet (the form's list comes
 *     from the candidates endpoint) and then behaves exactly like an edit: the
 *     entity's Chinese text is filled in from the pick and both columns are
 *     saved in one request, so a new translation can also fix a source text
 *     that reads badly;
 *   - delete drops that English text, which is the same thing as clearing the
 *     field — the entity keeps its Chinese source and is never removed here.
 * That is why delete is greyed out on a row that has no translation to drop.
 *
 * The target column is machine-translated from the source while it is typed, so
 * an operator only writes the Chinese text. The result lands in the editable
 * column and can still be corrected; a hand-written value is never overwritten
 * by an answer that arrives afterwards. Saving before the translation returns is
 * safe too — the create endpoint fills a blank target itself — so a quick save
 * cannot store an empty translation by accident.
 */
/**
 * Reads the text out of an onChange argument.
 * `FieldConfig.onChange` is called with whatever the wrapped control passes:
 * a Select hands over the picked value, an Input hands over the DOM event. The
 * source field is an Input, so the event is the common case here — reading it as
 * a string would translate "[object Object]".
 */
function sourceTextOf(value: unknown): string {
  if (typeof value === "string") return value;
  const event = value as { target?: { value?: unknown } } | null | undefined;
  const raw = event?.target?.value;
  return typeof raw === "string" ? raw : "";
}

// Preset data is authored in Chinese and the page edits one target language next
// to it; the endpoints resolve the same pair when a request leaves them out.
const SOURCE_LANG = "zh";
const TARGET_LANG = "en";

export function TranslationPage() {
  const { t } = useTranslation();
  // Chinese texts of the entities the create form is offering, so picking one
  // can fill the source column without a second round trip.
  const candidateSources = useRef(new Map<string, string>());
  // Debounce handle for the automatic translation of the source text.
  const translateTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Monotonic id: only the newest request may write the target column, so a
  // slow answer for an earlier keystroke cannot overwrite a newer one.
  const translateSeq = useRef(0);
  // Set once the operator edits the target column by hand, so a translation
  // that lands afterwards does not throw their wording away.
  const manualTarget = useRef(false);
  // One warning per form session: a blocked service must not toast on every key.
  const warned = useRef(false);
  // Which column's translate button is in flight, so only that button spins.
  const [translating, setTranslating] = useState<"source" | "value" | null>(
    null,
  );

  useEffect(
    () => () => {
      if (translateTimer.current) clearTimeout(translateTimer.current);
    },
    [],
  );

  /**
   * Fills the target column from the Chinese source, so typing the source is the
   * only input the operator has to give.
   *
   * Debounced because it runs per keystroke, and silent on failure: the column
   * stays editable and saving reports a blank target on its own, which is a
   * better prompt than a toast per character.
   */
  const scheduleTranslate = useCallback(
    (text: string, setFieldValue: (name: string, value: unknown) => void) => {
      if (translateTimer.current) clearTimeout(translateTimer.current);
      const trimmed = text.trim();
      if (!trimmed) return;
      const seq = ++translateSeq.current;
      translateTimer.current = setTimeout(() => {
        void translatePresetTextApi({ text: trimmed })
          .then((result) => {
            if (seq !== translateSeq.current || manualTarget.current) return;
            if (result?.text) setFieldValue("value", result.text);
          })
          .catch(() => {
            if (seq !== translateSeq.current || warned.current) return;
            warned.current = true;
            void message.warning(
              t("system.settings.translation.autoTranslateFailed"),
            );
          });
      }, 400);
    },
    [t],
  );

  /**
   * The translate button each language column carries: it takes that column as
   * the source and replaces the other one.
   *
   * The silent pass above only fills an empty target, which is right while
   * typing but does nothing on an entity that already has a translation — the
   * case an operator comes here to fix. A press is therefore explicit and always
   * overwrites: it is the only way to ask "translate this again".
   */
  const translateInto = useCallback(
    async (
      from: "source" | "value",
      getFieldValue: (name: string) => unknown,
      setFieldValue: (name: string, value: unknown) => void,
    ) => {
      const text = String(getFieldValue(from) ?? "").trim();
      if (!text) {
        void message.warning(t("system.settings.translation.translateEmpty"));
        return;
      }
      const into = from === "source" ? "value" : "source";
      setTranslating(from);
      try {
        const result = await translatePresetTextApi({
          text,
          source: from === "source" ? SOURCE_LANG : TARGET_LANG,
          target: from === "source" ? TARGET_LANG : SOURCE_LANG,
        });
        if (!result?.text) return;
        setFieldValue(into, result.text);
        if (into === "value") {
          // A press counts as the operator's own wording: the debounced pass
          // that runs after the next source edit must not undo it silently.
          manualTarget.current = true;
        }
      } catch {
        void message.error(t("system.settings.translation.autoTranslateFailed"));
      } finally {
        setTranslating(null);
      }
    },
    [t],
  );

  /**
   * The trailing control both language columns share: same button, same place,
   * same behaviour. Only the tooltip differs, because the direction does — an
   * operator who already has the English text can translate back into Chinese.
   */
  const translateButton = (
    from: "source" | "value",
    getFieldValue: (name: string) => unknown,
    setFieldValue: (name: string, value: unknown) => void,
  ) => {
    // Icon only: the field already says what it holds, and a label would eat
    // the width the text itself needs. The direction moves into the tooltip,
    // which doubles as the accessible name.
    const label =
      from === "source"
        ? t("system.settings.translation.translateToTarget")
        : t("system.settings.translation.translateToSource");
    return (
      <Tooltip title={label}>
        <Button
          aria-label={label}
          htmlType="button"
          icon={<TranslationOutlined />}
          loading={translating === from}
          onClick={() => void translateInto(from, getFieldValue, setFieldValue)}
          size="small"
          type="text"
        />
      </Tooltip>
    );
  };

  return (
    <ResourceCrudPage
      title={t("system.settings.translation.title")}
      description={t("system.settings.translation.description")}
      endpoint="/system/translations"
      paged
      createCode="system:translation:manage"
      updateCode="system:translation:manage"
      deleteCode="system:translation:manage"
      searchPlaceholder={t("system.settings.translation.searchPlaceholder")}
      columns={[
        {
          title: t("system.settings.translation.key"),
          dataIndex: "entityKey",
          align: "left",
          // Narrow enough to leave the text columns room; the key itself still
          // truncates, the sentence columns are the ones that need the width.
          width: 260,
          render: (value) => {
            const key = String(value ?? "");
            const prefix = key.slice(0, key.indexOf("."));
            return (
              <>
                <Tag>{prefix}</Tag>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {key}
                </Typography.Text>
              </>
            );
          },
        },
        {
          title: t("system.settings.translation.source"),
          dataIndex: "source",
          align: "left",
          // Both columns carry whole sentences; wrapping shows them in full
          // instead of leaving a “…” that the reviewer has to hover to read.
          wrap: true,
        },
        {
          title: t("system.settings.translation.translation"),
          dataIndex: "value",
          align: "left",
          wrap: true,
          render: (value) =>
            String(value ?? "") ? (
              value as string
            ) : (
              <Typography.Text type="secondary">
                {t("system.settings.translation.untranslated")}
              </Typography.Text>
            ),
        },
      ]}
      fields={[
        {
          key: "entityKey",
          label: t("system.settings.translation.key"),
          type: "input",
          // On create the entity is picked from the ones still untranslated;
          // afterwards the key identifies the row and is locked.
          typeOnCreate: "select",
          required: true,
          remoteSearch: true,
          disabledOnEdit: true,
          placeholder: t("system.settings.translation.selectEntity"),
          optionLoader: ({ editing, keyword }) => {
            if (editing) return Promise.resolve([]);
            return listTranslationCandidatesApi({ keyword }).then((items) => {
              candidateSources.current = new Map(
                items.map((item) => [item.entityKey, item.source]),
              );
              // Only the key is listed: the source text has a column of its own
              // in the form, and repeating it in the option made the list hard
              // to scan for the identifier an operator came here with.
              return items.map((item) => ({
                label: item.entityKey,
                value: item.entityKey,
              }));
            });
          },
          // Picking an entity brings its Chinese text along and translates it,
          // so the create form submits both columns without the operator typing
          // the target language.
          onChange: (value, { setFieldValue }) => {
            const source = candidateSources.current.get(String(value ?? ""));
            if (source === undefined) return;
            setFieldValue("source", source);
            manualTarget.current = false;
            scheduleTranslate(source, setFieldValue);
          },
        },
        {
          key: "source",
          label: t("system.settings.translation.source"),
          type: "input",
          required: true,
          placeholder: t("system.settings.translation.valuePlaceholderSource"),
          // The button sits in the Chinese column and fills the English one, so a
          // text that is already translated can be re-translated on demand.
          suffix: ({ getFieldValue, setFieldValue }) =>
            translateButton("source", getFieldValue, setFieldValue),
          // Every edit of the source re-translates, which also resets the
          // "operator typed the target" guard: a new source means the old
          // translation belongs to text that no longer exists.
          onChange: (value, { setFieldValue }) => {
            manualTarget.current = false;
            warned.current = false;
            scheduleTranslate(sourceTextOf(value), setFieldValue);
          },
        },
        {
          key: "value",
          label: t("system.settings.translation.translation"),
          type: "input",
          placeholder: t("system.settings.translation.valuePlaceholder"),
          // Same control as the Chinese column, mirrored: an operator who has the
          // English text can translate it back into Chinese.
          suffix: ({ getFieldValue, setFieldValue }) =>
            translateButton("value", getFieldValue, setFieldValue),
          // Typing here wins over the automatic translation.
          onChange: () => {
            manualTarget.current = true;
          },
          // Required when adding a translation; on edit an empty value is
          // meaningful — it deletes the record and falls back to Chinese.
          rules: ({ editing }) =>
            editing
              ? []
              : [
                  {
                    required: true,
                    whitespace: true,
                    message: t("crud.requiredField", {
                      label: t("system.settings.translation.translation"),
                    }),
                  },
                ],
        },
      ]}
      // Create and edit submit the same shape: both columns, one save. `source`
      // being present is what tells the backend this is a two-column save.
      mapSubmitValues={(values) => ({
        entityKey: values.entityKey,
        source: values.source ?? "",
        value: values.value ?? "",
      })}
      deleteDisabled={(record) => !String(record.value ?? "").trim()}
      onDelete={(record) =>
        deleteTranslationApi(record.id, String(record.entityKey))
      }
    />
  );
}
