import { useState, type ReactElement, type ReactNode } from "react";
import { Button } from "antd";
import { Maximize2, Minimize2, RotateCw, Search } from "lucide-react";
import { useTranslation } from "react-i18next";

export interface ListPagePanelProps {
  title?: ReactNode;
  toolbar?: { actions?: ReactNode; onRefresh?: () => void; search?: ReactNode };
  children: ReactNode;
}

/** Page layout and toolbar, independent of the Table API. */
export function ListPagePanel({
  title: listTitle,
  toolbar,
  children,
}: ListPagePanelProps): ReactElement {
  const { t } = useTranslation();
  const [fullscreen, setFullscreen] = useState(false);
  const [searchVisible, setSearchVisible] = useState(true);

  return (
    <div
      className={`list-table-render-wrapper ${fullscreen ? "is-fullscreen" : ""}`}
    >
      {toolbar?.search && searchVisible ? (
        <div role="search" className="list-table-toolbar-search">
          {toolbar.search}
        </div>
      ) : null}
      <div className="list-table-panel">
        {listTitle || toolbar ? (
          <div className="list-table-panel-header">
            <div className="list-table-panel-title">{listTitle}</div>
            <div className="list-table-panel-actions">
              <div className="list-table-toolbar-actions">
                {toolbar?.actions}
              </div>
              <div className="list-table-toolbar-tools">
                {toolbar?.search ? (
                  <Button
                    type="text"
                    aria-label={searchVisible ? t("common.hideSearch") : t("common.showSearch")}
                    title={searchVisible ? t("common.hideSearch") : t("common.showSearch")}
                    icon={<Search size={15} />}
                    onClick={() => setSearchVisible((value) => !value)}
                  />
                ) : null}
                {toolbar?.onRefresh ? (
                  <Button
                    type="text"
                    aria-label={t("common.refresh")}
                    title={t("common.refresh")}
                    icon={<RotateCw size={15} />}
                    onClick={toolbar.onRefresh}
                  />
                ) : null}
                <Button
                  type="text"
                  aria-label={fullscreen ? t("common.exitFullscreen") : t("common.fullscreen")}
                  title={fullscreen ? t("common.exitFullscreen") : t("common.fullscreen")}
                  icon={
                    fullscreen ? (
                      <Minimize2 size={15} />
                    ) : (
                      <Maximize2 size={15} />
                    )
                  }
                  onClick={() => setFullscreen((value) => !value)}
                />
              </div>
            </div>
          </div>
        ) : null}
        <div className="list-table-main-wrapper">{children}</div>
      </div>
    </div>
  );
}
