import { useMemo, useState } from 'react';
import { Bell, BellOff, CheckCheck, Filter } from 'lucide-react';
import { api } from '@/api/client';
import type { Notification, PaginatedResponse } from '@/api/types';
import { useAsyncResource } from '@/hooks/useAsyncResource';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { ErrorMessage } from '@/components/ui/ErrorMessage';

const EMPTY_PAGE: PaginatedResponse<Notification> = {
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
};

function formatCreatedAt(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString('ru-RU');
}

export function NotificationsPage() {
  const [unreadOnly, setUnreadOnly] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const resource = useAsyncResource<PaginatedResponse<Notification>>(
    () => api.notifications.list({ unread_only: unreadOnly, page: 1, page_size: 50 }),
    [unreadOnly],
  );
  const page = resource.data ?? EMPTY_PAGE;
  const items = Array.isArray(page.items) ? page.items : [];
  const unreadCount = useMemo(() => items.filter((item) => !item.is_read).length, [items]);

  const runAction = async (action: () => Promise<unknown>) => {
    setActionError(null);
    try {
      await action();
      resource.refresh();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Не удалось обновить уведомления');
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">Уведомления</h1>
          <p className="text-sm text-gray-500">Напоминания, просрочки и события аккаунта</p>
        </div>
        <div className="grid grid-cols-1 gap-2 min-[420px]:grid-cols-2 sm:flex">
          <Button size="sm" variant={unreadOnly ? 'primary' : 'secondary'} onClick={() => setUnreadOnly((value) => !value)} leftIcon={<Filter className="h-4 w-4" />}>
            Только непрочитанные{unreadCount > 0 ? ` (${unreadCount})` : ''}
          </Button>
          <Button size="sm" variant="secondary" disabled={unreadCount === 0} onClick={() => void runAction(() => api.notifications.markAllRead())} leftIcon={<CheckCheck className="h-4 w-4" />}>
            Прочитать все
          </Button>
        </div>
      </div>

      {actionError ? (
        <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700" role="alert">
          {actionError}
        </div>
      ) : null}

      <div className="card overflow-hidden">
        {resource.isLoading ? (
          <div className="flex items-center justify-center gap-3 p-12 text-sm text-gray-500">
            <span className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-blue-600" />
            Загрузка уведомлений…
          </div>
        ) : resource.error ? (
          <div className="p-6">
            <ErrorMessage message={resource.error} onRetry={resource.refresh} />
          </div>
        ) : items.length === 0 ? (
          <div className="flex flex-col items-center gap-3 px-6 py-14 text-center">
            <div className="rounded-full border border-gray-200 bg-gray-50 p-3">
              <BellOff className="h-6 w-6 text-gray-400" aria-hidden="true" />
            </div>
            <p className="text-sm font-medium text-gray-900">
              {unreadOnly ? 'Непрочитанных уведомлений нет' : 'Уведомлений пока нет'}
            </p>
            <p className="max-w-sm text-sm text-gray-500">
              Здесь появятся напоминания о предстоящих оплатах и предупреждения о просрочках.
            </p>
          </div>
        ) : (
          items.map((item) => (
            <div key={item.id} className={['flex flex-col gap-3 border-b border-gray-100 px-4 py-4 last:border-0 sm:flex-row sm:gap-4 sm:px-5', item.is_read ? '' : 'bg-blue-50/40 dark:bg-blue-950/30'].join(' ')}>
              <div className="hidden self-start rounded-full border border-gray-200 bg-white p-2 sm:block"><Bell className="h-4 w-4 text-blue-600" /></div>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="min-w-0 break-words text-sm font-semibold text-gray-900 [overflow-wrap:anywhere]">{item.title}</p>
                  {!item.is_read ? <Badge variant="info">Новое</Badge> : null}
                </div>
                {item.message ? <p className="mt-1 break-words text-sm text-gray-600 [overflow-wrap:anywhere]">{item.message}</p> : null}
                <p className="mt-2 text-xs text-gray-400">
                  {formatCreatedAt(item.created_at)}
                  {item.channels.length > 0 ? ` · каналы: ${item.channels.join(', ')}` : ''}
                </p>
              </div>
              {!item.is_read ? (
                <Button className="self-end sm:self-start" size="sm" variant="ghost" onClick={() => void runAction(() => api.notifications.markRead(item.id))}>
                  Прочитано
                </Button>
              ) : null}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
