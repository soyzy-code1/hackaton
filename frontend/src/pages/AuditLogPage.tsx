import { useEffect, useState } from 'react';
import { ClipboardList, RefreshCw } from 'lucide-react';
import { api } from '@/api/client';
import type { AuditLogEntry } from '@/api/types';
import { Button } from '@/components/ui/Button';
import { Select } from '@/components/ui/Select';
import { Table } from '@/components/ui/Table';
import type { Column } from '@/components/ui/Table';

const PAGE_SIZE = 25;
const ACTION_OPTIONS = [
  { value: 'login', label: 'Вход' },
  { value: 'create', label: 'Создание' },
  { value: 'update', label: 'Изменение' },
  { value: 'delete', label: 'Удаление' },
  { value: 'mark_paid', label: 'Оплата' },
  { value: 'mark_unpaid', label: 'Снятие оплаты' },
];

function formatDate(value: string): string {
  if (!value) return '—';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('ru-RU');
}

function actionLabel(action: string): string {
  return ACTION_OPTIONS.find((option) => option.value === action)?.label ?? action;
}

export function AuditLogPage() {
  const [items, setItems] = useState<AuditLogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [action, setAction] = useState('');
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = async (nextPage = page, nextAction = action) => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await api.auditLog.list({
        page: nextPage,
        page_size: PAGE_SIZE,
        ...(nextAction ? { action: nextAction } : {}),
      });
      setItems(response.items);
      setTotal(response.total);
      setPage(response.page);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Не удалось загрузить журнал действий');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void load(1, action);
    // Loading is intentionally triggered when the filter changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [action]);

  const columns: Column<AuditLogEntry>[] = [
    { key: 'created_at', header: 'Дата', render: (row) => formatDate(row.created_at) },
    { key: 'action', header: 'Действие', render: (row) => actionLabel(row.action) },
    { key: 'entity', header: 'Объект', render: (row) => row.entity || '—' },
    { key: 'entity_id', header: 'ID', render: (row) => row.entity_id ?? '—' },
    { key: 'user_id', header: 'Пользователь', render: (row) => row.user_id ?? 'система' },
    { key: 'details', header: 'Описание', render: (row) => row.details || '—' },
  ];

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2">
            <ClipboardList className="h-5 w-5 text-blue-600" aria-hidden="true" />
            <h1 className="text-xl font-semibold text-gray-900">Журнал действий</h1>
          </div>
          <p className="mt-1 text-sm text-gray-500">История ключевых операций администраторов и пользователей</p>
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => void load()}
          isLoading={isLoading}
          leftIcon={<RefreshCw className="h-4 w-4" aria-hidden="true" />}
        >
          Обновить
        </Button>
      </div>

      <section className="card p-4">
        <div className="max-w-xs">
          <Select
            label="Фильтр по действию"
            value={action}
            onChange={(event) => setAction(event.target.value)}
            options={ACTION_OPTIONS}
            placeholder="Все действия"
          />
        </div>
      </section>

      {error ? (
        <div className="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700" role="alert">
          {error}
        </div>
      ) : null}

      <div className="card overflow-hidden">
        <Table
          columns={columns}
          rows={items}
          rowKey={(row) => String(row.id)}
          isLoading={isLoading}
          emptyMessage="Записей пока нет"
        />
      </div>

      <div className="flex flex-col gap-3 text-sm text-gray-500 sm:flex-row sm:items-center sm:justify-between">
        <span>Всего записей: {total}</span>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" disabled={page <= 1 || isLoading} onClick={() => void load(page - 1)}>
            Назад
          </Button>
          <span>Страница {page} из {pageCount}</span>
          <Button variant="secondary" size="sm" disabled={page >= pageCount || isLoading} onClick={() => void load(page + 1)}>
            Далее
          </Button>
        </div>
      </div>
    </div>
  );
}
