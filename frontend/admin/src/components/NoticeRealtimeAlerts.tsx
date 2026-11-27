import { Button, Tooltip } from 'antd';
import { Bell, Megaphone, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { NoticePriorityTag } from '@/components/NoticePriorityTag';
import { useMessagesStore } from '@/stores/messages';
import { normalizeNoticeType } from '@/utils/notice';

function MarqueeText({ text }: { text: string }) {
  const trackRef = useRef<HTMLDivElement>(null);
  const measureRef = useRef<HTMLSpanElement>(null);
  const [shouldScroll, setShouldScroll] = useState(false);

  useEffect(() => {
    const track = trackRef.current;
    const measure = measureRef.current;
    if (!track || !measure) return;

    const update = () => {
      setShouldScroll(measure.scrollWidth > track.clientWidth + 1);
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(track);
    observer.observe(measure);
    return () => observer.disconnect();
  }, [text]);

  return (
    <div className="notice-marquee-body-track" ref={trackRef}>
      <span aria-hidden className="notice-marquee-body-measure" ref={measureRef}>
        {text}
      </span>
      {shouldScroll ? (
        <div className="notice-marquee-body is-scrolling">
          <span>{text}</span>
          <span aria-hidden>{text}</span>
        </div>
      ) : (
        <span className="notice-marquee-body">{text}</span>
      )}
    </div>
  );
}

/** Urgent notices keep a top marquee; important/urgent also use OS notifications. */
export function NoticeRealtimeAlerts() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const marqueeItems = useMessagesStore((state) => state.marqueeItems);
  const dismissMarquee = useMessagesStore((state) => state.dismissMarquee);

  if (marqueeItems.length === 0) return null;

  return (
    <div className="notice-marquee" role="status">
      <div className="notice-marquee-items">
        {marqueeItems.map((item) => {
          const isNotification = normalizeNoticeType(item.noticeType) === 'notification';
          const Icon = isNotification ? Bell : Megaphone;
          return (
            <button
              className="notice-marquee-item"
              key={item.id}
              onClick={() => {
                dismissMarquee(item.id);
                navigate('/messages/mine');
              }}
              type="button"
            >
              <Icon
                aria-label={
                  isNotification
                    ? t('system.content.inboxNotifications')
                    : t('system.content.inboxNotices')
                }
                className="notice-marquee-icon"
                size={15}
              />
              <NoticePriorityTag priority={item.priority} />
              <strong className="notice-marquee-title">{item.title}</strong>
              {item.content ? <MarqueeText text={item.content} /> : null}
            </button>
          );
        })}
      </div>
      <Tooltip title={t('common.close')}>
        <Button
          aria-label={t('common.close')}
          className="notice-marquee-close"
          icon={<X size={14} />}
          onClick={() => marqueeItems.forEach((item) => dismissMarquee(item.id))}
          size="small"
          type="text"
        />
      </Tooltip>
    </div>
  );
}
