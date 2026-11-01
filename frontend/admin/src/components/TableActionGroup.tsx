import { useState } from "react";
import { Button, Dropdown, Popconfirm, Space } from "antd";
import { MoreHorizontal } from "lucide-react";
import { useTranslation } from "react-i18next";
import { MIN_LIST_COLUMN_WIDTH } from "@/utils/table-columns";

export interface TableAction {
  danger?: boolean;
  disabled?: boolean;
  /** Force this action into the 「更多」overflow menu. */
  inMore?: boolean;
  label: string;
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

const ACTION_BUTTON_PADDING_X = 16;
const ACTION_GAP = 24;
function isCJK(ch: string) {
  return /[\u2e80-\u9fff\uf900-\ufaff\uff00-\uffef]/.test(ch);
}
function linkButtonLabelWidth(label: string) {
  const chars = Array.from(label);
  const textWidth = chars.reduce((sum, ch) => sum + (isCJK(ch) ? 14 : 8), 0);
  // antd 对恰好两个中文字符的按钮加 letter-spacing: 0.34em。
  return textWidth + (chars.length === 2 ? 5 : 0) + ACTION_BUTTON_PADDING_X * 2;
}

/** 操作列初始宽度：刚好容纳所有链接按钮（文字宽 + 内边距 + 间距）。 */
export function actionColumnWidth(actions: Array<{ label: string }>) {
  if (!actions.length) return MIN_LIST_COLUMN_WIDTH;
  const total =
    actions.reduce(
      (sum, action) => sum + linkButtonLabelWidth(action.label),
      0,
    ) +
    ACTION_GAP * (actions.length - 1);
  return Math.max(MIN_LIST_COLUMN_WIDTH, Math.ceil(total));
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
      <Button
        danger={action.danger}
        disabled={action.disabled}
        type="link"
        onClick={action.danger ? undefined : () => run()}
      >
        {action.label}
      </Button>
    );
    return action.danger ? (
      <Popconfirm
        key={action.label}
        title={t("common.confirmDelete", { name: action.label })}
        description={t("common.confirmAction")}
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
      <Space size={12}>
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
