import { useRef, useState } from 'react';
import type { ChangeEvent } from 'react';
import { AlertCircle, CheckCircle2, Download, FileUp } from 'lucide-react';
import { api } from '@/api/client';
import type { CsvImportPreview } from '@/api/types';
import { PAYMENT_CATEGORY_LABELS } from '@/api/types';
import { downloadCsv } from '@/lib/finance';
import { formatAmount, formatDate } from '@/lib/date';
import { CSV_TEMPLATE_EXAMPLE, CSV_TEMPLATE_HEADER, parsePaymentsCsv } from '@/lib/planning';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';

interface CsvImportModalProps {
  isOpen: boolean;
  onClose: () => void;
  /** Usernames that may appear in the `assigned_usernames` column. */
  knownUsernames: string[];
  onImported: (count: number) => void;
}

const MAX_FILE_SIZE = 1024 * 1024;

/**
 * Bulk import of recurring payments from a CSV template.
 * The file is parsed and validated in the browser; only valid rows are sent
 * to `POST /recurring-payments/import` in a single request.
 */
export function CsvImportModal({ isOpen, onClose, knownUsernames, onImported }: CsvImportModalProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [fileName, setFileName] = useState('');
  const [preview, setPreview] = useState<CsvImportPreview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isImporting, setIsImporting] = useState(false);

  const reset = () => {
    setFileName('');
    setPreview(null);
    setError(null);
    if (inputRef.current) inputRef.current.value = '';
  };

  const close = () => {
    if (isImporting) return;
    reset();
    onClose();
  };

  const downloadTemplate = () => {
    downloadCsv('invoice-keeper-import-template.csv', [CSV_TEMPLATE_HEADER, ...CSV_TEMPLATE_EXAMPLE]);
  };

  const handleFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    setError(null);
    setPreview(null);
    if (!file) return;
    if (file.size > MAX_FILE_SIZE) {
      setError('Файл больше 1 МБ — разбейте его на части');
      return;
    }
    if (!/\.(csv|txt)$/i.test(file.name)) {
      setError('Поддерживается CSV. Из Excel сохраните файл как «CSV UTF-8».');
      return;
    }
    setFileName(file.name);
    const text = await file.text();
    setPreview(parsePaymentsCsv(text, knownUsernames));
  };

  const handleImport = async () => {
    if (!preview || preview.valid_rows.length === 0) return;
    setIsImporting(true);
    setError(null);
    try {
      const created = await api.recurringPayments.importBatch(preview.valid_rows);
      onImported(created.length);
      reset();
      onClose();
    } catch (importError) {
      setError(importError instanceof Error ? importError.message : 'Не удалось импортировать платежи');
    } finally {
      setIsImporting(false);
    }
  };

  const validCount = preview?.valid_rows.length ?? 0;
  const errorCount = preview?.errors.length ?? 0;

  return (
    <Modal
      isOpen={isOpen}
      title="Импорт платежей из CSV"
      onClose={close}
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={close} disabled={isImporting}>
            Отмена
          </Button>
          <Button
            onClick={() => void handleImport()}
            isLoading={isImporting}
            disabled={validCount === 0}
          >
            {validCount > 0 ? `Импортировать ${validCount}` : 'Импортировать'}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <ol className="space-y-1 text-sm text-gray-600">
          <li>1. Скачайте шаблон и заполните его в Excel или Google Таблицах.</li>
          <li>2. Сохраните как «CSV UTF-8» и загрузите файл ниже.</li>
          <li>3. Проверьте строки — с ошибками не импортируются.</li>
        </ol>

        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" size="sm" onClick={downloadTemplate} leftIcon={<Download className="h-4 w-4" />}>
            Скачать шаблон
          </Button>
          <Button size="sm" variant="secondary" onClick={() => inputRef.current?.click()} leftIcon={<FileUp className="h-4 w-4" />}>
            {fileName ? 'Выбрать другой файл' : 'Выбрать файл'}
          </Button>
          <input
            ref={inputRef}
            type="file"
            accept=".csv,text/csv,text/plain"
            className="hidden"
            onChange={(event) => void handleFile(event)}
            aria-label="CSV-файл с платежами"
          />
          {fileName ? <span className="self-center text-sm text-gray-500">{fileName}</span> : null}
        </div>

        <p className="text-xs text-gray-500">
          Колонки: <code>name</code>, <code>amount</code>, <code>currency</code> (RUB/USD/EUR),{' '}
          <code>next_payment_date</code> (ГГГГ-ММ-ДД или ДД.ММ.ГГГГ), <code>periodicity</code> (monthly),{' '}
          <code>category</code>, <code>assigned_usernames</code> (через «;»), <code>notes</code>.
        </p>

        {error ? (
          <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700" role="alert">
            {error}
          </div>
        ) : null}

        {preview ? (
          <div className="space-y-3">
            <div className="flex flex-wrap gap-2 text-sm">
              <span className="inline-flex items-center gap-1.5 rounded-md border border-green-200 bg-green-50 px-2 py-1 text-green-700">
                <CheckCircle2 className="h-4 w-4" aria-hidden="true" /> Готово к импорту: {validCount}
              </span>
              {errorCount > 0 ? (
                <span className="inline-flex items-center gap-1.5 rounded-md border border-red-200 bg-red-50 px-2 py-1 text-red-700">
                  <AlertCircle className="h-4 w-4" aria-hidden="true" /> С ошибками: {errorCount}
                </span>
              ) : null}
            </div>

            {errorCount > 0 ? (
              <ul className="max-h-32 space-y-1 overflow-y-auto rounded-md border border-red-200 p-2 text-xs text-red-700">
                {preview.errors.map((item) => (
                  <li key={`${item.row}-${item.message}`}>
                    <span className="font-medium">Строка {item.row}:</span> {item.message}
                  </li>
                ))}
              </ul>
            ) : null}

            {validCount > 0 ? (
              <div className="max-h-56 overflow-auto rounded-md border border-gray-200">
                <table className="w-full min-w-[560px] text-xs">
                  <thead className="sticky top-0 bg-gray-50 text-left text-gray-500">
                    <tr>
                      <th className="px-3 py-2 font-semibold">Название</th>
                      <th className="px-3 py-2 font-semibold">Сумма</th>
                      <th className="px-3 py-2 font-semibold">Дата</th>
                      <th className="px-3 py-2 font-semibold">Категория</th>
                      <th className="px-3 py-2 font-semibold">Пользователи</th>
                    </tr>
                  </thead>
                  <tbody>
                    {preview.valid_rows.map((row, index) => (
                      <tr key={`${row.name}-${index}`} className="border-t border-gray-100">
                        <td className="px-3 py-1.5 text-gray-900">{row.name}</td>
                        <td className="whitespace-nowrap px-3 py-1.5 tabular-nums">{formatAmount(row.amount, row.currency)}</td>
                        <td className="whitespace-nowrap px-3 py-1.5">{formatDate(row.next_payment_date)}</td>
                        <td className="px-3 py-1.5">{PAYMENT_CATEGORY_LABELS[row.category ?? 'other']}</td>
                        <td className="px-3 py-1.5 text-gray-500">
                          {row.assigned_usernames && row.assigned_usernames.length > 0
                            ? row.assigned_usernames.join(', ')
                            : 'все пользователи'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    </Modal>
  );
}
