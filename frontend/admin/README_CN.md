# 前端管理后台（dragz-admin）

[English](README.md) | **中文**

React 19 + TypeScript + antd 6 + Vite 的后台壳。所有业务页面都是「后端菜单驱动」的：
后端 `/auth/routes` 返回菜单树，前端按 `component` 字段（形如 `/system/user/index`）
去 `src/router/page-map.tsx` 里查真正的组件。前端不硬编码路由，也不做菜单裁剪。

## 技术栈

| 关注点 | 选型 |
| --- | --- |
| 框架 | React 19 + TypeScript 6（`tsc -b` 严格模式） |
| 构建 | Vite 8（rolldown） |
| UI | antd 6 + `@ant-design/icons` |
| 表格 | 自封装 `components/vxe-table`（基于 vxe-table） |
| 状态 | zustand 5（`persist` 落 localStorage） |
| 请求 | axios（`src/api/request.ts`，统一信封 + 自动续期） |
| 国际化 | i18next + react-i18next（`src/i18n.ts`） |
| 测试 | vitest 4 + jsdom + @testing-library/react |
| Lint | oxlint（无 eslint/prettier 配置文件，格式靠约定） |

## 常用命令

```bash
pnpm install          # 装依赖
pnpm dev              # 开发服务器 :5279，/api 代理到 127.0.0.1:5280
pnpm typecheck        # tsc -b --pretty false，CI 门禁之一
pnpm lint             # oxlint src
pnpm check:i18n       # 文案门禁：硬编码中文 + 中英 key 对齐
pnpm test             # vitest run（一次性跑完，非 watch）
pnpm build            # tsc -b && vite build，产物在 dist/
pnpm preview          # 预览 dist/
```

连别的后端：`VITE_API_TARGET=https://xxx pnpm dev`。**不要把远端地址写进
`vite.config.ts` 提交**，否则所有人 `pnpm dev` 都会直连那个环境。

`pnpm build` 前建议先把旧 `dist/` 挪走（`mv dist ../../_attic/dist-xxx`）。
本机 safe-delete 保护会在文件数 > 50 时拦截 `rm -rf dist`。

## 目录结构

```
src/
├── api/            接口层：request.ts（信封/续期/错误提示）+ 各模块 API
├── components/     通用组件：ResourceCrudPage、DataTablePage、TablePanel、
│                   FormDrawer、ResizableTable、StorageImageUpload、vxe-table/
├── constants/      静态常量（auth.ts 头像路径、layout.ts 布局尺寸）
├── hooks/          useMixedMenu（混合菜单树打平）
├── layout/         AdminLayout / AuthLayout / TabsBar / CachedViews 等壳组件
├── locales/        zh.json / en.json 语言包（唯一文案来源）
├── pages/          业务页面，按模块分目录
├── router/         page-map.tsx（菜单 component → 组件）、menu-utils.tsx
├── stores/         zustand：access（权限码/菜单/用户）、tabs、preferences、locale…
├── styles/         全局样式与主题变量
├── test/           vitest setup
├── types/          index.ts：与后端 DTO 对齐的类型
└── utils/          纯函数（分页、时间、表格列、压缩、重定向…）
```

## 一个页面是怎么出现的

1. **后端**注册路由时声明 `Permission`，启动同步把接口写成 `sys_menu` 里的按钮；
   页面菜单本身由种子 SQL 或管理后台「菜单管理」维护。
2. **后端** `/auth/routes` 返回当前用户可见的菜单树（停用插件的菜单已被裁掉）。
3. **前端** `AdminLayout` 把菜单树渲染成侧边栏，同时把每个节点的 `component`
   交给 `page-map.tsx` 查组件，`lazy()` 动态 import。
4. 菜单里没有的 `component` 会落到 `pages/fallback`，不会白屏。

所以新增一个页面**不需要动路由表**，只需要：

- 在 `pages/<模块>/` 下写组件并具名导出；
- 在 `router/page-map.tsx` 里登记 `'/system/xxx/index': named(() => import('@/pages/...'), 'XxxPage')`；
- 在「菜单管理」里加一条页面菜单，`component` 填同一个路径；
- 页面里用到的按钮权限码，由后端路由同步生成，角色授权页勾选即可。

`page-map.test.tsx` 里维护了后端菜单用到的全部 `component` 清单，并断言它们在
`pageMap` 中都有登记。新增菜单忘了登记，这个用例会直接失败。

## 通用 CRUD 脚手架

大多数列表页不用手写表格和抽屉，直接用 `components/ResourceCrudPage`：

```tsx
<ResourceCrudPage
  title="用户"
  endpoint="/system/user"
  columns={columns}
  fields={fields}
  createCode="system:user:create"
  updateCode="system:user:update"
  deleteCode="system:user:delete"
  paged
/>
```

`endpoint` 是资源根路径，组件据此调 `GET/POST ${endpoint}`、
`PUT ${endpoint}/:id`、`PUT ${endpoint}/:id/status`、`DELETE ${endpoint}/:id`；
其余开关见 `Props`（`tree`、`paged`、`cascadeStatus`、`loadDetailOnEdit`、`rowActions`、
`sidePanel` 等）。

它内置了分页、搜索区、工具栏、行操作（`TableActionGroup` 溢出折叠）、
新增/编辑抽屉（`FormDrawer`）和权限码驱动的按钮显隐。页面里只剩列定义和表单字段。

## 权限控制

只提供一个命令式判断，没有 `<Auth>` 之类的声明式包装组件：

