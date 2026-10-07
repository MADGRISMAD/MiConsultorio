export type FileKind = 'xray' | 'lab' | 'ultrasound' | 'consent' | 'photo' | 'document' | 'other';

export interface Attachment {
  id: string;
  patient_id: string;
  encounter_id: string | null;
  kind: FileKind;
  title: string;
  note: string;
  original_name: string;
  mime: string;
  size_bytes: number;
  sha256: string;
  uploaded_by_name: string;
  created_at: string;
  archived_at: string | null;
  archived_by_name: string;
  archive_reason: string;
  can_archive: boolean;
}

export interface FilesUsage {
  used_bytes: number;
  quota_bytes: number;
}

export interface FilesList {
  files: Attachment[];
  usage: FilesUsage;
  max_upload_bytes: number;
}

export interface UploadMeta {
  kind: FileKind;
  title: string;
  note: string;
  encounter_id: string;
}
