import { useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import { FileUp, Plus, Trash2 } from 'lucide-react';
import { api } from '@/api/client';
import type { Currency, PaymentCategory, Periodicity, RecurringPayment, User } from '@/api/types';
import { CURRENCIES, PAYMENT_CATEGORY_LABELS, PERIODICITIES, PERIODICITY_LABELS } from '@/api/types';
import { useAsyncResource } from '@/hooks/useAsyncResource';
import { formatAmount, formatDate, fromDateInputValue, todayDateInputValue } from '@/lib/date';
import { CsvImportModal } from '@/components/CsvImportModal';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { ConfirmDialog } from '@/components/ui/ConfirmDialog';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { Select } from '@/components/ui/Select';
import { Table } from '@/components/ui/Table';
import type { Column } from '@/components/ui/Table';

interface FormState {
  name: string;
  amount: string;
  currency: Currency;
  nextPaymentDate: string;
  periodicity: Periodicity;
  assignedUserIds: string[];
  category: PaymentCategory;
  notes: string;
  receiptUrl: string;
}

const INITIAL_FORM: FormState = {
  name: '',
  amount: '',
  currency: 'RUB',
  nextPaymentDate: todayDateInputValue(),
  periodicity: 'monthly',
  assignedUserIds: [],
  category: 'other',
  notes: '',
  receiptUrl: '',
};

const CURRENCY_OPTIONS = CURRENCIES.map((currency) => ({ value: currency, label: currency }));
const PERIODICITY_OPTIONS = PERIODICITIES.map((periodicity) => ({
  value: periodicity,
  label: PERIODICITY_LABELS[periodicity],
}));

export function RecurringPaymentsPage() {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [form, setForm] = useState<FormState>(INITIAL_FORM);
  const [formError, setFormError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [isImportOpen, setIsImportOpen] = useState(false);
  const [importNotice, setImportNotice] = useState<string | null>(null);

  const payments = useAsyncResource<RecurringPayment[]>(() => api.recurringPayments.list(), []);
  const users = useAsyncResource<User[]>(() => api.users.list(), []);
  const assignableUsers = useMemo(
    () => (users.data ?? []).filter((user) => user.role === 'user'),
    [users.data],
  );

  const openModal = () => {
    setForm(INITIAL_FORM);
    setFormError(null);
    setIsModalOpen(true);
  };

  const closeModal = () => {
    if (isSaving) return;
    setIsModalOpen(false);
    setFormError(null);
  };

  const updateForm = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError(null);

    const amount = Number(form.amount);
    if (!form.name.trim()) {
      setFormError('Укажите название платежа');
      return;
    }
    if (!Number.isFinite(amount) || amount <= 0) {
      setFormError('Укажите корректную сумму');
      return;
    }
    if (!form.nextPaymentDate) {
      setFormError('Укажите дату ближайшей оплаты');
      return;
    }

    setIsSaving(true);
    try {
      const created = await api.recurringPayments.create({
        name: form.name.trim(),
        amount,
        currency: form.currency,
        next_payment_date: fromDateInputValue(form.nextPaymentDate),
        periodicity: form.periodicity,
        assigned_user_ids: form.assignedUserIds,
        category: form.category,
        notes: form.notes || undefined,
        receipt_url: form.receiptUrl || null,
      });
      payments.setData([...(payments.data ?? []), created]);
      setIsModalOpen(false);
      setForm(INITIAL_FORM);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : 'Не удалось создать платеж');
    } finally {
      setIsSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deletingId) return;
    setIsDeleting(true);
    setActionError(null);
    try {
      await api.recurringPayments.remove(deletingId);
      payments.setData((payments.data ?? []).filter((item) => item.id !== deletingId));
      setDeletingId(null);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Не удалось удалить платеж');
    } finally {
      setIsDeleting(false);
    }
  };

  const deletingPayment = (payments.data ?? []).find((item) => item.id === deletingId);

  const columns: Column<RecurringPayment>[] = [
    {
      key: 'name',
      header: 'Название',
      render: (row) => <span className="font-medium text-gray-900">{row.name}</span>,
    },
    {
      key: 'amount',
      header: 'Сумма',
      render: (row) => (
        <span className="whitespace-nowrap tabular-nums text-gray-700">
          {formatAmount(row.amount, row.currency)}
        </span>
      ),
    },
    {
      key: 'next_payment_date',
      header: 'Ближайшая оплата',
      render: (row) => (
        <span className="whitespace-nowrap text-gray-700">{formatDate(row.next_payment_date)}</span>
      ),
    },
    {
      key: 'periodicity',
      header: 'Периодичность',
      render: (row) => <Badge variant="info">{PERIODICITY_LABELS[row.periodicity]}</Badge>,
    },
    {
      key: 'category',
      header: 'Категория',
      render: (row) => <Badge variant="neutral">{PAYMENT_CATEGORY_LABELS[row.category ?? 'other']}</Badge>,
    },
    {
      key: 'assigned_users',
      header: 'Пользователи',
      render: (row) => (
        <span className="text-gray-600">
          {(row.assigned_user_ids ?? [])
            .map((id) => users.data?.find((user) => user.id === id)?.username ?? id)
            .join(', ') || 'Все пользователи'}
        </span>
      ),
    },
    {
      key: 'actions',
      header: 'Действия',
      headerClassName: 'text-right',
      className: 'text-right',
      render: (row) => (
        <Button
          size="sm"
          variant="danger"
          onClick={() => setDeletingId(row.id)}
          leftIcon={<Trash2 className="h-3.5 w-3.5" aria-hidden="true" />}
        >
          Удалить
        </Button>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="text-xl font-semibold tracking-tight text-gray-900">Регулярные платежи</h1>
          <p className="text-sm text-gray-500">
            Список платежей, которые будут начисляться ежемесячно
          </p>
        </div>
        <div className="grid grid-cols-1 gap-2 min-[420px]:grid-cols-2 sm:flex">
          <Button
            variant="secondary"
            onClick={() => {
              setImportNotice(null);
              setIsImportOpen(true);
            }}
            leftIcon={<FileUp className="h-4 w-4" aria-hidden="true" />}
          >
            Импорт из CSV
          </Button>
          <Button onClick={openModal} leftIcon={<Plus className="h-4 w-4" aria-hidden="true" />}>
            Добавить платеж
          </Button>
        </div>
      </div>

      {importNotice ? (
        <div className="rounded-md border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-700" role="status">
          {importNotice}
        </div>
      ) : null}

      <CsvImportModal
        isOpen={isImportOpen}
        onClose={() => setIsImportOpen(false)}
        knownUsernames={assignableUsers.map((user) => user.username)}
        onImported={(count) => {
          setImportNotice(`Импортировано платежей: ${count}`);
          payments.refresh();
        }}
      />

      {actionError ? (
        <div
          className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700"
          role="alert"
        >
          {actionError}
        </div>
      ) : null}

      <div className="card overflow-hidden">
        <Table
          columns={columns}
          rows={payments.data ?? []}
          rowKey={(row) => row.id}
          isLoading={payments.isLoading}
          error={payments.error}
          onRetry={payments.refresh}
          emptyMessage="Регулярных платежей пока нет"
        />
      </div>

      <Modal
        isOpen={isModalOpen}
        title="Новый регулярный платёж"
        onClose={closeModal}
        footer={
          <>
            <Button variant="secondary" onClick={closeModal} disabled={isSaving}>
              Отмена
            </Button>
            <Button type="submit" form="recurring-payment-form" isLoading={isSaving}>
              Создать
            </Button>
          </>
        }
      >
        <form
          id="recurring-payment-form"
          onSubmit={handleSubmit}
          className="space-y-4"
          noValidate
        >
          <Input
            label="Название"
            placeholder="Например, аренда офиса"
            value={form.name}
            onChange={(event) => updateForm('name', event.target.value)}
            disabled={isSaving}
          />

          <Input
            label="Сумма"
            type="number"
            inputMode="decimal"
            step="0.01"
            min="0"
            placeholder="0.00"
            value={form.amount}
            onChange={(event) => updateForm('amount', event.target.value)}
            disabled={isSaving}
          />

          <Select
            label="Валюта"
            options={CURRENCY_OPTIONS}
            value={form.currency}
            onChange={(event) => updateForm('currency', event.target.value as Currency)}
            disabled={isSaving}
          />

          <Input
            label="Дата ближайшей оплаты"
            type="date"
            value={form.nextPaymentDate}
            onChange={(event) => updateForm('nextPaymentDate', event.target.value)}
            disabled={isSaving}
          />

          <Select
            label="Периодичность"
            options={PERIODICITY_OPTIONS}
            value={form.periodicity}
            onChange={(event) => updateForm('periodicity', event.target.value as Periodicity)}
            disabled={isSaving}
          />

          <Select label="Категория" options={Object.entries(PAYMENT_CATEGORY_LABELS).map(([value, label]) => ({ value, label }))} value={form.category} onChange={(event) => updateForm('category', event.target.value as PaymentCategory)} disabled={isSaving} />
          <Input label="Заметки" placeholder="Например, договор или комментарий" value={form.notes} onChange={(event) => updateForm('notes', event.target.value)} disabled={isSaving} />
          <Input label="Ссылка на чек" type="url" placeholder="https://…" value={form.receiptUrl} onChange={(event) => updateForm('receiptUrl', event.target.value)} disabled={isSaving} />

          <fieldset>
            <legend className="field-label">Назначить пользователям</legend>
            <div className="space-y-2 rounded-md border border-gray-200 p-3">
              {users.isLoading ? (
                <p className="text-sm text-gray-500">Загрузка пользователей…</p>
              ) : assignableUsers.length === 0 ? (
                <p className="text-sm text-gray-500">Нет обычных пользователей</p>
              ) : (
                assignableUsers.map((user) => (
                  <label
                    key={user.id}
                    className="flex items-center gap-2 text-sm text-gray-700"
                  >
                    <input
                      type="checkbox"
                      checked={form.assignedUserIds.includes(user.id)}
                      onChange={(event) => {
                        const ids = event.target.checked
                          ? [...form.assignedUserIds, user.id]
                          : form.assignedUserIds.filter((id) => id !== user.id);
                        updateForm('assignedUserIds', ids);
                      }}
                      disabled={isSaving}
                      className="h-4 w-4 rounded border-gray-300 text-blue-600"
                    />
                    {user.username}
                  </label>
                ))
              )}
            </div>
            <p className="mt-1 text-xs text-gray-500">
              Если ничего не выбрать, платёж назначится всем обычным пользователям.
            </p>
          </fieldset>

          {formError ? (
            <p className="text-sm text-red-600" role="alert">
              {formError}
            </p>
          ) : null}
        </form>
      </Modal>

      <ConfirmDialog
        isOpen={deletingId !== null}
        title="Удалить платёж?"
        message={
          deletingPayment
            ? `Платёж «${deletingPayment.name}» на сумму ${formatAmount(
                deletingPayment.amount,
                deletingPayment.currency,
              )} будет удалён.`
            : 'Платёж будет удалён.'
        }
        confirmLabel="Удалить"
        isLoading={isDeleting}
        onConfirm={() => void handleDelete()}
        onCancel={() => setDeletingId(null)}
      />
    </div>
  );
}
