import { Input, Popover, Tooltip } from 'antd';
import { Grid3X3, Search, X } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { getLucideIconList, iconNode } from '@/router/menu-utils';
import { paginationPageWindow } from '@/utils/pagination';

const PAGE_SIZE = 30;

export interface IconPickerProps {
  'aria-label'?: string;
  disabled?: boolean;
  id?: string;
  onChange?: (value: string | undefined) => void;
  value?: string;
}

function iconName(value?: string) {
  return (value || '').replace(/^lucide:/, '');
}

export function IconPicker({ disabled, id, onChange, value }: IconPickerProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const list = useMemo(() => {
    const needle = keyword.trim().toLowerCase();
    if (!needle) return getLucideIconList();
    return getLucideIconList().filter((item) => item.search.includes(needle));
  }, [keyword]);
  const totalPages = Math.max(1, Math.ceil(list.length / PAGE_SIZE));
  const currentPage = Math.min(page, totalPages);
  const pageItems = list.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE);
  const pageWindow = paginationPageWindow(currentPage, totalPages);
  useEffect(() => { setPage(1); }, [keyword]);

  const select = (next: string | undefined) => { onChange?.(next); setOpen(false); };
  const changePage = (next: number) => setPage(Math.min(Math.max(next, 1), totalPages));

  return (
    <Popover
      placement="bottomLeft"
      arrow={false}
      open={open}
      onOpenChange={setOpen}
      content={
        <div className="icon-picker-popover">
          <Input allowClear prefix={<Search size={14} />} placeholder={t('common.searchIcon')} value={keyword} onChange={(event) => setKeyword(event.target.value)} />
          <div className="icon-picker-grid">
            {pageItems.map((item) => (
              <Tooltip key={item.value} title={iconName(item.value)} mouseEnterDelay={0.2}>
                <button aria-label={iconName(item.value)} className={`icon-picker-item${value === item.value ? ' is-active' : ''}`} onClick={() => select(item.value)} type="button">
                  <item.Component size={18} />
                  <span className="icon-picker-item-name">{iconName(item.value)}</span>
                </button>
              </Tooltip>
            ))}
          </div>
          <div className="icon-picker-pagination">
            <div className="icon-picker-pagination-pages">
              <button aria-label={t('common.firstPage')} disabled={currentPage <= 1} type="button" onClick={() => changePage(1)}>|&lt;</button>
              <button aria-label={t('common.prev5')} disabled={currentPage <= 1} type="button" onClick={() => changePage(currentPage - 5)}>&lt;&lt;</button>
              {pageWindow.map((pageNumber) => <button aria-current={pageNumber === currentPage ? 'page' : undefined} className={pageNumber === currentPage ? 'active' : undefined} key={pageNumber} type="button" onClick={() => changePage(pageNumber)}>{pageNumber}</button>)}
              <button aria-label={t('common.next5')} disabled={currentPage >= totalPages} type="button" onClick={() => changePage(currentPage + 5)}>&gt;&gt;</button>
              <button aria-label={t('common.lastPage')} disabled={currentPage >= totalPages} type="button" onClick={() => changePage(totalPages)}>&gt;|</button>
            </div>
          </div>
        </div>
      }
    >
      <div className={`icon-picker${disabled ? ' is-disabled' : ''}`}>
        <button aria-label={t('common.selectIcon')} className="icon-picker-box" disabled={disabled} id={id} type="button" onClick={() => setOpen((prev) => !prev)}>
          <span className="icon-picker-name">{value ? iconName(value) : <span className="icon-picker-placeholder">{t('common.select')}</span>}</span>
          {value ? <span className="icon-picker-clear" role="button" tabIndex={-1} onClick={(event) => { event.stopPropagation(); select(undefined); }}><X size={14} /></span> : null}
          <span className="icon-picker-icon">{value ? iconNode(value) : <span className="icon-picker-grid-button"><Grid3X3 size={16} /></span>}</span>
        </button>
      </div>
    </Popover>
  );
}
