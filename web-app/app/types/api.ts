// Types mirroring ../../API_CONTRACT.md

export interface Paginated<T> {
  data: T[]
  meta: {
    page: number
    per_page: number
    total: number
    last_page: number
  }
}

export interface ApiError {
  message: string
  errors?: Record<string, string[] | string>
  status?: number
}

export type ClientStatus = 'active' | 'inactive' | 'blocked'

export interface Client {
  id: number
  name: string
  username: string
  status: ClientStatus
  quota_bytes: number | null
  used_bytes: number
  files_count: number
  allowed_mimes: string[]
  max_file_size: number | null
  owner_id?: number | null
  owner?: { id: number; name: string; email?: string } | null
  creator_id?: number | null
  updater_id?: number | null
  created_at: string
  updated_at?: string
}

export type Visibility = 'public' | 'private'

export interface FileItem {
  id: number
  client_id: number
  client?: { id: number; name: string }
  folder: string
  name: string
  original_name: string
  path: string
  url: string | null
  mime: string
  extension: string
  size: number
  sha256: string
  visibility: Visibility
  created_at: string
}

export interface Folder {
  name: string
  path: string
  files_count: number
  size: number
}

export type ArchiveStatus = 'pending' | 'processing' | 'done' | 'failed' | 'expired'

export interface Archive {
  id: number
  client_id: number
  client?: { id: number; name: string }
  created_by: { type: 'user' | 'client'; id: number }
  folders: string[]
  status: ArchiveStatus
  progress: number
  files_count: number
  size: number | null
  path: string | null
  url: string | null
  error: string | null
  callback_url: string | null
  expires_at: string | null
  created_at: string
  finished_at: string | null
}

export interface ArchiveStatusItem {
  id: number
  status: ArchiveStatus
  progress: number
  url: string | null
  expires_at: string | null
}

export interface Device {
  id: number
  uid: string
  platform: string | null
  app_version: string | null
  ip: string | null
  last_seen_at: string | null
  client_id: number
  created_at: string
}

/** `pending` = invited, OTP not verified yet. */
export type UserStatus = 'pending' | 'active' | 'blocked'

/** Roles the admin panel knows about. */
export type RoleSlug = 'super_admin' | 'admin'

export interface Role {
  id: number
  name: string
  slug: string
  permissions?: unknown
}

export interface User {
  id: number
  name: string
  email: string
  avatar_src: string | null
  status: UserStatus
  role: Role
  clients_count?: number
  last_login_at?: string | null
  created_at?: string
}

export interface AuditLog {
  id: number
  actor: { type: 'user' | 'client'; id: number; name: string }
  action: string
  subject_type: string | null
  subject_id: number | null
  client_id: number | null
  details: Record<string, unknown> | null
  ip: string | null
  created_at: string
}

export interface DashboardStats {
  clients_count: number
  files_count: number
  total_size: number
  archives_pending: number
  by_client: Array<{ client_id: number; name: string; files_count: number; size: number }>
  uploads_last_30_days: Array<{ date: string; count: number; size: number }>
}

export interface StorageSyncResult {
  orphans_on_disk: number
  missing_on_disk: number
  removed_records: number
  removed_files: number
  errors: number
}
