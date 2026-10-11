import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Form } from 'antd';

import SubconverterSettingsModal from '@/pages/subconverter/SubconverterSettingsModal';
import type {
  SettingsValues,
  TemplateVariant,
  SubconverterTemplateRecord,
} from '@/schemas/subconverter';

const generalURL = 'https://raw.githubusercontent.com/zenvor/mihomo-config/main/mihomo.yaml';
const aggressiveURL =
  'https://raw.githubusercontent.com/zenvor/mihomo-config/main/mihomo-aggressive.yaml';

function Harness({
  savedVariant = 'general',
  statusVariant = savedVariant,
  saving = false,
  refreshing = false,
  error = false,
  loading = false,
  checkedAt = 1234567900000,
  checkResult = 'updated',
  onRefresh = vi.fn(),
  onSave = vi.fn(),
  templates,
  templatesLoading = false,
}: {
  savedVariant?: TemplateVariant;
  statusVariant?: TemplateVariant;
  saving?: boolean;
  refreshing?: boolean;
  error?: boolean;
  loading?: boolean;
  checkedAt?: number;
  checkResult?: '' | 'updated' | 'unchanged';
  onRefresh?: () => void;
  onSave?: (values: SettingsValues) => void;
  templates?: SubconverterTemplateRecord[];
  templatesLoading?: boolean;
}) {
  const [form] = Form.useForm<SettingsValues>();
  return (
    <SubconverterSettingsModal
      open
      saving={saving}
      form={form}
      savedTemplateVariant={savedVariant}
      templateStatus={{
        variant: statusVariant,
        url: statusVariant === 'general' ? generalURL : aggressiveURL,
        cached: true,
        updatedAt: 1234567890000,
        checkedAt,
        checkResult,
      }}
      templateStatusLoading={loading}
      templateStatusError={error}
      templateRefreshing={refreshing}
      onRefreshTemplate={onRefresh}
      templates={templates}
      templatesLoading={templatesLoading}
      onSaveTemplate={async () => {}}
      onDeleteTemplate={async () => {}}
      onRefreshNamedTemplate={async () => {}}
      onOk={() => {
        void form.validateFields().then(onSave);
      }}
      onCancel={() => {}}
    />
  );
}

