import { App, Button } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

const CHECK_INTERVAL_MS = 5 * 60_000;
let buildTagPromise: Promise<string> | null = null;

async function fetchBuildTag() {
  const response = await fetch(`/index.html?_=${Date.now()}`, { cache: 'no-store' });
  const text = await response.text();
  const match = text.match(/assets\/index-[^"']+\.js/) || text.match(/<script[^>]+src="([^"]+)"/);
  return match?.[0] || text.slice(0, 200);
}

function getBuildTag() {
  if (!buildTagPromise) {
    buildTagPromise = fetchBuildTag().finally(() => {
      buildTagPromise = null;
    });
  }
  return buildTagPromise;
}

export function CheckUpdates() {
  const { t } = useTranslation();
  const { notification } = App.useApp();
  const [visible, setVisible] = useState(false);
  const baseline = useRef<string>('');
  const notified = useRef(false);

  useEffect(() => {
    let cancelled = false;
    const check = async () => {
      try {
        const tag = await getBuildTag();
        if (cancelled) return;
        if (!baseline.current) {
          baseline.current = tag;
          return;
        }
        if (tag !== baseline.current && !notified.current) {
          notified.current = true;
          setVisible(true);
          notification.info({
            message: t('layout.checkUpdatesTitle'),
            description: t('layout.checkUpdatesDesc'),
            duration: 0,
            btn: (
              <Button
                size="small"
                type="primary"
                onClick={() => {
                  window.location.reload();
                }}
              >
                {t('layout.checkUpdatesAction')}
              </Button>
            ),
          });
        }
      } catch {
        /* ignore network errors during version checks */
      }
    };

    void check();
    const timer = window.setInterval(() => void check(), CHECK_INTERVAL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [notification, t]);

  return visible ? <span className="check-updates-flag" hidden /> : null;
}
