import { afterEach, describe, expect, it, vi } from 'vitest';
import { message } from 'antd';
import { apiGet, requestClient } from './request';

vi.mock('antd', async () => {
  const actual = await vi.importActual<typeof import('antd')>('antd');
  return {
    ...actual,
    message: { ...actual.message, error: vi.fn() },
  };
});

function rejectWith(status: number, url: string, code: number, msg: string) {
  return (config: { url?: string }) => Promise.reject({
    isAxiosError: true,
    config,
    response: { status, data: { code, message: msg } },
  });
}

describe('apiGet request dedupe', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shares concurrent GET requests with the same url and params', async () => {
    const get = vi.spyOn(requestClient, 'get').mockResolvedValue({ ok: true });

    const [first, second] = await Promise.all([
      apiGet('/system/menus', { params: { page: 1, keyword: 'org' } }),
      apiGet('/system/menus', { params: { keyword: 'org', page: 1 } }),
    ]);

    expect(first).toEqual({ ok: true });
    expect(second).toEqual({ ok: true });
    expect(get).toHaveBeenCalledTimes(1);
  });

  it('keeps different GET params isolated', async () => {
    const get = vi.spyOn(requestClient, 'get')
      .mockResolvedValueOnce({ page: 1 })
      .mockResolvedValueOnce({ page: 2 });

    const [first, second] = await Promise.all([
      apiGet('/system/users', { params: { page: 1 } }),
      apiGet('/system/users', { params: { page: 2 } }),
    ]);

    expect(first).toEqual({ page: 1 });
    expect(second).toEqual({ page: 2 });
    expect(get).toHaveBeenCalledTimes(2);
  });
});

describe('requestClient error toast', () => {
  const originalAdapter = requestClient.defaults.adapter;

  afterEach(() => {
    requestClient.defaults.adapter = originalAdapter;
    vi.mocked(message.error).mockClear();
  });

  it('stays silent for /auth/login so the login page owns the single error notice', async () => {
    requestClient.defaults.adapter = rejectWith(400, '/auth/login', 40101, 'bad credentials') as never;

    await expect(requestClient.post('/auth/login', {})).rejects.toBeDefined();
    expect(message.error).not.toHaveBeenCalled();
  });

  it('still toasts for ordinary API errors', async () => {
    requestClient.defaults.adapter = rejectWith(500, '/system/users', 50000, 'boom') as never;

    await expect(requestClient.get('/system/users')).rejects.toBeDefined();
    expect(message.error).toHaveBeenCalled();
  });
});