```tsx
import { hasAccessByCodes } from '@/stores/access';

const canCreate = hasAccessByCodes(['system:user:create']);
// 常见用法：拼 actions 数组、决定按钮是否渲染
```

- 权限码就是后端 `sys_menu.code`（按钮类型那一行），与接口 `Permission` 一一对应。
- `accessCodes` 含 `*` 或用户是超级管理员时一律放行。
- 前端隐藏按钮只是体验优化，**真正的拦截在后端 Casbin**，不要在前端做安全假设。

## 国际化（i18n）

**规则：界面上的每一段文字都来自语言包，代码里不写死中文或英文。**

- 语言包只有两份：`src/locales/zh.json`、`src/locales/en.json`，**key 集合必须
  完全一致**。`src/i18n.ts` 只负责装载，不放任何文案。
- 语言只有中文和英文两种，列表在 `src/constants/locales.ts`（`LOCALE_CODES` + antd
  语言包映射），必须与后端 `internal/pkg/i18n/languages.go` 的注册表一致 —— 切换
  语言时 `Accept-Language` 发出去的就是这个标签，后端据此返回对应语言的菜单、角色、
  部门等预设数据。
- **界面静态文案只走语言包，不进「系统设置 → 翻译管理」**：后者只维护预设数据
  （菜单、权限按钮、角色、部门、任务、存储）的英文，改按钮/标签/提示就是改代码。
- key 用点分层级：`<命名空间>.<对象>.<字段>`。已有命名空间：`common`、`validation`、
  `auth`、`layout`、`upload`、`shell`、`preferences`、`locale`、`crud`、`vxe`、
  `pages`、`preview`、`system.<模块>.*`（users / settings / org / security / tools /
  content / plugins）。新页面按模块建 `pages.<模块>.*` 或 `system.<模块>.*`。
- 组件里用 hook：

  ```tsx
  const { t } = useTranslation();
  <Button>{t('system.users.create')}</Button>;
  ```

- **非组件模块**（工具函数、常量表、axios 拦截器、zustand store）用
  `import i18n from '@/i18n'` 并在**调用时**取：`i18n.t('validation.email')`。
  不要在模块顶层求值（那会在语言切换后留下旧语言）。
- 插值：`t('auth.lockCountdown', { error, countdown })`，语言包里写
  `"{{error}}，{{countdown}} 秒后可重试"`。需要内嵌 `<strong>`/`<a>` 时用
  `<Trans components={{ ... }}>`，不要用字符串拼接拼 JSX。
- 动态 key 拼接（如 `t('preferences.layoutName.' + value)`）只在后缀来自有限枚举时用；
  更稳的写法是让常量表直接存完整 key。门禁会跳过以 `.` 结尾和用 `+` 拼接的 key，
  所以这类 key 的中英对齐要自己保证。
- 例外标记：`/* i18n-exempt-start: 原因 */ … /* i18n-exempt-end */`，只给
  「不是 UI 文案」的内容用（例如 `router/menu-utils.tsx` 里匹配后端菜单标题的
  图标提示词）。加标记要在 PR 里说明理由。
- 门禁：`pnpm check:i18n` 扫 `src/**/*.{ts,tsx}`（跳过 `*.test.*` 和
  `src/locales/**`），命中「字符串/JSX 文本里的中文」或「key 在任一份语言包里缺失」
  都会失败；`tsc -b` 不检查文案，这条必须单独跑。
- **什么不翻译**：后端菜单标题、按钮权限名、字典项、配置项描述、插件名称与描述、
  公告正文——这些是数据库里的内容，由管理员维护，见
  [backend/docs/CONVENTIONS_CN.md](../../backend/docs/CONVENTIONS_CN.md#文案与-i18n)。
- 语言切换由 `stores/locale.ts` 落 localStorage 并调 `i18n.changeLanguage`；
  新增语言要同时改 `LocaleCode` 类型、`i18n.ts` 的 `resources` 和这份语言包。

## 请求层约定

`src/api/request.ts` 统一处理：

- 信封 `{ code, message, data }`，`code === 0 || code === 200` 视为成功，其余抛错；
- 401 时自动调 `/auth/refresh` 续期，并发请求排队等同一个 refresh（`refreshTokenQueue`）；
- 续期失败、且不是登录/刷新这类公开端点，才弹全局错误提示并登出；
- 业务代码只用 `apiGet/apiPost/apiPut/apiDelete`，不要直接用 axios 实例。

## 测试

- 单测与组件测试放在被测文件旁边，命名 `*.test.ts(x)`；
- `src/test/setup.ts` 里挂了 `@testing-library/jest-dom` 断言，并补齐 jsdom 缺的
  `matchMedia`、`navigator.clipboard`、`getComputedStyle`（antd/vxe-table 需要）；
- 涉及 `message`/`modal` 的断言优先断言 store 或渲染结果，不要断言 toast 文本。

## 已知约束

- **不分片**：`index--*.js` 单包约 840 kB（gzip 244 kB），构建会告警。要优化就从
  `manualChunks` 入手，别为了消警告去改 `chunkSizeWarningLimit`。
- **无 eslint/prettier 配置文件**：格式一致性靠 oxlint + 人工 review，不要擅自加一套
  和现有代码风格冲突的格式化配置。
- **菜单 component 是字符串契约**：前端 `page-map.tsx` 的 key 与数据库
  `sys_menu.component` 必须逐字一致，改名要两边一起改。
