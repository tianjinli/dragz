import { useCallback, useMemo, useState } from "react";
import { Button, Drawer, Segmented, Tag, Upload, message } from "antd";
import {
  Download,
  Eye,
  ListTree,
  RotateCcw,
  Send,
  Trash2,
  Undo2,
  Upload as UploadIcon,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  downloadFileApi,
  getFileDetailApi,
  getMyNoticeApi,
  listDepartmentsApi,
  listFilesApi,
  listMyNoticesApi,
  listMyNotificationsApi,
  listRolesApi,
  listUsersApi,
  markNoticeReadApi,
  markNotificationReadApi,
  publishNoticeApi,
  purgeFileApi,
  recycleFileApi,
  restoreFileApi,
  revokeNoticeApi,
  uploadFileApi,
  type ResourceRecord,
} from "@/api/system";
import { DataTablePage } from "@/components/DataTablePage";
import { NoticePriorityTag } from "@/components/NoticePriorityTag";
import {
  ResourceCrudPage,
  type FieldConfig,
  type RowAction,
} from "@/components/ResourceCrudPage";
import { useMessagesStore } from "@/stores/messages";
import { useSystemConfigStore } from "@/stores/system-config";
import { formatDateTime } from "@/utils/format-time";
import {
  isAnnouncementNoticeType,
  normalizeNoticePriority,
  normalizeNoticeType,
} from "@/utils/notice";

export function NoticePage() {
  const { t } = useTranslation();
  const revokeWindowMinutes = useSystemConfigStore((s) =>
    Number(s.get("notice.revokeWindowMinutes", "0")),
  );
  const actions: RowAction[] = [
    {
      icon: <Send size={16} />,
      label: t("system.content.publish"),
      hidden: (row) => String(row.status) !== "draft",
      onClick: async (row) => {
        await publishNoticeApi(row.id);
        void message.success(t("system.content.published"));
      },
    },
    {
      icon: <Undo2 size={16} />,
      label: t("system.content.revoke"),
      hidden: (row) => String(row.status) !== "published",
      disabled: (row) => {
        if (!(revokeWindowMinutes > 0) || !row.publishAt) return false;
        const publishedAt = Date.parse(String(row.publishAt));
        return (
          !Number.isNaN(publishedAt) &&
          Date.now() - publishedAt > revokeWindowMinutes * 60_000
        );
      },
      onClick: async (row) => {
        await revokeNoticeApi(row.id);
        void message.success(t("system.content.revoked"));
      },
    },
  ];
  const fields: FieldConfig[] = [
    {
      key: "noticeType",
      label: t("common.type"),
      type: "select",
      required: true,
      options: [
        { label: t("system.content.announcement"), value: "announcement" },
        { label: t("system.content.notification"), value: "notification" },
      ],
      onChange: (value, { setFieldValue }) => {
        if (isAnnouncementNoticeType(value)) {
          setFieldValue("targetType", "all");
          setFieldValue("targetIds", []);
        } else if (!String(value || "")) {
          setFieldValue("targetType", "user");
        } else {
          setFieldValue("targetType", "user");
          setFieldValue("targetIds", []);
        }
      },
    },
    {
      key: "priority",
      label: t("system.content.priority"),
      type: "select",
      required: true,
      options: [
        { label: t("system.content.priorityNormal"), value: "normal" },
        { label: t("system.content.priorityImportant"), value: "important" },
        { label: t("system.content.priorityUrgent"), value: "urgent" },
      ],
    },
    {
      key: "isPinned",
      label: t("system.content.isPinned"),
      type: "switch",
    },
    {
      key: "publishAt",
      label: t("system.content.publishAt"),
      type: "datetime",
      placeholder: t("system.content.publishAtPlaceholder"),
    },
    { key: "title", label: t("system.content.title"), required: true },
    {
      key: "content",
      label: t("system.content.content"),
      type: "textarea",
      required: true,
    },
    {
      key: "targetType",
      label: t("system.content.targetType"),
      type: "select",
      required: true,
      dependencies: ["noticeType"],
      visibleWhen: (get) => normalizeNoticeType(get("noticeType")) === "notification",
      options: [
        { label: t("system.content.targetUser"), value: "user" },
        { label: t("system.content.targetDepartment"), value: "department" },
        { label: t("system.content.targetRole"), value: "role" },
      ],
      onChange: (_value, { setFieldValue }) => setFieldValue("targetIds", []),
    },
    {
      key: "targetIds",
      label: t("system.content.targetIds"),
      type: "select",
      mode: "multiple",
      numeric: true,
      required: true,
      dependencies: ["noticeType", "targetType"],
      visibleWhen: (get) => normalizeNoticeType(get("noticeType")) === "notification",
      optionDependencies: ["targetType"],
      optionLoader: async ({ values }) => {
        const type = String(values.targetType || "");
        if (type === "user") {
          const page = await listUsersApi({ page: 1, pageSize: 200 });
          return (page.items || []).map((row) => ({
            label: String(row.displayName || row.username || row.id),
            value: Number(row.id),
          }));
        }
        if (type === "department") {
          const rows = await listDepartmentsApi();
          return rows.map((row) => ({
            label: String(row.name || row.id),
            value: Number(row.id),
          }));
        }
        if (type === "role") {
          const rows = await listRolesApi();
          return rows.map((row) => ({
            label: String(row.name || row.code || row.id),
            value: Number(row.id),
          }));
        }
        return [];
      },
    },
  ];
  return (
    <ResourceCrudPage
      title={t("system.content.noticeTitle")}
      endpoint="/system/notices"
      paged
      loadDetailOnEdit
      createCode="system:notice:manage"
      updateCode="system:notice:manage"
      deleteCode="system:notice:manage"
      defaultValues={{
        noticeType: "announcement",
        priority: "normal",
        targetType: "all",
        targetIds: [],
        isPinned: false,
      }}
      mapRecordValues={(values) => ({
        ...values,
        noticeType: normalizeNoticeType(values.noticeType),
        targetType: isAnnouncementNoticeType(values.noticeType)
          ? "all"
          : values.targetType === "all" || !values.targetType
            ? "user"
            : values.targetType,
      })}
      mapSubmitValues={(values) => {
        const noticeType = normalizeNoticeType(values.noticeType);
        if (noticeType === "announcement") {
          return {
            ...values,
            noticeType,
            targetType: "all",
            targetIds: [],
          };
        }
        return {
          ...values,
          noticeType,
          targetType: values.targetType || "user",
          targetIds: Array.isArray(values.targetIds) ? values.targetIds : [],
        };
      }}
      rowActions={actions}
      columns={[
        { title: t("system.content.title"), dataIndex: "title", width: 240 },
        {
          title: t("common.type"),
          dataIndex: "noticeType",
          width: 100,
          labelMap: {
            announcement: t("system.content.announcement"),
            notice: t("system.content.announcement"),
            notification: t("system.content.notification"),
          },
        },
        {
          title: t("system.content.priority"),
          dataIndex: "priority",
          width: 120,
          render: (value) => <NoticePriorityTag priority={value} />,
        },
        { title: t("common.status"), dataIndex: "status" },
        {
          title: t("system.content.publishedAt"),
          dataIndex: "publishAt",
          width: 180,
        },
      ]}
      fields={fields}
    />
  );
}

