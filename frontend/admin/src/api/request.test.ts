import { afterEach, describe, expect, it, vi } from 'vitest';
import { apiGet, requestClient } from './request';

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
