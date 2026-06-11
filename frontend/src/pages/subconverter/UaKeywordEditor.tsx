import { useTranslation } from 'react-i18next';
import { Button, Input, Space } from 'antd';
import { DeleteOutlined, PlusOutlined, StopOutlined } from '@ant-design/icons';

import { normalizeUAKeywords } from './utils';

interface UaKeywordEditorProps {
  addLabel: string;
  value?: string[];
  onChange?: (value: string[]) => void;
}

export default function UaKeywordEditor({ addLabel, value = [], onChange }: UaKeywordEditorProps) {
  const { t } = useTranslation();
  const rows = value.length > 0 ? value : [''];

  const updateAt = (index: number, nextValue: string) => {
    const next = [...value];
    next[index] = nextValue;
    onChange?.(next);
  };

  const canRemoveAt = (index: number) => normalizeUAKeywords(
    value.filter((_, itemIndex) => itemIndex !== index),
  ).length > 0;

  const removeAt = (index: number) => {
    if (!canRemoveAt(index)) return;
    onChange?.(value.filter((_, itemIndex) => itemIndex !== index));
  };

  const renderKeywordRow = (keyword: string, index: number) => {
    const removable = canRemoveAt(index);
    return (
      <Space.Compact key={index} block>
        <Input
          value={keyword}
          placeholder="clash"
          onChange={(event) => updateAt(index, event.target.value)}
        />
        <Button
          disabled={!removable}
          aria-label={removable ? t('delete') : t('pages.subconverter.uaKeywordsRequired')}
          title={removable ? t('delete') : t('pages.subconverter.uaKeywordsRequired')}
          icon={removable ? <DeleteOutlined /> : <StopOutlined />}
          onClick={() => removeAt(index)}
        />
      </Space.Compact>
    );
  };

  return (
    <Space direction="vertical" size={8} className="subconverter-ua-editor">
      {rows.map(renderKeywordRow)}
      <Button
        type="dashed"
        block
        icon={<PlusOutlined />}
        onClick={() => onChange?.([...value, ''])}
      >
        {addLabel}
      </Button>
    </Space>
  );
}