type InboxTab = "all" | "notice" | "notification";

export function MyMessagesPage() {
  const { t } = useTranslation();
  const refreshBell = useMessagesStore((state) => state.refresh);
  const [tab, setTab] = useState<InboxTab>("all");
  const [current, setCurrent] = useState<ResourceRecord | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  const load = useCallback(
    async (q: { keyword: string; page: number; pageSize: number }) => {
      const mapNoticeRow = (row: ResourceRecord) => {
        const noticeType = normalizeNoticeType(row.noticeType);
        return {
          ...row,
          noticeType,
          _kind: noticeType,
          _source: "notice" as const,
          read: row.read,
          _sort: String(row.publishAt || row.createdAt || ""),
        };
      };

      if (tab === "notice") {
        const data = await listMyNoticesApi({
          page: 1,
          pageSize: 100,
          keyword: q.keyword,
          reloadKey,
        } as never);
        const items = (data.items || [])
          .map(mapNoticeRow)
          .filter((row) => row._kind === "announcement");
        const start = (q.page - 1) * q.pageSize;
        return {
          items: items.slice(start, start + q.pageSize),
          total: items.length,
        };
      }
      if (tab === "notification") {
        const [notices, notifications] = await Promise.all([
          listMyNoticesApi({
            page: 1,
            pageSize: 100,
            keyword: q.keyword,
            reloadKey,
          } as never),
          listMyNotificationsApi({
            page: 1,
            pageSize: 50,
            keyword: q.keyword,
            reloadKey,
          } as never),
        ]);
        const merged = [
          ...(notices.items || [])
            .map(mapNoticeRow)
            .filter((row) => row._kind === "notification"),
          ...(notifications.items || []).map((row) => ({
            ...row,
            _kind: "notification" as const,
            _source: "inbox" as const,
            _sort: String(row.createdAt || ""),
          })),
        ].sort((a, b) => String(b._sort).localeCompare(String(a._sort)));
        const start = (q.page - 1) * q.pageSize;
        return {
          items: merged.slice(start, start + q.pageSize),
          total: merged.length,
        };
      }
      const [notices, notifications] = await Promise.all([
        listMyNoticesApi({ page: 1, pageSize: 50, keyword: q.keyword }),
        listMyNotificationsApi({ page: 1, pageSize: 50, keyword: q.keyword }),
      ]);
      const merged = [
        ...(notices.items || []).map(mapNoticeRow),
        ...(notifications.items || []).map((row) => ({
          ...row,
          _kind: "notification" as const,
          _source: "inbox" as const,
          _sort: String(row.createdAt || ""),
        })),
      ].sort((a, b) => String(b._sort).localeCompare(String(a._sort)));
      const start = (q.page - 1) * q.pageSize;
      return {
        items: merged.slice(start, start + q.pageSize),
        total: merged.length,
      };
    },
    [reloadKey, tab],
  );

  const columns = useMemo(
    () => [
      { title: t("system.content.title"), dataIndex: "title", width: 180 },
      {
        title: t("system.content.content"),
        dataIndex: "content",
        ellipsis: true,
        render: (value: unknown, row: ResourceRecord) =>
          String(value || row.summary || row.content || "—"),
      },
      {
        title: t("common.type"),
        dataIndex: "_kind",
        width: 100,
        render: (value: unknown) =>
          value === "notification"
            ? t("system.content.inboxNotifications")
            : t("system.content.inboxNotices"),
      },
      {
        title: t("system.content.priority"),
        dataIndex: "priority",
        width: 110,
        render: (value: unknown, row: ResourceRecord) =>
          row._source === "inbox" ? "—" : <NoticePriorityTag priority={value} />,
      },
      {
        title: t("common.status"),
        dataIndex: "read",
        width: 90,
        render: (value: unknown) => (
          <Tag color={value ? "default" : "blue"}>
            {value ? t("system.content.read") : t("system.content.unread")}
          </Tag>
        ),
      },
      {
        title: t("system.content.time"),
        dataIndex: "createdAt",
        width: 170,
        render: (_: unknown, row: ResourceRecord) =>
          formatDateTime(
            (row.publishAt || row.createdAt || "") as string | number | Date,
          ),
      },
    ],
    [t],
  );

  return (
    <>
      <DataTablePage
        title={t("system.content.myMessages")}
        loadData={load}
        columns={columns}
        extra={
          <Segmented
            value={tab}
            onChange={(value) => {
              setTab(value as InboxTab);
              setReloadKey((key) => key + 1);
            }}
            options={[
              { label: t("system.content.inboxAll"), value: "all" },
              { label: t("system.content.inboxNotices"), value: "notice" },
              {
                label: t("system.content.inboxNotifications"),
                value: "notification",
              },
            ]}
          />
        }
        actions={[
          {
            icon: <Eye size={16} />,
            label: t("system.content.view"),
            onClick: async (row, reload) => {
              if (row._source === "inbox") {
                setCurrent({ ...row, _kind: "notification", _source: "inbox" });
                if (!row.read) {
                  await markNotificationReadApi(row.id);
                  await reload();
                  await refreshBell();
                }
                return;
              }
              const detail = await getMyNoticeApi(row.id);
              const noticeType = normalizeNoticeType(detail.noticeType);
              setCurrent({
                ...detail,
                noticeType,
                _kind: noticeType,
                _source: "notice",
              });
              if (!row.read) {
                await markNoticeReadApi(row.id);
                await reload();
                await refreshBell();
              }
            },
          },
        ]}
      />
      <Drawer
        open={!!current}
        title={String(current?.title || t("system.content.messageDetail"))}
        size={640}
        onClose={() => setCurrent(null)}
      >
        {current?._source === "notice" ? (
          <div
            className={`notice-detail-meta is-${normalizeNoticePriority(current.priority)}`}
          >
            <NoticePriorityTag priority={current.priority} />
            <Tag>
              {normalizeNoticeType(current.noticeType) === "notification"
                ? t("system.content.inboxNotifications")
                : t("system.content.inboxNotices")}
            </Tag>
          </div>
        ) : (
          <div className="notice-detail-meta is-normal">
            <Tag>{t("system.content.inboxNotifications")}</Tag>
          </div>
        )}
        <div className="notice-content">
          {String(current?.content || "")}
        </div>
      </Drawer>
    </>
  );
}

