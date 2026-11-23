import { Button, Result } from "antd";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { useAccessStore } from "@/stores/access";

/** Super administrators only. Everyone else sees 403 and can go home. */
export function SuperAdminDenied() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const homePath = useAccessStore((state) => state.userInfo?.homePath);

  return (
    <Result
      status="403"
      title="403"
      subTitle={t("platform.forbidden")}
      extra={
        <Button type="primary" onClick={() => navigate(homePath || "/analytics")}>
          {t("common.backHome")}
        </Button>
      }
    />
  );
}
