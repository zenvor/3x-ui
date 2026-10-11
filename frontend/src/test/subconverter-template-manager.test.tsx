import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';

import { ConfigProvider } from 'antd';

import SubconverterTemplateManager from '@/pages/subconverter/SubconverterTemplateManager';
import type { SubconverterTemplateRecord } from '@/schemas/subconverter';

const record: SubconverterTemplateRecord = {
  id: 'remote-12345678-1234-1234-1234-123456789abc',
  variant: 'remote-12345678-1234-1234-1234-123456789abc',
  name: 'My DNS template',
  source: 'remote',
  builtin: false,
  url: 'https://templates.example/mihomo.yaml',
  cached: true,
  checkedAt: 1234567900000,
  updatedAt: 1234567890000,
  checkResult: 'unchanged',
};
const builtin: SubconverterTemplateRecord = {
  ...record,
  id: 'general',
  variant: 'general',
  name: 'General',
  source: 'builtin',
  builtin: true,
  url: 'https://raw.githubusercontent.com/zenvor/mihomo-config/main/mihomo.yaml',
};
function Harness({
  activeId = 'general',
  pending = false,
  save = vi.fn().mockResolvedValue(undefined),
  remove = vi.fn().mockResolvedValue(undefined),
  refresh = vi.fn().mockResolvedValue(undefined),
} = {}) {
  return (
    <ConfigProvider theme={{ token: { motion: false } }}>
      <SubconverterTemplateManager
        templates={[builtin, record]}
        activeId={activeId}
        disabled={false}
        pending={pending}
        onSave={save}
        onDelete={remove}
        onRefresh={refresh}
      />
    </ConfigProvider>
  );
}

describe('custom Mihomo template catalog', () => {
  it('shows both sources and their separate history while keeping built-ins immutable', () => {
    render(<Harness />);
    expect(screen.getByText('General (default)')).toBeTruthy();
    expect(screen.getByText('My DNS template')).toBeTruthy();
    expect(screen.getAllByText(/Last successful check at/)).toHaveLength(2);
    expect(screen.getAllByText(/Cache updated at/)).toHaveLength(2);
    expect(screen.getAllByRole('button', { name: 'Edit' })).toHaveLength(1);
    expect(screen.getAllByRole('button', { name: 'Delete' })).toHaveLength(1);
  });

  it('validates a name and HTTPS link before creating a remote template', async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    render(<Harness save={save} />);
    fireEvent.click(screen.getByRole('button', { name: 'Add template' }));
    const dialog = screen.getByRole('dialog');
    expect(within(dialog).queryByText('Enter a configuration name')).toBeNull();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Validate and save' }));
    await within(dialog).findByText('Enter a configuration name');
    expect(save).not.toHaveBeenCalled();
    fireEvent.change(within(dialog).getByLabelText('Configuration name'), {
      target: { value: 'My second template' },
    });
    fireEvent.change(within(dialog).getByLabelText('Template URL'), {
      target: { value: 'http://templates.example/new.yaml' },
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Validate and save' }));
    await within(dialog).findByText('Enter a valid HTTPS URL without credentials or a fragment');
    expect(save).not.toHaveBeenCalled();
    fireEvent.change(within(dialog).getByLabelText('Template URL'), {
      target: { value: 'https://templates.example/new.yaml' },
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Validate and save' }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(undefined, {
        name: 'My second template',
        url: 'https://templates.example/new.yaml',
        source: 'remote',
      }),
    );
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('keeps a failed edit open and retains the entered fields for retry', async () => {
    const save = vi.fn().mockRejectedValue(new Error('Template download failed'));
    render(<Harness save={save} />);
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
    const dialog = screen.getByRole('dialog');
    expect((within(dialog).getByLabelText('Configuration name') as HTMLInputElement).value).toBe(
      record.name,
    );
    fireEvent.change(within(dialog).getByLabelText('Configuration name'), {
      target: { value: 'Edited name' },
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Validate and save' }));
    await within(dialog).findByText('Template download failed');
    expect((within(dialog).getByLabelText('Configuration name') as HTMLInputElement).value).toBe(
      'Edited name',
    );
    expect(save).toHaveBeenCalledWith(record.id, expect.objectContaining({ name: 'Edited name' }));
  });

  it('blocks deleting the active template and refreshes the requested entry', async () => {
    const refresh = vi.fn().mockResolvedValue(undefined);
    render(<Harness activeId={record.id} refresh={refresh} />);
    expect((screen.getByRole('button', { name: 'Delete' }) as HTMLButtonElement).disabled).toBe(
      true,
    );
    fireEvent.click(screen.getAllByRole('button', { name: /Refresh Now/ })[1]);
    await waitFor(() => expect(refresh).toHaveBeenCalledWith(record.id));
  });

  it('confirms deletion of an inactive entry and blocks mutations while busy', async () => {
    const remove = vi.fn().mockResolvedValue(undefined);
    const { rerender } = render(<Harness remove={remove} />);
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    await screen.findByText(/Delete this template\?/);
    fireEvent.click(screen.getAllByRole('button', { name: 'Delete' })[1]);
    await waitFor(() => expect(remove).toHaveBeenCalledWith(record.id));
    rerender(<Harness pending />);
    for (const name of ['Add template', 'Edit', 'Delete']) {
      expect((screen.getAllByRole('button', { name })[0] as HTMLButtonElement).disabled).toBe(true);
    }
    for (const button of screen.getAllByRole('button', { name: /Refresh Now/ }))
      expect((button as HTMLButtonElement).disabled).toBe(true);
  });
});