export function FilePage({ recycle = false }: { recycle?: boolean }) {
  const { t } = useTranslation();
  const [reloadKey, setReloadKey] = useState(0);
  const load = useCallback(
    (q: { keyword: string; page: number; pageSize: number }) =>
      listFilesApi({ ...q, recycled: recycle, reloadKey }),
    [recycle, reloadKey],
  );
  const upload = (
    <Upload
      showUploadList={false}
      customRequest={({ file, onSuccess, onError }) => {
        const data = new FormData();
        data.append("file", file as Blob);
        void uploadFileApi(data)
          .then(() => {
            onSuccess?.({});
            setReloadKey((k) => k + 1);
            void message.success(t("upload.uploadSuccess"));
          })
          .catch(onError);
      }}
    >
      <Button
        aria-label={t("system.content.uploadFile")}
        title={t("system.content.uploadFile")}
        type="primary"
        icon={<UploadIcon size={16} />}
      />
    </Upload>
  );
  const common = [
    { title: t("system.content.fileName"), dataIndex: "originalName" },
    { title: t("system.content.size"), dataIndex: "size" },
    { title: t("common.type"), dataIndex: "mimeType" },
    { title: t("system.content.uploader"), dataIndex: "uploaderName" },
    { title: t("system.content.time"), dataIndex: "createdAt" },
  ];
  const actions = recycle
    ? [
        {
          icon: <RotateCcw size={16} />,
          label: t("system.content.restore"),
          onClick: async (
            row: ResourceRecord,
            reload: () => Promise<void>,
          ) => {
            await restoreFileApi(row.id);
            void message.success(t("system.content.restored"));
            await reload();
          },
        },
        {
          icon: <Trash2 size={16} />,
          label: t("system.content.purge"),
          danger: true,
          onClick: async (
            row: ResourceRecord,
            reload: () => Promise<void>,
          ) => {
            await purgeFileApi(row.id);
            void message.success(t("system.content.purged"));
            await reload();
          },
        },
      ]
    : [
        {
          icon: <Download size={16} />,
          label: t("system.content.download"),
          onClick: (row: ResourceRecord) =>
            downloadFileApi(
              row.id,
              String(row.originalName || row.name || "download"),
            ),
        },
        {
          icon: <Trash2 size={16} />,
          label: t("common.delete"),
          danger: true,
          onClick: async (
            row: ResourceRecord,
            reload: () => Promise<void>,
          ) => {
            await recycleFileApi(row.id);
            void message.success(t("system.content.movedToRecycle"));
            await reload();
          },
        },
      ];
  return (
    <DataTablePage
      title={
        recycle
          ? t("system.content.recycleTitle")
          : t("system.content.fileTitle")
      }
      loadData={load}
      columns={common}
      actions={actions}
      extra={recycle ? undefined : upload}
    />
  );
}
export const FileRecyclePage = () => <FilePage recycle />;
export function FileReferencesPage() {
  const { t } = useTranslation();
  const [current, setCurrent] = useState<{
    file: ResourceRecord;
    references: ResourceRecord[];
  } | null>(null);
  const load = useCallback(
    (q: { keyword: string; page: number; pageSize: number }) =>
      listFilesApi({ ...q, status: "ready" }),
    [],
  );
  const columns = [
    { title: t("system.content.fileName"), dataIndex: "originalName" },
    {
      title: t("system.content.referenceCount"),
      dataIndex: "referenceCount",
    },
    { title: t("system.content.uploader"), dataIndex: "uploaderName" },
    { title: t("system.content.time"), dataIndex: "createdAt" },
  ];
  const actions = [
    {
      icon: <ListTree size={16} />,
      label: t("system.content.viewReferences"),
      onClick: async (row: ResourceRecord) => {
        setCurrent(await getFileDetailApi(row.id));
      },
    },
  ];
  return (
    <>
      <DataTablePage
        title={t("system.content.referencesTitle")}
        loadData={load}
        columns={columns}
        actions={actions}
      />
      <Drawer
        open={!!current}
        title={String(
          current?.file?.originalName || t("system.content.referenceDetail"),
        )}
        size={640}
        onClose={() => setCurrent(null)}
      >
        {(current?.references ?? []).map((ref) => (
          <div key={String(ref.id)}>
            <Tag>{String(ref.businessType)}</Tag>
            {String(ref.businessId)} · {String(ref.fieldName)}
          </div>
        ))}
        {(current?.references ?? []).length === 0 && (
          <div>{t("system.content.noReferences")}</div>
        )}
      </Drawer>
    </>
  );
}