describe('subconverter template version settings', () => {
  it('keeps the active URL/cache visible until a new choice is saved, and submits it', async () => {
    const save = vi.fn();
    const refresh = vi.fn();
    render(<Harness onSave={save} onRefresh={refresh} />);
    expect(
      (screen.getByRole('radio', { name: 'General (default)' }) as HTMLInputElement).checked,
    ).toBe(true);
    expect(screen.getByText(generalURL)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /Refresh Now/ }));
    expect(refresh).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByRole('radio', { name: 'Aggressive' }));
    await screen.findByText(/Save to download/);
    expect(
      (screen.getByRole('button', { name: /Refresh Now/ }) as HTMLButtonElement).disabled,
    ).toBe(true);
    expect(screen.getByText(generalURL)).toBeTruthy();
    expect(screen.queryByText(aggressiveURL)).toBeNull();
    expect(screen.getByText(/Cache updated at/)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(expect.objectContaining({ templateVariant: 'aggressive' })),
    );
    fireEvent.click(screen.getByRole('radio', { name: 'General (default)' }));
    await waitFor(() =>
      expect(
        (screen.getByRole('button', { name: /Refresh Now/ }) as HTMLButtonElement).disabled,
      ).toBe(false),
    );
  });

  it('hides stale URL and cache time if a previous version status survives a save', () => {
    render(<Harness savedVariant="aggressive" statusVariant="general" loading />);
    expect(screen.getByText('Active template: Aggressive')).toBeTruthy();
    expect(screen.queryByText(generalURL)).toBeNull();
    expect(screen.queryByText(/Cache updated at/)).toBeNull();
    expect(screen.queryByText(/Last successful check/)).toBeNull();
    expect(screen.getByText(/update the subscription in your client/i)).toBeTruthy();
  });

  it('updates the check time while retaining the cache time when the template is unchanged', () => {
    const { rerender } = render(<Harness />);
    const cacheTime = screen.getByText(/Cache updated at/).textContent;
    const checkTime = screen.getByText(/Last successful check at/).textContent;
    expect(screen.getByText('Last successful check: cache updated')).toBeTruthy();
    rerender(<Harness checkedAt={1234568000000} checkResult="unchanged" />);
    expect(screen.getByText(/Cache updated at/).textContent).toBe(cacheTime);
    expect(screen.getByText(/Last successful check at/).textContent).not.toBe(checkTime);
    expect(
      screen.getByText('Last successful check: template unchanged; cache retained'),
    ).toBeTruthy();
  });

  it('shows unknown check history for a legacy cache without inventing a timestamp', () => {
    render(<Harness checkedAt={0} checkResult="" />);
    expect(screen.getByText('Last successful check: not recorded')).toBeTruthy();
    expect(screen.getByText(/Cache updated at/)).toBeTruthy();
    expect(screen.queryByText('Last successful check: cache updated')).toBeNull();
  });

  it('disables editing and refresh while saving', () => {
    render(<Harness saving />);
    expect((screen.getByRole('radio', { name: 'Aggressive' }) as HTMLInputElement).disabled).toBe(
      true,
    );
    expect(
      (screen.getByRole('button', { name: /Refresh Now/ }) as HTMLButtonElement).disabled,
    ).toBe(true);
    expect((screen.getByRole('button', { name: 'Cancel' }) as HTMLButtonElement).disabled).toBe(
      true,
    );
  });

  it('shows an aggressive selection on reopen and reflects fresh status from another admin', () => {
    const { rerender } = render(<Harness savedVariant="aggressive" />);
    expect((screen.getByRole('radio', { name: 'Aggressive' }) as HTMLInputElement).checked).toBe(
      true,
    );
    expect(screen.getByText(aggressiveURL)).toBeTruthy();
    rerender(<Harness savedVariant="aggressive" statusVariant="general" />);
    expect(screen.getByText('Active template: General (default)')).toBeTruthy();
    expect(screen.getByText(generalURL)).toBeTruthy();
    expect(screen.queryByText(aggressiveURL)).toBeNull();
    expect(
      (screen.getByRole('button', { name: /Refresh Now/ }) as HTMLButtonElement).disabled,
    ).toBe(true);
  });

  it('prevents saving while refreshing and allows retry after a status error', () => {
    const { rerender } = render(<Harness refreshing />);
    expect((screen.getByRole('button', { name: 'Save' }) as HTMLButtonElement).disabled).toBe(true);
    rerender(<Harness error />);
    expect(
      (screen.getByRole('button', { name: /Refresh Now/ }) as HTMLButtonElement).disabled,
    ).toBe(false);
  });
});

const customTemplate: SubconverterTemplateRecord = {
  id: 'remote-12345678-1234-1234-1234-123456789abc',
  variant: 'remote-12345678-1234-1234-1234-123456789abc',
  name: 'My custom template',
  source: 'remote',
  builtin: false,
  url: 'https://templates.example/config.yaml',
  cached: true,
  updatedAt: 1234567890000,
  checkedAt: 1234567900000,
  checkResult: 'unchanged',
};

describe('template catalog selection', () => {
  it('submits a custom stable ID without changing the currently active status before saving', async () => {
    const save = vi.fn();
    render(<Harness templates={[customTemplate]} onSave={save} />);
    fireEvent.click(screen.getByRole('radio', { name: customTemplate.name }));
    await screen.findByText(/Save to download/);
    expect(screen.getByText('Active template: General (default)')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(
        expect.objectContaining({ templateVariant: customTemplate.id }),
      ),
    );
  });

  it('preserves editor input when the catalog refetches while the dialog is open', async () => {
    const { rerender } = render(<Harness templates={[customTemplate]} />);
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
    fireEvent.change(screen.getByLabelText('Configuration name'), {
      target: { value: 'Unsaved name' },
    });
    rerender(<Harness templates={[customTemplate]} templatesLoading />);
    expect((screen.getByLabelText('Configuration name') as HTMLInputElement).value).toBe(
      'Unsaved name',
    );
    expect(
      (screen.getByRole('button', { name: 'Validate and save' }) as HTMLButtonElement).disabled,
    ).toBe(true);
    rerender(<Harness templates={[customTemplate]} />);
    expect((screen.getByLabelText('Configuration name') as HTMLInputElement).value).toBe(
      'Unsaved name',
    );
  });
});
