import { useCallback, useState } from 'react';
import { Button, Drawer, Tag, Upload, message } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { DataTablePage } from '@/components/DataTablePage';
import { ResourceCrudPage } from '@/components/ResourceCrudPage';
import { downloadFileApi, getFileDetailApi, getMyNoticeApi, listFilesApi, listMyNoticesApi, markNoticeReadApi, publishNoticeApi, purgeFileApi, recycleFileApi, restoreFileApi, revokeNoticeApi, uploadFileApi, type ResourceRecord } from '@/api/system';

export function NoticePage() {
  const { t } = useTranslation();
  const actions = [
    {label:t('system.content.publish'),hidden:(row:ResourceRecord)=>String(row.status)==='published',onClick:async(row:ResourceRecord)=>{await publishNoticeApi(row.id);void message.success(t('system.content.published'));}},
    {label:t('system.content.revoke'),hidden:(row:ResourceRecord)=>String(row.status)!=='published',onClick:async(row:ResourceRecord)=>{await revokeNoticeApi(row.id);void message.success(t('system.content.revoked'));}},
  ];
  return <ResourceCrudPage title={t('system.content.noticeTitle')} endpoint="/system/notices" paged createCode="system:notice:manage" updateCode="system:notice:manage" deleteCode="system:notice:manage" rowActions={actions} columns={[{title:t('system.content.title'),dataIndex:'title',width:260},{title:t('common.type'),dataIndex:'type'},{title:t('common.status'),dataIndex:'status'},{title:t('system.content.publishedAt'),dataIndex:'publishedAt',width:180}]} fields={[{key:'title',label:t('system.content.title'),required:true},{key:'type',label:t('common.type'),type:'select',options:[{label:t('system.content.notification'),value:'notice'},{label:t('system.content.announcement'),value:'announcement'}],required:true},{key:'content',label:t('system.content.content'),type:'textarea',required:true},{key:'priority',label:t('system.content.priority'),type:'number'},{key:'status',label:t('common.status'),type:'select',options:[{label:t('system.content.draft'),value:'draft'},{label:t('system.content.publishedStatus'),value:'published'}]}]}/>;
}

export function MyMessagesPage() {
  const { t } = useTranslation();
  const [current,setCurrent]=useState<ResourceRecord|null>(null); const load=useCallback((q:{keyword:string;page:number;pageSize:number})=>listMyNoticesApi(q),[]);
  return <><DataTablePage title={t('system.content.myMessages')} loadData={load} columns={[{title:t('system.content.title'),dataIndex:'title'},{title:t('common.type'),dataIndex:'type'},{title:t('common.status'),dataIndex:'isRead',render:v=><Tag color={v?'default':'blue'}>{v?t('system.content.read'):t('system.content.unread')}</Tag>},{title:t('system.content.time'),dataIndex:'publishedAt'}]} actions={[{label:t('system.content.view'),onClick:async(row,reload)=>{const detail=await getMyNoticeApi(row.id);setCurrent(detail);if(!row.isRead){await markNoticeReadApi(row.id);await reload();}}}]}/><Drawer open={!!current} title={String(current?.title||t('system.content.messageDetail'))} size={640} onClose={()=>setCurrent(null)}><div className="notice-content">{String(current?.content||'')}</div></Drawer></>;
}

export function FilePage({ recycle = false }: { recycle?: boolean }) {
  const { t } = useTranslation();
  const [reloadKey,setReloadKey]=useState(0); const load=useCallback((q:{keyword:string;page:number;pageSize:number})=>listFilesApi({...q,recycled:recycle, reloadKey}),[recycle,reloadKey]);
  const upload=<Upload showUploadList={false} customRequest={({file,onSuccess,onError})=>{const data=new FormData();data.append('file',file as Blob);void uploadFileApi(data).then(()=>{onSuccess?.({});setReloadKey(k=>k+1);void message.success(t('upload.uploadSuccess'));}).catch(onError);}}><Button type="primary" icon={<UploadOutlined/>}>{t('system.content.uploadFile')}</Button></Upload>;
  const common=[{title:t('system.content.fileName'),dataIndex:'originalName'},{title:t('system.content.size'),dataIndex:'size'},{title:t('common.type'),dataIndex:'mimeType'},{title:t('system.content.uploader'),dataIndex:'uploaderName'},{title:t('system.content.time'),dataIndex:'createdAt'}];
  const actions = recycle ? [{label:t('system.content.restore'),onClick:async(row:ResourceRecord,reload:()=>Promise<void>)=>{await restoreFileApi(row.id);void message.success(t('system.content.restored'));await reload();}},{label:t('system.content.purge'),danger:true,onClick:async(row:ResourceRecord,reload:()=>Promise<void>)=>{await purgeFileApi(row.id);void message.success(t('system.content.purged'));await reload();}}] : [{label:t('system.content.download'),onClick:(row:ResourceRecord)=>downloadFileApi(row.id,String(row.originalName||row.name||'download'))},{label:t('common.delete'),danger:true,onClick:async(row:ResourceRecord,reload:()=>Promise<void>)=>{await recycleFileApi(row.id);void message.success(t('system.content.movedToRecycle'));await reload();}}];
  return <DataTablePage title={recycle?t('system.content.recycleTitle'):t('system.content.fileTitle')} loadData={load} columns={common} actions={actions} extra={recycle?undefined:upload}/>;
}
export const FileRecyclePage = () => <FilePage recycle/>;
export function FileReferencesPage() {
  const { t } = useTranslation();
  const [current,setCurrent]=useState<{file:ResourceRecord;references:ResourceRecord[]}|null>(null);
  const load=useCallback((q:{keyword:string;page:number;pageSize:number})=>listFilesApi({...q,status:'ready'}),[]);
  const columns=[{title:t('system.content.fileName'),dataIndex:'originalName'},{title:t('system.content.referenceCount'),dataIndex:'referenceCount'},{title:t('system.content.uploader'),dataIndex:'uploaderName'},{title:t('system.content.time'),dataIndex:'createdAt'}];
  const actions=[{label:t('system.content.viewReferences'),onClick:async(row:ResourceRecord)=>{setCurrent(await getFileDetailApi(row.id));}}];
  return <><DataTablePage title={t('system.content.referencesTitle')} loadData={load} columns={columns} actions={actions}/><Drawer open={!!current} title={String(current?.file?.originalName||t('system.content.referenceDetail'))} size={640} onClose={()=>setCurrent(null)}>{(current?.references??[]).map(ref=><div key={String(ref.id)}><Tag>{String(ref.businessType)}</Tag>{String(ref.businessId)} · {String(ref.fieldName)}</div>)}{(current?.references??[]).length===0&&<div>{t('system.content.noReferences')}</div>}</Drawer></>;
}
