import type { LayoutType } from '@/types';

const primary = 'var(--primary)';
const muted = 'currentColor';

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <svg aria-hidden="true" fill="none" height="66" viewBox="0 0 104 66" width="104">
      <rect fill={muted} height="66" opacity=".035" rx="4.5" width="104" />
      {children}
    </svg>
  );
}

/** Brand mark square. */
function Logo({ x, y = 6, fill = 'white' }: { x: number; y?: number; fill?: string }) {
  return <rect fill={fill} height="5" opacity={fill === primary ? 1 : 0.92} rx="1" width="5" x={x} y={y} />;
}

/** Compact header widgets: optional left mark + two icons on the right. */
function HeaderIcons({
  x,
  y = 6,
  /** Left header mark (toggle/breadcrumb stub). Pass `0` to hide. */
  leftX,
  color = muted,
  opacity = 0.22,
}: {
  x: number;
  y?: number;
  leftX?: number;
  color?: string;
  opacity?: number;
}) {
  const resolvedLeftX = leftX === undefined ? x - 50 : leftX;
  return (
    <>
      {resolvedLeftX !== 0 ? (
        <rect fill={color} height="4" opacity={opacity} rx="1" width="4" x={resolvedLeftX} y={y} />
      ) : null}
      <rect fill={color} height="4" opacity={opacity} rx="1" width="4" x={x} y={y} />
      <rect fill={color} height="4" opacity={opacity} rx="1" width="4" x={x + 6} y={y} />
    </>
  );
}

/** Vertical sider menu dashes. */
function SiderDashes({
  x,
  y = 20,
  count = 4,
  color = 'white',
  opacity = 0.9,
  width = 8,
}: {
  x: number;
  y?: number;
  count?: number;
  color?: string;
  opacity?: number;
  width?: number;
}) {
  return (
    <>
      {Array.from({ length: count }, (_, index) => (
        <rect
          fill={color}
          height="2"
          key={index}
          opacity={opacity}
          rx="1"
          width={width}
          x={x}
          y={y + index * 8}
        />
      ))}
    </>
  );
}

/** Horizontal primary-menu dashes in a blue header. */
function HeaderMenuDashes({ x, y = 7, count = 4 }: { x: number; y?: number; count?: number }) {
  return (
    <>
      {Array.from({ length: count }, (_, index) => (
        <rect
          fill="white"
          height="2"
          key={index}
          opacity={0.85}
          rx="1"
          width="7"
          x={x + index * 10}
          y={y}
        />
      ))}
    </>
  );
}

/** Light content pane: two small blocks + one larger block. */
function ContentBlocks({ x, y = 18, width, height = 43 }: { x: number; y?: number; width: number; height?: number }) {
  const gap = 4;
  const pad = 5;
  const innerW = width - pad * 2;
  const topH = 8;
  const halfW = (innerW - gap) / 2;
  return (
    <>
      <rect fill={muted} height={height} opacity=".04" rx="2" width={width} x={x} y={y} />
      <rect fill={muted} height={topH} opacity=".11" rx="1.5" width={halfW} x={x + pad} y={y + pad} />
      <rect fill={muted} height={topH} opacity=".11" rx="1.5" width={halfW} x={x + pad + halfW + gap} y={y + pad} />
      <rect
        fill={muted}
        height={Math.max(14, height - pad * 2 - topH - gap)}
        opacity=".09"
        rx="1.5"
        width={innerW}
        x={x + pad}
        y={y + pad + topH + gap}
      />
    </>
  );
}

function HeaderBar({ x = 4, width = 96, fill = muted, opacity = 0.1 }: { x?: number; width?: number; fill?: string; opacity?: number }) {
  return <rect fill={fill} height="11" opacity={opacity} rx="2" width={width} x={x} y="4" />;
}

