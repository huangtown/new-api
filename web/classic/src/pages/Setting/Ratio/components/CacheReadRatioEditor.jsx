import React, { useState, useCallback, useRef } from 'react';
import {
  Button,
  Input,
  InputNumber,
  Typography,
  Popconfirm,
} from '@douyinfe/semi-ui';
import { IconPlus, IconDelete } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import CardTable from '../../../../components/common/ui/CardTable';

const { Text } = Typography;

let _idCounter = 0;
const uid = () => `crr_${++_idCounter}`;

function parseJSON(str, fallback) {
  if (!str || !str.trim()) return fallback;
  try {
    return JSON.parse(str);
  } catch {
    return fallback;
  }
}

// Build editable rows from the persisted JSON. The configured group names
// come from `groupNames` (typically derived from GroupRatio) so that operators
// see one row per existing group; rows from the JSON itself are also included
// (in case the cache read config names a group that GroupRatio doesn't).
function buildRows(value, groupNames) {
  const ratioMap = parseJSON(value, {});
  const allNames = new Set([...groupNames, ...Object.keys(ratioMap)]);
  return Array.from(allNames).map((name) => ({
    _id: uid(),
    name,
    ratio: name in ratioMap ? ratioMap[name] : 1,
  }));
}

function serializeRows(rows) {
  const out = {};
  rows.forEach((row) => {
    if (!row.name) return;
    out[row.name] = row.ratio;
  });
  return JSON.stringify(out, null, 2);
}

export default function CacheReadRatioEditor({ value, groupNames, onChange }) {
  const { t } = useTranslation();

  const [rows, setRows] = useState(() => buildRows(value, groupNames));

  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const emitAndSet = useCallback((updater) => {
    setRows((prev) => {
      const next = typeof updater === 'function' ? updater(prev) : updater;
      onChangeRef.current?.(serializeRows(next));
      return next;
    });
  }, []);

  const updateRow = useCallback(
    (id, field, v) => {
      emitAndSet((prev) =>
        prev.map((r) => (r._id === id ? { ...r, [field]: v } : r)),
      );
    },
    [emitAndSet],
  );

  const addRow = useCallback(() => {
    emitAndSet((prev) => {
      const existing = new Set(prev.map((r) => r.name));
      let counter = 1;
      let newName = `group_${counter}`;
      while (existing.has(newName)) {
        counter++;
        newName = `group_${counter}`;
      }
      return [...prev, { _id: uid(), name: newName, ratio: 1 }];
    });
  }, [emitAndSet]);

  const removeRow = useCallback(
    (id) => {
      emitAndSet((prev) => prev.filter((r) => r._id !== id));
    },
    [emitAndSet],
  );

  const columns = [
    {
      title: t('分组名称'),
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: (_, record) => (
        <Input
          size='small'
          value={record.name}
          onChange={(v) => updateRow(record._id, 'name', v)}
        />
      ),
    },
    {
      title: t('缓存读取放大倍率'),
      dataIndex: 'ratio',
      key: 'ratio',
      width: 160,
      render: (_, record) => (
        <InputNumber
          size='small'
          min={0}
          step={0.1}
          precision={2}
          value={record.ratio}
          style={{ width: '100%' }}
          onChange={(v) => updateRow(record._id, 'ratio', v ?? 1)}
        />
      ),
    },
    {
      title: '',
      key: 'actions',
      width: 50,
      render: (_, record) => (
        <Popconfirm
          title={t('确认删除该分组？')}
          onConfirm={() => removeRow(record._id)}
          position='left'
        >
          <Button
            icon={<IconDelete />}
            type='danger'
            theme='borderless'
            size='small'
          />
        </Popconfirm>
      ),
    },
  ];

  return (
    <div>
      <CardTable
        columns={columns}
        dataSource={rows}
        rowKey='_id'
        hidePagination
        size='small'
        empty={<Text type='tertiary'>{t('暂无分组，点击下方按钮添加')}</Text>}
      />
      <div className='mt-3 flex justify-center'>
        <Button icon={<IconPlus />} theme='outline' onClick={addRow}>
          {t('添加分组')}
        </Button>
      </div>
    </div>
  );
}