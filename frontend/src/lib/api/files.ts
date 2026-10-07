import { ApiError, request, seg } from '$lib/api';
import type { Attachment, FilesList, UploadMeta } from '$lib/types/files';

export const filesApi = {
  list: (patientId: string, archived = false) => request<FilesList>('GET', `/patients/${seg(patientId)}/files${archived ? '?archived=1' : ''}`),
  archive: (id: string, reason: string) => request<{ file: Attachment }>('POST', `/files/${seg(id)}/archive`, { reason }),
  url: (id: string, download = false) => `/api/files/${seg(id)}${download ? '?download=1' : ''}`,
  /** Fetches the decrypted bytes (used by the DICOM viewer). */
  async bytes(id: string): Promise<ArrayBuffer> {
    const res = await fetch(filesApi.url(id), { credentials: 'same-origin' });
    if (!res.ok) {
      const data = await res.json().catch(() => null);
      throw new ApiError(data?.message ?? 'No se pudo abrir el archivo.', res.status, data?.code ?? '');
    }
    return res.arrayBuffer();
  },
  /** Uploads one file with progress (fetch cannot report upload progress, XHR can). */
  upload(patientId: string, file: File, meta: UploadMeta, onprogress: (fraction: number) => void): Promise<Attachment> {
    return new Promise((resolve, reject) => {
      const form = new FormData();
      form.append('kind', meta.kind);
      form.append('title', meta.title);
      form.append('note', meta.note);
      if (meta.encounter_id) form.append('encounter_id', meta.encounter_id);
      form.append('file', file, file.name);
      const xhr = new XMLHttpRequest();
      xhr.open('POST', `/api/patients/${seg(patientId)}/files`);
      xhr.withCredentials = true;
      xhr.upload.onprogress = (e) => e.lengthComputable && onprogress(e.loaded / e.total);
      xhr.onerror = () => reject(new ApiError('No se pudo conectar con el servidor.', 0));
      xhr.onload = () => {
        let data: { file?: Attachment; message?: string; code?: string } | null = null;
        try {
          data = JSON.parse(xhr.responseText);
        } catch {
          data = null;
        }
        if (xhr.status >= 200 && xhr.status < 300 && data?.file) resolve(data.file);
        else reject(new ApiError(data?.message ?? 'No se pudo subir el archivo.', xhr.status, data?.code ?? ''));
      };
      xhr.send(form);
    });
  }
};