export function LayoutPreview({ type }: { type: LayoutType }) {
  // 垂直: light left sider + blue logo + gray menu lines; light header to the right
  if (type === 'sidebar-nav') {
    return (
      <Frame>
        <rect fill={primary} height="58" opacity=".92" rx="2" width="23" x="4" y="4" />
        <Logo x={13} />
        <SiderDashes x={11.5} y={20} />
        <HeaderBar x={31} width={69} />
        <HeaderIcons leftX={34} x={88} />
        <ContentBlocks height={43} width={69} x={31} />
      </Frame>
    );
  }

  // 双列菜单: blue icon rail + light secondary sider + light header over content
  if (type === 'sidebar-mixed-nav') {
    return (
      <Frame>
        <rect fill={primary} height="58" opacity=".92" rx="2" width="14" x="4" y="4" />
        <SiderDashes x={7} y={18} />
        <rect fill={muted} height="58" opacity=".1" width="22" x="18" y="4" />
        <HeaderBar x={44} width={56} />
        <HeaderIcons leftX={47} x={88} />
        <ContentBlocks height={43} width={56} x={44} />
      </Frame>
    );
  }

  // 水平: full-width primary header with logo + menu dashes; no sider
  if (type === 'header-nav') {
    return (
      <Frame>
        <HeaderBar fill={primary} opacity={0.92} width={96} x={4} />
        <Logo x={8} />
        <HeaderMenuDashes x={20} />
        <ContentBlocks height={41} width={96} x={4} y={19} />
      </Frame>
    );
  }

  // 侧边导航: full-height blue left sider (logo + dashes); light header to the right
  if (type === 'header-sidebar-nav') {
    return (
      <Frame>
        <HeaderBar fill={muted} opacity={0.1} width={96} x={4} />
        <Logo x={10} />
        <rect fill={primary} height="47" opacity=".92" rx="2" width="24" x="4" y="15" />
        <SiderDashes width={10} x={10.5} y={20} />
        <HeaderIcons leftX={0} x={88} />
        <ContentBlocks height={43} width={69} x={31} />
      </Frame>
    );
  }

  // 混合垂直: full-width blue header (logo + dashes) + light left sider below
  if (type === 'mixed-nav') {
    return (
      <Frame>
        <HeaderBar fill={primary} opacity={0.92} width={96} x={4} />
        <Logo x={10} />
        <HeaderMenuDashes x={28} />
        <rect fill={muted} height="47" opacity=".1" width="20" x="4" y="15" />
        <SiderDashes color={muted} opacity={0.22} width={10} x={9} y={22} />
        <ContentBlocks height={43} width={72} x={28} y={19} />
      </Frame>
    );
  }

  // 混合双列: full-height blue rail (logo + dashes) + light secondary; split header (gray / blue)
  if (type === 'header-mixed-nav') {
    return (
      <Frame>
        <rect fill={primary} height="58" opacity=".92" rx="2" width="14" x="4" y="4" />
        <Logo x={8.5} />
        <SiderDashes x={7} y={20} />
        <HeaderBar opacity={0.1} width={18} x={18} />
        <rect fill={muted} height="47" opacity=".1" width="18" x="18" y="15" />
        <HeaderBar fill={primary} opacity={0.92} width={60} x={40} />
        <HeaderMenuDashes x={46} />
        <HeaderIcons color="white" leftX={0} opacity={0.75} x={90} />
        <ContentBlocks height={43} width={60} x={40} y={19} />
      </Frame>
    );
  }

  return null;
}

export function ContentPreview({ type }: { type: 'compact' | 'wide' }) {
  return (
    <Frame>
      <HeaderBar />
      {type === 'wide' ? (
        <ContentBlocks height={43} width={96} x={4} />
      ) : (
        <>
          <ContentBlocks height={43} width={64} x={20} />
          <path d="M12 39h5m70 0h5" stroke={primary} strokeLinecap="round" strokeWidth="2" />
        </>
      )}
    </Frame>
  );
}
