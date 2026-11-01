import { useEffect, useMemo, useState } from 'react';
import { Card, Col, Progress, Row, Skeleton, Statistic, Table, Tag, Typography } from 'antd';
import { Activity, CircleDollarSign, ShoppingCart, Users } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { getDashboardApi } from '@/api/system';
import { Page } from '@/components/Page';
import { useAccessStore } from '@/stores/access';

const colors = ['#5b8ff9', '#5ad8a6', '#5d7092', '#f6bd16', '#e8684a', '#6dc8ec', '#9270ca'];
function number(value: unknown) { return typeof value === 'number' ? value : Number(value || 0); }
export function AnalyticsPage() {
  const { t } = useTranslation();
  const [data, setData] = useState<Record<string, unknown>>({}); const [loading, setLoading] = useState(true); const user = useAccessStore((state) => state.userInfo);
  useEffect(() => {
    let cancelled = false;
    void getDashboardApi().then((value) => {
      if (!cancelled) setData(value);
    }).finally(() => {
      if (!cancelled) setLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, []);
  const stats = useMemo(() => [
    { icon: <Users/>, label: t('pages.dashboard.userCount'), value: number(data.userCount ?? data.totalUsers), color: '#1677ff' },
    { icon: <Activity/>, label: t('pages.dashboard.activeToday'), value: number(data.activeUsers ?? data.todayActive), color: '#52c41a' },
    { icon: <ShoppingCart/>, label: t('pages.dashboard.visitsToday'), value: number(data.todayVisits ?? data.visitCount), color: '#faad14' },
    { icon: <CircleDollarSign/>, label: t('pages.dashboard.messages'), value: number(data.noticeCount ?? data.messageCount), color: '#722ed1' },
  ], [data, t]);
  const trend = Array.isArray(data.trend) ? data.trend as Array<Record<string, unknown>> : Array.from({ length: 12 }, (_, i) => ({ label: t('pages.dashboard.month', { month: i + 1 }), value: Math.round(35 + Math.sin(i / 1.5) * 20 + i * 3) }));
  const sources = [
    t('pages.dashboard.sourceDirect'),
    t('pages.dashboard.sourceSearch'),
    t('pages.dashboard.sourceEmail'),
    t('pages.dashboard.sourceAffiliate'),
    t('pages.dashboard.sourceVideo'),
  ];
  return <Page title={t('pages.dashboard.greeting', { name: user?.realName || user?.username || t('pages.dashboard.greetingFallback') })} description={t('pages.dashboard.weather')}>
    <Skeleton loading={loading} active><Row gutter={[16,16]}>{stats.map((item) => <Col xs={24} sm={12} xl={6} key={item.label}><Card className="stat-card"><div className="stat-icon" style={{ background: `${item.color}1c`, color: item.color }}>{item.icon}</div><Statistic title={item.label} value={item.value}/><div className="stat-growth">{t('pages.dashboard.vsYesterday')} <Tag color="success">+12.5%</Tag></div></Card></Col>)}</Row>
    <Row gutter={[16,16]} className="dashboard-row"><Col xs={24} xl={16}><Card title={t('pages.dashboard.trend')} extra={<Typography.Text type="secondary">{t('pages.dashboard.trendRange')}</Typography.Text>}><div className="bar-chart">{trend.map((item, index) => <div className="bar-item" key={String(item.label)}><div className="bar-value" style={{ height: `${Math.max(8, number(item.value))}%`, background: colors[index % colors.length] }} title={String(item.value)}/><span>{String(item.label)}</span></div>)}</div></Card></Col><Col xs={24} xl={8}><Card title={t('pages.dashboard.sources')}><div className="source-list">{sources.map((label,index) => <div key={label}><span>{label}</span><Progress percent={[36,28,18,12,6][index]} strokeColor={colors[index]} showInfo={false}/></div>)}</div></Card></Col></Row>
    <Row gutter={[16,16]}><Col span={24}><Card title={t('pages.dashboard.activities')}><Table pagination={false} size="small" rowKey="id" dataSource={Array.isArray(data.activities) ? data.activities as Array<Record<string, unknown>> : []} columns={[{title:t('pages.dashboard.activityUser'),dataIndex:'user'},{title:t('pages.dashboard.activityAction'),dataIndex:'action'},{title:t('pages.dashboard.activityTime'),dataIndex:'time'}]}/></Card></Col></Row></Skeleton>
  </Page>;
}
