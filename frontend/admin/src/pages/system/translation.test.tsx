import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { StrictMode } from 'react';
import i18n from '@/i18n';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  createResourceApi: vi.fn(),
  deleteResourceApi: vi.fn(),
  deleteTranslationApi: vi.fn(),
  getResourceApi: vi.fn(),
  listResourceApi: vi.fn(),
  listTranslationCandidatesApi: vi.fn(),
  translatePresetTextApi: vi.fn(),
  updateResourceApi: vi.fn(),
  updateResourceStatusApi: vi.fn(),
}));

vi.mock('@/api/system', () => ({
  createResourceApi: apiMocks.createResourceApi,
  deleteResourceApi: apiMocks.deleteResourceApi,
  deleteTranslationApi: apiMocks.deleteTranslationApi,
  getResourceApi: apiMocks.getResourceApi,
  listResourceApi: apiMocks.listResourceApi,
  listTranslationCandidatesApi: apiMocks.listTranslationCandidatesApi,
  translatePresetTextApi: apiMocks.translatePresetTextApi,
  updateResourceApi: apiMocks.updateResourceApi,
  updateResourceStatusApi: apiMocks.updateResourceStatusApi,
}));
vi.mock('@/stores/access', () => ({ hasAccessByCodes: () => true }));

const preferencesState = {
  preferences: {
    custom: { defaultTableSize: 20, enableFormFullscreen: false },
  },
};
vi.mock('@/stores/preferences', () => ({
  usePreferencesStore: Object.assign(
    (selector?: (state: typeof preferencesState) => unknown) => (selector ? selector(preferencesState) : preferencesState),
    { getState: () => preferencesState },
  ),
}));

class ResizeObserverMock {
  disconnect() {}
  observe() {}
  unobserve() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverMock);
vi.mock('@/components/Page', () => ({ Page: ({ children }: { children: React.ReactNode }) => <>{children}</> }));

import { TranslationPage } from './translation';

// One row is one preset entity: the Chinese text its owning table holds plus the
// English record next to it. `id` is the list's display key, never the address
// of a save.
const translatedRow = {
  id: 1000000000000001,
  entityKey: 'menu.dashboard',
  source: '工作台',
  value: 'Dashboard',
  lang: 'en',
  updatedAt: '2026-10-30 10:00:00',
};
const untranslatedRow = {
  id: 1000000000000002,
  entityKey: 'menu.system',
  source: '系统管理',
  value: '',
  lang: 'en',
  updatedAt: '2026-10-30 10:00:00',
};

const rowOf = (element: HTMLElement) => element.closest('.ant-table-row');

function headerLabels(): string[] {
  return Array.from(document.querySelectorAll('thead th')).map((node) => (node.textContent ?? '').trim());
}

async function openEditor(name: string, editLabel: RegExp) {
  const row = rowOf(await screen.findByText(name));
  expect(row).not.toBeNull();
  fireEvent.click(within(row!).getByRole('button', { name: editLabel }));
  await waitFor(() => expect(document.getElementById('source')).not.toBeNull());
}

