import type { ReactNode } from 'react';

export interface Column<T> {
  key: string;
  header: ReactNode;
  /** Cell renderer. */
  render: (row: T) => ReactNode;
  className?: string;
  headerClassName?: string;
}

interface TableProps<T> {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  isLoading?: boolean;
  error?: string | null;
  emptyMessage?: string;
  onRetry?: () => void;
  /** Renders a highlighted row (e.g. overdue payments). */
  getRowClassName?: (row: T) => string | undefined;
}

/**
 * Responsive table.
 *
 * - `md` and wider: a classic table with horizontal scroll as a last resort.
 * - Mobile: every row becomes a card (first column is the title, the
 *   `actions` column is rendered as a footer, the rest as label/value pairs),
 *   so long names and big amounts wrap instead of breaking the layout.
 */
export function Table<T>({
  columns,
  rows,
  rowKey,
  isLoading = false,
  error = null,
  emptyMessage = 'Нет данных',
  onRetry,
  getRowClassName,
}: TableProps<T>) {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center gap-3 px-4 py-16 text-sm text-gray-500">
        <span className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-blue-600" />
        Загрузка данных…
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center gap-3 px-4 py-16 text-center">
        <p className="break-words text-sm text-red-600">{error}</p>
        {onRetry ? (
          <button
            type="button"
            onClick={onRetry}
            className="text-sm font-medium text-blue-600 hover:text-blue-700"
          >
            Повторить
          </button>
        ) : null}
      </div>
    );
  }

  if (rows.length === 0) {
    return (
      <div className="px-4 py-16 text-center text-sm text-gray-500">{emptyMessage}</div>
    );
  }

  const [titleColumn, ...restColumns] = columns;
  const actionsColumn = restColumns.find((column) => column.key === 'actions');
  const detailColumns = restColumns.filter((column) => column.key !== 'actions');

  return (
    <>
      {/* Mobile: cards */}
      <ul className="divide-y divide-gray-100 md:hidden">
        {rows.map((row) => (
          <li
            key={rowKey(row)}
            className={['space-y-3 px-4 py-4', getRowClassName?.(row) ?? ''].join(' ')}
          >
            {titleColumn ? (
              <div className="min-w-0 break-words text-sm [overflow-wrap:anywhere]">
                {titleColumn.render(row)}
              </div>
            ) : null}
            {detailColumns.length > 0 ? (
              <dl className="grid grid-cols-1 gap-x-4 gap-y-2 text-sm min-[420px]:grid-cols-2">
                {detailColumns.map((column) => (
                  <div key={column.key} className="min-w-0">
                    <dt className="text-[11px] font-semibold uppercase tracking-wide text-gray-500">
                      {column.header}
                    </dt>
                    <dd className="mt-0.5 min-w-0 break-words [overflow-wrap:anywhere]">
                      {column.render(row)}
                    </dd>
                  </div>
                ))}
              </dl>
            ) : null}
            {actionsColumn ? (
              <div className="flex flex-wrap justify-end gap-2">{actionsColumn.render(row)}</div>
            ) : null}
          </li>
        ))}
      </ul>

      {/* Tablet / desktop: table */}
      <div className="hidden w-full overflow-x-auto md:block">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-gray-200 bg-gray-50">
              {columns.map((column) => (
                <th
                  key={column.key}
                  scope="col"
                  className={[
                    'whitespace-nowrap px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500',
                    column.headerClassName ?? '',
                  ].join(' ')}
                >
                  {column.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr
                key={rowKey(row)}
                className={[
                  'border-b border-gray-100 last:border-b-0',
                  getRowClassName?.(row) ?? '',
                ].join(' ')}
              >
                {columns.map((column, index) => (
                  <td
                    key={column.key}
                    className={[
                      'px-4 py-3 align-middle',
                      // The first (title) column absorbs long text by wrapping.
                      index === 0 ? 'min-w-[12rem] max-w-md break-words [overflow-wrap:anywhere]' : '',
                      column.className ?? '',
                    ].join(' ')}
                  >
                    {column.render(row)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
