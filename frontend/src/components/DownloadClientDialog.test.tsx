import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { DownloadClientDialog } from './DownloadClientDialog';

const mutations = vi.hoisted(() => ({ create: vi.fn(), update: vi.fn(), test: vi.fn() }));
vi.mock('@/lib/queries', () => ({
  useCreateDownloadClient: () => ({ mutateAsync: mutations.create, isPending: false }),
  useUpdateDownloadClient: () => ({ mutateAsync: mutations.update, isPending: false }),
  useTestDownloadClient: () => ({ mutateAsync: mutations.test, isPending: false }),
}));

const sab = (APIPath?: string) => ({ ID: 12, Implementation: 'sabnzbd', Scheme: 'http', Host: 'sab.local', Port: 8080, Type: 'usenet', ...(APIPath ? { APIPath } : {}) });
function renderDialog(client: any = null) {
  return render(<DownloadClientDialog open onOpenChange={vi.fn()} client={client} />);
}
async function testAndSave() {
  fireEvent.click(screen.getByRole('button', { name: /test connection/i }));
  await waitFor(() => expect(mutations.test).toHaveBeenCalled());
  fireEvent.click(screen.getByRole('button', { name: /update|create/i }));
}

beforeAll(() => { Element.prototype.scrollIntoView = vi.fn(); });
afterEach(() => { cleanup(); vi.clearAllMocks(); });

describe('DownloadClientDialog SAB API path', () => {
  it.each([
    ['root', sab('/api'), '/api'],
    ['legacy', sab(), '/sabnzbd/api'],
    ['custom', sab('/prefix/api'), '/prefix/api'],
  ])('loads and persists the %s path without sending a stored secret', async (_name, client, expectedPath) => {
    mutations.test.mockResolvedValue(undefined);
    mutations.update.mockResolvedValue(undefined);
    renderDialog(client);
    expect(screen.getByLabelText('API path')).toHaveValue(expectedPath);
    await testAndSave();
    expect(mutations.test).toHaveBeenCalledWith(expect.objectContaining({ id: 12, apiPath: expectedPath, apiKey: null }));
    expect(mutations.test.mock.calls[0][0]).not.toHaveProperty('APIKey');
    await waitFor(() => expect(mutations.update).toHaveBeenCalledWith(expect.objectContaining({ id: 12, request: expect.objectContaining({ id: 12, apiPath: expectedPath }) })));
  });

  it('defaults new SAB clients to /api and 8080; changing API path resets test feedback', async () => {
    renderDialog();
    fireEvent.click(screen.getByRole('combobox', { name: 'Implementation' }));
    fireEvent.click(await screen.findByRole('option', { name: /SABnzbd/i }));
    expect(screen.getByLabelText('API path')).toHaveValue('/api');
    expect(screen.getByLabelText('Port (optional)')).toHaveValue('8080');
  });

  it('does not include an API path in Transmission requests', async () => {
    mutations.test.mockResolvedValue(undefined);
    renderDialog();
    fireEvent.change(screen.getByLabelText('Host'), { target: { value: 'transmission.local' } });
    fireEvent.click(screen.getByRole('button', { name: /test connection/i }));
    await waitFor(() => expect(mutations.test).toHaveBeenCalled());
    expect(mutations.test.mock.calls[0][0]).not.toHaveProperty('apiPath');
  });
});