describe('TranslationPage', () => {
  beforeEach(() => {
    // The page translates as the operator types. The default stub answers with
    // an empty string, which the page ignores, so only the test that asserts on
    // the filled column supplies a value.
    apiMocks.translatePresetTextApi.mockResolvedValue({
      text: '',
      provider: 'stub',
      cached: false,
    });
  });

  afterEach(() => {
    cleanup();
    void i18n.changeLanguage('zh-CN');
    vi.clearAllMocks();
  });

  it('lists the Chinese source and the English text side by side', async () => {
    apiMocks.listResourceApi.mockResolvedValue({ items: [translatedRow], total: 1 });
    render(<StrictMode><TranslationPage /></StrictMode>);

    await waitFor(() => expect(headerLabels()).toContain('中文原文'));
    const headers = headerLabels();
    expect(headers).toContain('英文译文');
    expect(screen.getByText('工作台')).toBeInTheDocument();
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
  });

  it('saves both languages in one request, whatever the interface language is', async () => {
    void i18n.changeLanguage('en-US');
    apiMocks.listResourceApi.mockResolvedValue({ items: [translatedRow], total: 1 });
    render(<StrictMode><TranslationPage /></StrictMode>);

    await waitFor(() => expect(headerLabels()).toContain('Chinese (Source)'));

    await openEditor('menu.dashboard', /Edit/);
    const source = document.getElementById('source') as HTMLInputElement;
    const value = document.getElementById('value') as HTMLInputElement;
    // Both columns are filled from the same row: neither is read-only.
    expect(source.value).toBe('工作台');
    expect(value.value).toBe('Dashboard');

    fireEvent.change(source, { target: { value: '控制台' } });
    fireEvent.change(value, { target: { value: 'Workbench' } });
    fireEvent.click(await screen.findByRole('button', { name: /Save/ }));
    await waitFor(() => expect(apiMocks.updateResourceApi).toHaveBeenCalled());

    const call = apiMocks.updateResourceApi.mock.calls[0];
    expect(call[0]).toBe('/system/translations');
    expect(call[2]).toEqual({ entityKey: 'menu.dashboard', source: '控制台', value: 'Workbench' });
    expect(call[2]).not.toHaveProperty('lang');
  });

  it('marks an entity with no English text and lets it be filled in', async () => {
    apiMocks.listResourceApi.mockResolvedValue({ items: [untranslatedRow], total: 1 });
    render(<StrictMode><TranslationPage /></StrictMode>);

    await waitFor(() => expect(screen.getByText('未翻译')).toBeInTheDocument());

    await openEditor('menu.system', /修改/);
    const value = document.getElementById('value') as HTMLInputElement;
    expect(value.value).toBe('');

    fireEvent.change(value, { target: { value: 'System' } });
    fireEvent.click(await screen.findByRole('button', { name: /保\s*存/ }));
    await waitFor(() => expect(apiMocks.updateResourceApi).toHaveBeenCalled());
    expect(apiMocks.updateResourceApi.mock.calls[0][2]).toEqual({
      entityKey: 'menu.system',
      source: '系统管理',
      value: 'System',
    });
  }, 120000);

  it('adds a translation by picking an entity that has none yet', async () => {
    apiMocks.listResourceApi.mockResolvedValue({ items: [untranslatedRow], total: 1 });
    apiMocks.listTranslationCandidatesApi.mockResolvedValue([
      { entityKey: 'menu.system', source: '系统管理' },
    ]);
    render(<StrictMode><TranslationPage /></StrictMode>);

    await waitFor(() => expect(screen.getByText('未翻译')).toBeInTheDocument());

    fireEvent.click(await screen.findByRole('button', { name: /新\s*增/ }));
    // On create the key is picked from the untranslated entities; the rest of
    // the form is the same two columns as an edit.
    const keyInput = await waitFor(() => {
      const node = document.getElementById('entityKey');
      expect(node).not.toBeNull();
      return node as HTMLInputElement;
    });
    fireEvent.mouseDown(keyInput);
    // The candidate list shows the identifier only — the Chinese text has a
    // column of its own in the form, so repeating it in the option was noise.
    fireEvent.click(
      await waitFor(() => {
        const option = Array.from(
          document.querySelectorAll('.ant-select-item-option'),
        ).find((node) => node.textContent?.trim() === 'menu.system');
        expect(option).toBeDefined();
        return option as HTMLElement;
      }),
    );

    const source = document.getElementById('source') as HTMLInputElement;
    expect(source.value).toBe('系统管理');

    fireEvent.change(source, { target: { value: '系统管理（改）' } });
    fireEvent.change(document.getElementById('value') as HTMLInputElement, {
      target: { value: 'System' },
    });
    fireEvent.click(await screen.findByRole('button', { name: /保\s*存/ }));
    await waitFor(() => expect(apiMocks.createResourceApi).toHaveBeenCalled());
    // A create saves both columns in one request, exactly like an edit.
    expect(apiMocks.createResourceApi.mock.calls[0]).toEqual([
      '/system/translations',
      { entityKey: 'menu.system', source: '系统管理（改）', value: 'System' },
    ]);
  }, 120000);

  it('deletes only the translation, and only when the row has one', async () => {
    apiMocks.listResourceApi.mockResolvedValue({
      items: [translatedRow, untranslatedRow],
      total: 2,
    });
    apiMocks.deleteTranslationApi.mockResolvedValue(undefined);
    render(<StrictMode><TranslationPage /></StrictMode>);

    const translated = rowOf(await screen.findByText('menu.dashboard'));
    expect(translated).not.toBeNull();
    // Delete is confirmed first: the row action opens a popconfirm.
    fireEvent.click(within(translated!).getByRole('button', { name: /删除/ }));
    fireEvent.click(await screen.findByRole('button', { name: /确\s*[定认]/ }));
    await waitFor(() =>
      expect(apiMocks.deleteTranslationApi).toHaveBeenCalledWith(
        translatedRow.id,
        'menu.dashboard',
      ),
    );
    // The entity itself is never deleted from this page.
    expect(apiMocks.deleteResourceApi).not.toHaveBeenCalled();

    const untranslated = rowOf(screen.getByText('menu.system'));
    expect(within(untranslated!).getByRole('button', { name: /删除/ })).toBeDisabled();
  }, 120000);

  it('fills the target column from the Chinese source automatically', async () => {
    void i18n.changeLanguage('en-US');
    apiMocks.listResourceApi.mockResolvedValue({ items: [translatedRow], total: 1 });
    apiMocks.translatePresetTextApi.mockResolvedValue({
      text: 'Workbench',
      provider: 'stub',
      cached: false,
    });
    render(<StrictMode><TranslationPage /></StrictMode>);

    await waitFor(() => expect(headerLabels()).toContain('Chinese (Source)'));
    await openEditor('menu.dashboard', /Edit/);
    const source = document.getElementById('source') as HTMLInputElement;
    const value = document.getElementById('value') as HTMLInputElement;
    expect(value.value).toBe('Dashboard');

    // Typing the source is the only input the operator gives; the target column
    // is filled from it after the debounce.
    fireEvent.change(source, { target: { value: '控制台' } });
    await waitFor(
      () => expect(apiMocks.translatePresetTextApi).toHaveBeenCalledWith({ text: '控制台' }),
      { timeout: 5000 },
    );
    await waitFor(() => expect(value.value).toBe('Workbench'), { timeout: 5000 });
  }, 120000);

  it('keeps a hand-written target when the source changes again', async () => {    void i18n.changeLanguage('en-US');
    apiMocks.listResourceApi.mockResolvedValue({ items: [translatedRow], total: 1 });
    apiMocks.translatePresetTextApi.mockResolvedValue({
      text: 'Workbench',
      provider: 'stub',
      cached: false,
    });
    render(<StrictMode><TranslationPage /></StrictMode>);

    await waitFor(() => expect(headerLabels()).toContain('Chinese (Source)'));
    await openEditor('menu.dashboard', /Edit/);
    const source = document.getElementById('source') as HTMLInputElement;
    const value = document.getElementById('value') as HTMLInputElement;

    // A typed translation wins over the one that lands afterwards.
    fireEvent.change(source, { target: { value: '控制台' } });
    fireEvent.change(value, { target: { value: 'My own wording' } });
    await waitFor(() => expect(value.value).toBe('My own wording'), { timeout: 5000 });
    await new Promise((resolve) => setTimeout(resolve, 800));
    expect(value.value).toBe('My own wording');
  }, 120000);

  it('translates on demand from either column, replacing what is there', async () => {
    void i18n.changeLanguage('en-US');
    apiMocks.listResourceApi.mockResolvedValue({ items: [translatedRow], total: 1 });
    apiMocks.translatePresetTextApi
      .mockResolvedValueOnce({ text: 'Workbench', provider: 'stub', cached: false })
      .mockResolvedValueOnce({ text: '控制台', provider: 'stub', cached: false });
    render(<StrictMode><TranslationPage /></StrictMode>);

    await waitFor(() => expect(headerLabels()).toContain('Chinese (Source)'));
    await openEditor('menu.dashboard', /Edit/);
    const source = document.getElementById('source') as HTMLInputElement;
    const value = document.getElementById('value') as HTMLInputElement;
    expect(value.value).toBe('Dashboard');

    // Both columns carry the same button: one in the Chinese field, one in the
    // English field, in that order.
    const buttons = await screen.findAllByRole('button', { name: /Translate/i });
    expect(buttons).toHaveLength(2);

    // The row already has a translation, so the silent pass has done nothing;
    // pressing the button is what replaces it.
    fireEvent.click(buttons[0]);
    await waitFor(() => expect(value.value).toBe('Workbench'), { timeout: 5000 });
    expect(apiMocks.translatePresetTextApi).toHaveBeenLastCalledWith({
      text: '工作台',
      source: 'zh',
      target: 'en',
    });

    // The mirrored direction: English is the source, Chinese gets replaced.
    fireEvent.click(buttons[1]);
    await waitFor(() => expect(source.value).toBe('控制台'), { timeout: 5000 });
    expect(apiMocks.translatePresetTextApi).toHaveBeenLastCalledWith({
      text: 'Workbench',
      source: 'en',
      target: 'zh',
    });
  }, 120000);
});
