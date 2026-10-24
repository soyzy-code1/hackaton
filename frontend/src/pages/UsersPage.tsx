import { useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import { Pencil, Plus, Trash2 } from 'lucide-react';
import { api } from '@/api/client';
import type { Role, User } from '@/api/types';
import { ROLE_LABELS } from '@/api/types';
import { useAsyncResource } from '@/hooks/useAsyncResource';
import { useAuthStore } from '@/store/auth';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { ConfirmDialog } from '@/components/ui/ConfirmDialog';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { Select } from '@/components/ui/Select';
import { Table } from '@/components/ui/Table';
import type { Column } from '@/components/ui/Table';

interface UserFormState {
  username: string;
  password: string;
  role: Role;
}

const INITIAL_FORM: UserFormState = {
  username: '',
  password: '',
  role: 'user',
};

const ROLE_OPTIONS: { value: Role; label: string }[] = [
  { value: 'user', label: ROLE_LABELS.user },
  { value: 'admin', label: ROLE_LABELS.admin },
];

export function UsersPage() {
  const currentUser = useAuthStore((state) => state.user);
  const adminUsername = import.meta.env.VITE_ADMIN_USERNAME || 'admin';

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [form, setForm] = useState<UserFormState>(INITIAL_FORM);
  const [formError, setFormError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const users = useAsyncResource<User[]>(() => api.users.list(), []);

  // The admin account from the backend .env is hidden from the table.
  const visibleUsers = useMemo(() => {
    const list = users.data ?? [];
    return list.filter(
      (user) =>
        user.username !== adminUsername &&
        user.id !== currentUser?.id &&
        user.role !== 'admin',
    );
  }, [users.data, adminUsername, currentUser?.id]);

  const openCreateModal = () => {
    setEditingUser(null);
    setForm(INITIAL_FORM);
    setFormError(null);
    setIsModalOpen(true);
  };

  const openEditModal = (user: User) => {
    setEditingUser(user);
    setForm({ username: user.username, password: '', role: user.role });
    setFormError(null);
    setIsModalOpen(true);
  };

  const closeModal = () => {
    if (isSaving) return;
    setIsModalOpen(false);
    setEditingUser(null);
    setFormError(null);
  };

  const updateForm = <K extends keyof UserFormState>(key: K, value: UserFormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError(null);

    if (!form.username.trim()) {
      setFormError('Укажите логин');
      return;
    }
    if (!editingUser && !form.password) {
      setFormError('Укажите пароль');
      return;
    }
    if (editingUser && form.password && form.password.length < 4) {
      setFormError('Пароль должен содержать минимум 4 символа');
      return;
    }

    setIsSaving(true);
    try {
      if (editingUser) {
        const updated = await api.users.update(editingUser.id, {
          username: form.username.trim(),
          ...(form.password ? { password: form.password } : {}),
          role: form.role,
        });
        users.setData(
          (users.data ?? []).map((user) => (user.id === updated.id ? updated : user)),
        );
      } else {
        const created = await api.users.create({
          username: form.username.trim(),
          password: form.password,
          role: form.role,
        });
        users.setData([...(users.data ?? []), created]);
      }
      setIsModalOpen(false);
      setEditingUser(null);
      setForm(INITIAL_FORM);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : 'Не удалось сохранить пользователя');
    } finally {
      setIsSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deletingId) return;
    setIsDeleting(true);
    setActionError(null);
    try {
      await api.users.remove(deletingId);
      users.setData((users.data ?? []).filter((user) => user.id !== deletingId));
      setDeletingId(null);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Не удалось удалить пользователя');
    } finally {
      setIsDeleting(false);
    }
  };

  const deletingUser = (users.data ?? []).find((user) => user.id === deletingId);

  const columns: Column<User>[] = [
    {
      key: 'username',
      header: 'Логин',
      render: (row) => <span className="font-medium text-gray-900">{row.username}</span>,
    },
    {
      key: 'role',
      header: 'Роль',
      render: (row) => (
        <Badge variant={row.role === 'admin' ? 'info' : 'neutral'}>{ROLE_LABELS[row.role]}</Badge>
      ),
    },
    {
      key: 'actions',
      header: 'Действия',
      headerClassName: 'text-right',
      className: 'text-right',
      render: (row) => (
        <div className="flex items-center justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEditModal(row)}
            leftIcon={<Pencil className="h-3.5 w-3.5" aria-hidden="true" />}
          >
            Изменить
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => setDeletingId(row.id)}
            leftIcon={<Trash2 className="h-3.5 w-3.5" aria-hidden="true" />}
          >
            Удалить
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="text-xl font-semibold tracking-tight text-gray-900">Пользователи</h1>
          <p className="text-sm text-gray-500">Управление учётными записями</p>
        </div>
        <Button
          onClick={openCreateModal}
          leftIcon={<Plus className="h-4 w-4" aria-hidden="true" />}
        >
          Добавить пользователя
        </Button>
      </div>

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
          rows={visibleUsers}
          rowKey={(row) => row.id}
          isLoading={users.isLoading}
          error={users.error}
          onRetry={users.refresh}
          emptyMessage="Пользователей пока нет"
        />
      </div>

      <Modal
        isOpen={isModalOpen}
        title={editingUser ? 'Изменить пользователя' : 'Новый пользователь'}
        onClose={closeModal}
        footer={
          <>
            <Button variant="secondary" onClick={closeModal} disabled={isSaving}>
              Отмена
            </Button>
            <Button type="submit" form="user-form" isLoading={isSaving}>
              {editingUser ? 'Сохранить' : 'Создать'}
            </Button>
          </>
        }
      >
        <form id="user-form" onSubmit={handleSubmit} className="space-y-4" noValidate>
          <Input
            label="Логин"
            type="text"
            autoComplete="off"
            placeholder="Введите логин"
            value={form.username}
            onChange={(event) => updateForm('username', event.target.value)}
            disabled={isSaving}
          />

          <Input
            label="Пароль"
            type="password"
            autoComplete="new-password"
            placeholder={editingUser ? 'Оставьте пустым, чтобы не менять' : 'Введите пароль'}
            value={form.password}
            onChange={(event) => updateForm('password', event.target.value)}
            disabled={isSaving}
          />

          <Select
            label="Роль"
            options={ROLE_OPTIONS}
            value={form.role}
            onChange={(event) => updateForm('role', event.target.value as Role)}
            disabled={isSaving}
          />

          {formError ? (
            <p className="text-sm text-red-600" role="alert">
              {formError}
            </p>
          ) : null}
        </form>
      </Modal>

      <ConfirmDialog
        isOpen={deletingId !== null}
        title="Удалить пользователя?"
        message={
          deletingUser
            ? `Пользователь «${deletingUser.username}» будет удалён.`
            : 'Пользователь будет удалён.'
        }
        confirmLabel="Удалить"
        isLoading={isDeleting}
        onConfirm={() => void handleDelete()}
        onCancel={() => setDeletingId(null)}
      />
    </div>
  );
}
