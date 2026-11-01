import { useState } from "react";
import { Button, Card, Space, Tag, Typography } from "antd";
import { Trans, useTranslation } from "react-i18next";
import { apiGet } from "@/api/request";
import { Page } from "@/components/Page";

interface PingResult {
  pong: boolean;
  plugin: string;
}

/**
 * Reference page for the example plugin.
 *
 * It exists to prove the whole loop: while the plugin is disabled this page is
 * unreachable (the menu is pruned and the API 404s), and after enabling it in
 * 插件中心 everything works without a restart.
 */
export function ExamplePage() {
  const { t } = useTranslation();
  const [result, setResult] = useState<string>();
  const [loading, setLoading] = useState(false);

  const ping = async () => {
    setLoading(true);
    try {
      const data = await apiGet<PingResult>("/example/ping");
      setResult(JSON.stringify(data));
    } catch (error) {
      setResult(error instanceof Error ? error.message : String(error));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Page title={t("pages.example.title")}>
      <Card className="table-card">
        <Space direction="vertical" size="middle" style={{ width: "100%" }}>
          <Typography.Paragraph>
            <Trans
              i18nKey="pages.example.intro"
              values={{ plugin: "plugins/example" }}
              components={{ tag: <Tag color="blue" /> }}
            />
          </Typography.Paragraph>
          <Space>
            <Button type="primary" loading={loading} onClick={() => void ping()}>
              {t("pages.example.ping")}
            </Button>
            {result ? (
              <Typography.Text code copyable>
                {result}
              </Typography.Text>
            ) : null}
          </Space>
        </Space>
      </Card>
    </Page>
  );
}
