import { describe, expect, it } from 'vitest';
import { pageMap } from './page-map';

const backendComponents=['/dashboard/analytics/index','/system/user/index','/system/department/index','/system/position/index','/system/role/index','/system/menu/index','/system/dict/index','/system/config/index','/system/notice/index','/system/message/index','/system/file/index','/system/file/recycle','/system/security/online-user','/system/security/token','/system/log/login','/system/log/operation','/system/log/error','/system/job/index','/system/webhook/index','/system/cache/index','/system/plugin/index','/example/index'];
describe('backend component map',()=>{it.each(backendComponents)('maps %s to an explicit lazy React component',(component)=>{expect(pageMap[component]).toBeDefined();});});
