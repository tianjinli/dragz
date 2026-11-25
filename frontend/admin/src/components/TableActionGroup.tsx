import { useState, type ReactNode } from "react";
import { Button, Dropdown, Popconfirm, Space, Tooltip } from "antd";
import { MoreHorizontal } from "lucide-react";
import { useTranslation } from "react-i18next";
import { MIN_LIST_COLUMN_WIDTH } from "@/utils/table-columns";

export interface TableAction {
  danger?: boolean;
  disabled?: boolean;
  /** Force this action into the 「更多」overflow menu. */
  inMore?: boolean;
  /** Visible control is icon-only; label is used for tooltip / aria / confirm. */
  icon: ReactNode;
  label: string;
  /**
   * Name of the object this action acts on (the row's display name). A
   * destructive confirmation has to say *what* is about to be deleted: the
   * dialog outlives the row it was opened from, and "确定删除吗？" is answered
   * from memory, not from the screen.
   */
  confirmName?: string;
  onClick: () => void | Promise<void>;
}

export function splitTableActions(actions: TableAction[]) {
  const primary = actions.filter((action) => !action.inMore);
  const forced = actions.filter((action) => action.inMore);

  if (forced.length === 0) {
    if (actions.length <= 3)
      return { visibleActions: actions, overflowActions: [] };
    return {
      visibleActions: actions.slice(0, 2),
      overflowActions: actions.slice(2),
    };
  }

  if (primary.length <= 3) {
    return { visibleActions: primary, overflowActions: forced };
  }
  return {
    visibleActions: primary.slice(0, 2),
    overflowActions: [...primary.slice(2), ...forced],
  };
}

/**
 * Ant Design `type="link"` icon-only button footprint (control height),
 * not just the 16px glyph — underestimating this clips the menu row's
 * 新增下级 / 修改 / 删除 trio.
 */
const ACTION_ICON_BUTTON_WIDTH = 32;
/** Matches `<Space size={ACTION_GAP}>` in the group. */
const ACTION_GAP = 8;
/** Ant Design table cell horizontal padding (16px × 2). */
const ACTION_CELL_PADDING_X = 32;

/**
 * How many controls the action cell actually paints for the given actions
 * (visible icon buttons + optional 「更多」).
 */
export function actionColumnSlotCount(
  actions: Array<{ inMore?: boolean }>,
): number {
  if (actions.length === 0) return 0;
  const { visibleActions, overflowActions } = splitTableActions(
    actions.map((action) => ({
      ...action,
      icon: null,
      label: "",
      onClick: () => undefined,
    })),
  );
  return visibleActions.length + (overflowActions.length > 0 ? 1 : 0);
}

/**
 * 操作列宽度：按单元格里实际会出现的图标按钮数自适应
 *（含溢出时的「更多」），并加上表格单元格左右内边距。
 * 仅 1 个按钮时仍按 2 个计宽，保证表头「操作」能在一行内显示。
 */
export function actionColumnWidth(
  actionsOrCount: number | Array<{ inMore?: boolean }>,
) {
  const raw =
    typeof actionsOrCount === "number"
      ? actionsOrCount <= 0
        ? 0
        : Math.min(actionsOrCount, 3)
      : actionColumnSlotCount(actionsOrCount);
  if (raw <= 0) return MIN_LIST_COLUMN_WIDTH;
  const slots = Math.max(raw, 2);
  const content =
    ACTION_ICON_BUTTON_WIDTH * slots + ACTION_GAP * Math.max(0, slots - 1);
  return Math.ceil(content + ACTION_CELL_PADDING_X);
}

export function TableActionGroup({ actions }: { actions: TableAction[] }) {
  const { t } = useTranslation();
  const [moreOpen, setMoreOpen] = useState(false);
  const { visibleActions, overflowActions } = splitTableActions(actions);

  const renderAction = (action: TableAction, options?: { closeMore?: boolean }) => {
    const run = () => {
      if (options?.closeMore) setMoreOpen(false);
      void action.onClick();
    };
    const button = (
      <Tooltip title={action.label}>
        <span className="table-action-trigger">
          <Button
            aria-label={action.label}
            danger={action.danger}
            disabled={action.disabled}
            icon={action.icon}
            type="link"
            onClick={action.danger ? undefined : () => run()}
          />
        </span>
      </Tooltip>
    );
    return action.danger ? (
      <Popconfirm
        key={action.label}
        title={t("common.confirmDeleteTitle", {
          name: action.confirmName || action.label,
        })}
        description={t("common.confirmDeleteDetail")}
        okText={t("common.confirm")}
        cancelText={t("common.cancel")}
        onConfirm={() => run()}
      >
        {button}
      </Popconfirm>
    ) : (
      <span key={action.label}>{button}</span>
    );
  };

  return (
    <div className="table-action-group">
      <Space size={ACTION_GAP}>
        {visibleActions.map((action) => renderAction(action))}
        {overflowActions.length > 0 && (
          <Dropdown
            destroyOnHidden
            open={moreOpen}
            trigger={["click"]}
            onOpenChange={setMoreOpen}
            popupRender={() => (
              <div className="table-action-menu">
                {overflowActions.map((action) =>
                  renderAction(action, { closeMore: true }),
                )}
              </div>
            )}
          >
            <Button
              aria-label={t("common.moreActions")}
              icon={<MoreHorizontal size={16} />}
              type="link"
            />
          </Dropdown>
        )}
      </Space>
    </div>
  );
}
