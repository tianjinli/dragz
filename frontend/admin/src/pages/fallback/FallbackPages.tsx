import { Button, Result } from 'antd';
import { CloudOff, Construction, ServerCrash } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';

function Fallback({ code, description, icon, title }: { code?: string; description: string; icon?: React.ReactNode; title: string }) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  return <div className="fallback-page"><Result status={code === '403' ? '403' : code === '404' ? '404' : code === '500' ? '500' : undefined} icon={icon} title={code || title} subTitle={description} extra={<Button type="primary" onClick={() => navigate('/analytics')}>{t('common.backHome')}</Button>}/></div>;
}
const useFallbackText = () => {
  const { t } = useTranslation();
  return {
    forbiddenTitle: t('pages.fallback.forbiddenTitle'),
    forbiddenDesc: t('pages.fallback.forbiddenDesc'),
    notFoundTitle: t('pages.fallback.notFoundTitle'),
    notFoundDesc: t('pages.fallback.notFoundDesc'),
    errorTitle: t('pages.fallback.errorTitle'),
    errorDesc: t('pages.fallback.errorDesc'),
    offlineTitle: t('pages.fallback.offlineTitle'),
    offlineDesc: t('pages.fallback.offlineDesc'),
    comingSoonTitle: t('pages.fallback.comingSoonTitle'),
    comingSoonDesc: t('pages.fallback.comingSoonDesc'),
  };
};
export const ForbiddenPage = () => { const tx = useFallbackText(); return <Fallback code="403" title={tx.forbiddenTitle} description={tx.forbiddenDesc}/>; };
export const NotFoundPage = () => { const tx = useFallbackText(); return <Fallback code="404" title={tx.notFoundTitle} description={tx.notFoundDesc}/>; };
export const InternalErrorPage = () => { const tx = useFallbackText(); return <Fallback code="500" title={tx.errorTitle} description={tx.errorDesc}/>; };
export const OfflinePage = () => { const tx = useFallbackText(); return <Fallback title={tx.offlineTitle} description={tx.offlineDesc} icon={<CloudOff/>}/>; };
export const ComingSoonPage = () => { const tx = useFallbackText(); return <Fallback title={tx.comingSoonTitle} description={tx.comingSoonDesc} icon={<Construction/>}/>; };
export const UnknownComponentPage = ({ component }: { component?: string }) => { const { t } = useTranslation(); return <Fallback title={t('common.unknownComponent')} description={t('common.unknownComponentDesc', { component: component || t('common.emptyComponent') })} icon={<ServerCrash/>}/>; };
