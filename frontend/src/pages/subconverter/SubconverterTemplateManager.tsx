import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Form, Input, Modal, Popconfirm, Space, Spin, Tag, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import type { SubconverterTemplateInput, SubconverterTemplateRecord } from '@/schemas/subconverter';
import { useDatepicker } from '@/hooks/useDatepicker';
import { IntlUtil } from '@/utils';

interface Props {
  templates: SubconverterTemplateRecord[];
  activeId: string;
  disabled: boolean;
  pending: boolean;
  onSave: (id: string | undefined, input: SubconverterTemplateInput) => Promise<unknown>;
  onDelete: (id: string) => Promise<unknown>;
  onRefresh: (id: string) => Promise<unknown>;
}

export function templateDisplayName(
  template: Pick<SubconverterTemplateRecord, 'id' | 'name'>,
  t: (key: string) => string,
) {
  if (template.id === 'general') return t('pages.subconverter.templateGeneral');
  if (template.id === 'aggressive') return t('pages.subconverter.templateAggressive');
  return template.name;
}

export default function SubconverterTemplateManager({
  templates,
  activeId,
  disabled,
  pending,
  onSave,
  onDelete,
  onRefresh,
}: Props) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();
  const [form] = Form.useForm<SubconverterTemplateInput>();
  const [editor, setEditor] = useState<SubconverterTemplateRecord | 'new' | null>(null);
  const [error, setError] = useState('');
  const [editorError, setEditorError] = useState('');
  const busy = disabled || pending;

  const openEditor = (template: SubconverterTemplateRecord | 'new') => {
    setEditorError('');
    form.resetFields();
    form.setFieldsValue(
      template === 'new'
        ? { name: '', url: '', source: 'remote' }
        : { name: template.name, url: template.url, source: 'remote' },
    );
    setEditor(template);
  };
  const save = async () => {
    let input: SubconverterTemplateInput;
    try {
      input = await form.validateFields();
    } catch {
      return;
    }
    setEditorError('');
    try {
      await onSave(editor && editor !== 'new' ? editor.id : undefined, {
        ...input,
        source: 'remote',
      });
      setEditor(null);
    } catch (reason) {
      setEditorError(
        reason instanceof Error ? reason.message : t('pages.subconverter.templateManageFailed'),
      );
    }
  };
  const run = async (action: () => Promise<unknown>) => {
    setError('');
    try {
      await action();
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : t('pages.subconverter.templateManageFailed'),
      );
    }
  };

  return (
    <Space orientation="vertical" style={{ width: '100%' }} size="middle">
      <Space wrap>
        <Typography.Text strong>{t('pages.subconverter.templateCatalog')}</Typography.Text>
        {pending ? <Spin size="small" /> : null}
        <Button disabled={busy} onClick={() => openEditor('new')}>
          {t('pages.subconverter.templateAdd')}
        </Button>
      </Space>
      <Typography.Text type="secondary">
        {t('pages.subconverter.templateManageHint')}
      </Typography.Text>
      {error ? <Alert type="error" showIcon title={error} /> : null}
      {templates.map((template) => (
        <Space
          orientation="vertical"
          key={template.id}
          size={4}
          style={{
            width: '100%',
            borderBottom: '1px solid var(--ant-color-border-secondary)',
            paddingBottom: 12,
          }}
        >
          <Space wrap>
            <Typography.Text strong>{templateDisplayName(template, t)}</Typography.Text>
            <Tag>
              {t(`pages.subconverter.templateSource${template.builtin ? 'Builtin' : 'Remote'}`)}
            </Tag>
            {template.id === activeId ? (
              <Tag color="blue">{t('pages.subconverter.templateCurrent')}</Tag>
            ) : null}
          </Space>
          <Typography.Text type="secondary" style={{ wordBreak: 'break-all' }}>
            {template.url}
          </Typography.Text>
          <Typography.Text>
            {template.checkedAt
              ? t('pages.subconverter.templateCheckedAt', {
                  time: IntlUtil.formatDate(template.checkedAt, datepicker),
                })
              : t('pages.subconverter.templateCheckUnknown')}
          </Typography.Text>
          <Typography.Text>
            {template.cached
              ? t('pages.subconverter.templateUpdatedAt', {
                  time: IntlUtil.formatDate(template.updatedAt, datepicker),
                })
              : t('pages.subconverter.templateNotCached')}
          </Typography.Text>
          {template.checkedAt && template.checkResult ? (
            <Typography.Text type="success">
              {t(
                `pages.subconverter.templateCheck${template.checkResult === 'updated' ? 'Updated' : 'Unchanged'}`,
              )}
            </Typography.Text>
          ) : null}
          <Space wrap>
            <Button
              size="small"
              icon={<ReloadOutlined />}
              disabled={busy}
              onClick={() => void run(() => onRefresh(template.id))}
            >
              {t('pages.subconverter.templateRefresh')}
            </Button>
            {!template.builtin ? (
              <>
                <Button size="small" disabled={busy} onClick={() => openEditor(template)}>
                  {t('edit')}
                </Button>
                <Popconfirm
                  title={t('pages.subconverter.templateDeleteConfirm')}
                  onConfirm={() => run(() => onDelete(template.id))}
                  okText={t('delete')}
                  cancelText={t('cancel')}
                  disabled={busy || template.id === activeId}
                >
                  <Button size="small" danger disabled={busy || template.id === activeId}>
                    {t('delete')}
                  </Button>
                </Popconfirm>
              </>
            ) : null}
          </Space>
        </Space>
      ))}
      <Modal
        open={editor !== null}
        title={t(`pages.subconverter.template${editor === 'new' ? 'Add' : 'Edit'}`)}
        okText={t('pages.subconverter.templateValidateSave')}
        cancelText={t('cancel')}
        onOk={() => void save()}
        onCancel={() => setEditor(null)}
        confirmLoading={pending}
        okButtonProps={{ disabled }}
        cancelButtonProps={{ disabled: busy }}
        closable={!busy}
        keyboard={!busy}
        mask={{ closable: !busy }}
        destroyOnHidden
      >
        {editorError ? (
          <Alert type="error" showIcon title={editorError} style={{ marginBottom: 16 }} />
        ) : null}
        <Form form={form} layout="vertical" disabled={busy} validateTrigger="onBlur">
          <Form.Item
            name="name"
            label={t('pages.subconverter.templateName')}
            rules={[
              {
                required: true,
                whitespace: true,
                message: t('pages.subconverter.templateNameRequired'),
              },
              { max: 100 },
            ]}
          >
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item
            name="url"
            label={t('pages.subconverter.templateURL')}
            rules={[
              { required: true, message: t('pages.subconverter.templateURLRequired') },
              { max: 2048 },
              {
                validator: (_, value: string) => {
                  if (!value?.trim()) return Promise.resolve();
                  try {
                    const url = new URL(value.trim());
                    if (
                      url.protocol === 'https:' &&
                      url.hostname &&
                      !url.username &&
                      !url.password &&
                      !url.hash
                    )
                      return Promise.resolve();
                  } catch {
                    /* display the localized field error */
                  }
                  return Promise.reject(new Error(t('pages.subconverter.templateURLRequired')));
                },
              },
            ]}
          >
            <Input type="url" maxLength={2048} placeholder="https://" />
          </Form.Item>
          <Typography.Text type="secondary">
            {t('pages.subconverter.templateCustomHint')}
          </Typography.Text>
        </Form>
      </Modal>
    </Space>
  );
}
